package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/soheilhy/cmux"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"github.com/arsykor/go-url-shortener/internal/audit"
	"github.com/arsykor/go-url-shortener/internal/config"
	"github.com/arsykor/go-url-shortener/internal/database"
	"github.com/arsykor/go-url-shortener/internal/grpcserver"
	"github.com/arsykor/go-url-shortener/internal/handler"
	"github.com/arsykor/go-url-shortener/internal/repository"
	"github.com/arsykor/go-url-shortener/internal/service"
	"github.com/arsykor/go-url-shortener/internal/urlapi"
)

// Build metadata injected at link time via -ldflags:
//
//	go build -ldflags "-X main.buildVersion=1.2.3 -X main.buildDate=2026-04-26 -X main.buildCommit=abc123"
var (
	buildVersion string
	buildDate    string
	buildCommit  string
)

func valueOrNA(s string) string {
	if s == "" {
		return "N/A"
	}
	return s
}

func main() {
	fmt.Printf("Build version: %s\n", valueOrNA(buildVersion))
	fmt.Printf("Build date: %s\n", valueOrNA(buildDate))
	fmt.Printf("Build commit: %s\n", valueOrNA(buildCommit))

	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatal(err)
	}
	defer logger.Sync()

	sugar := logger.Sugar()
	cfg, err := config.Load()
	if err != nil {
		sugar.Fatalw("Failed to load config", "error", err)
	}

	// Priority: DATABASE_DSN > FILE_STORAGE_PATH > in-memory
	var urlRepo service.URLRepository
	var db handler.DB

	if cfg.DatabaseDSN != "" {
		databaseConn, err := database.NewDB(cfg.DatabaseDSN)
		if err != nil {
			sugar.Fatalw("Failed to connect to database", "error", err)
		}
		defer databaseConn.Close()
		db = databaseConn
		urlRepo = repository.NewPostgresURLRepository(databaseConn.DB)
		sugar.Info("Using PostgreSQL storage")
	} else if cfg.FileStoragePath != "" {
		fileRepo, err := repository.NewFileURLRepository(cfg.FileStoragePath)
		if err != nil {
			sugar.Fatalw("Failed to create file repository", "error", err)
		}
		urlRepo = fileRepo
		sugar.Infow("Using file storage", "path", cfg.FileStoragePath)
	} else {
		urlRepo = repository.NewInMemoryURLRepository()
		sugar.Info("Using in-memory storage")
	}

	// Observer pattern
	var auditObservers []audit.Observer
	if cfg.AuditFile != "" {
		fo, err := audit.NewFileObserver(cfg.AuditFile)
		if err != nil {
			sugar.Fatalw("Failed to open audit file", "error", err)
		}
		defer fo.Close()
		auditObservers = append(auditObservers, fo)
		sugar.Infow("Audit file sink enabled", "path", cfg.AuditFile)
	}
	if cfg.AuditURL != "" {
		auditObservers = append(auditObservers, audit.NewHTTPObserver(cfg.AuditURL))
		sugar.Infow("Audit HTTP sink enabled", "url", cfg.AuditURL)
	}
	auditSvc := audit.NewService(sugar, auditObservers...)

	shortenerService := service.NewShortenerService(urlRepo, cfg.BaseURL, sugar)
	var trustedSubnet *net.IPNet
	if cidr := strings.TrimSpace(cfg.TrustedSubnet); cidr != "" {
		_, n, err := net.ParseCIDR(cidr)
		if err != nil {
			sugar.Fatalw("invalid trusted_subnet (CIDR)", "value", cidr, "error", err)
		}
		trustedSubnet = n
	}
	urlFacade := &urlapi.Facade{Svc: shortenerService, Audit: auditSvc}
	shortenerHandler := handler.NewShortener(urlFacade, db, sugar, trustedSubnet)

	r := shortenerHandler.Router()

	tcpLn, err := net.Listen("tcp", cfg.ServerAddress)
	if err != nil {
		sugar.Fatalw("listen failed", "addr", cfg.ServerAddress, "error", err)
	}

	var ln net.Listener = tcpLn
	if cfg.EnableHTTPS {
		tlsCfg, err := selfSignedTLSConfig()
		if err != nil {
			sugar.Fatalw("Failed to generate TLS certificate", "error", err)
		}
		ln = tls.NewListener(tcpLn, tlsCfg)
	}

	muxL := cmux.New(ln)
	grpcMatched := muxL.Match(cmux.HTTP2HeaderField("content-type", "application/grpc"))
	httpMatched := muxL.Match(cmux.Any())

	grpcSrv := grpc.NewServer()
	grpcserver.Register(grpcSrv, &grpcserver.Server{Facade: urlFacade, Logger: sugar})

	httpSrv := &http.Server{
		Handler: r,
	}

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		sugar.Infow("Multiplexed HTTP+gRPC listener", "addr", cfg.ServerAddress, "tls", cfg.EnableHTTPS)
		if err := muxL.Serve(); err != nil && !strings.Contains(err.Error(), "use of closed") {
			sugar.Errorw("cmux exited", "error", err)
		}
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := grpcSrv.Serve(grpcMatched); err != nil {
			sugar.Errorw("gRPC stopped", "error", err)
		}
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := httpSrv.Serve(httpMatched); err != nil && err != http.ErrServerClosed {
			sugar.Errorw("HTTP stopped", "error", err)
		}
	}()

	// Block until a shutdown signal is received.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)
	sig := <-quit
	sugar.Infow("Received signal, shutting down", "signal", sig)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	grpcSrv.GracefulStop()
	if err := httpSrv.Shutdown(ctx); err != nil {
		sugar.Warnw("HTTP forced to shutdown", "error", err)
	}
	muxL.Close()
	wg.Wait()
	sugar.Info("Server stopped")
}
