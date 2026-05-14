package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/arsykor/go-url-shortener/internal/audit"
	"github.com/arsykor/go-url-shortener/internal/config"
	"github.com/arsykor/go-url-shortener/internal/database"
	"github.com/arsykor/go-url-shortener/internal/handler"
	"github.com/arsykor/go-url-shortener/internal/repository"
	"github.com/arsykor/go-url-shortener/internal/service"
	"go.uber.org/zap"
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
	shortenerHandler := handler.NewShortener(shortenerService, db, sugar, auditSvc, trustedSubnet)

	r := shortenerHandler.Router()

	srv := &http.Server{
		Addr:    cfg.ServerAddress,
		Handler: r,
	}

	// Initialise TLS synchronously (before go func()).
	if cfg.EnableHTTPS {
		tlsCfg, err := selfSignedTLSConfig()
		if err != nil {
			sugar.Fatalw("Failed to generate TLS certificate", "error", err)
		}
		srv.TLSConfig = tlsCfg
	}

	// Start the server in a goroutine so we can listen for shutdown signals.
	go func() {
		var err error
		if cfg.EnableHTTPS {
			sugar.Infow("Starting HTTPS server", "addr", cfg.ServerAddress)
			// ("", "") - use cert already loaded into srv.TLSConfig
			err = srv.ListenAndServeTLS("", "")
		} else {
			sugar.Infow("Starting HTTP server", "addr", cfg.ServerAddress)
			err = srv.ListenAndServe()
		}
		if err != nil && err != http.ErrServerClosed {
			sugar.Fatalw(err.Error(), "event", "start server")
		}
	}()

	// Block until a shutdown signal is received.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)
	sig := <-quit
	sugar.Infow("Received signal, shutting down", "signal", sig)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		sugar.Warnw("Server forced to shutdown", "error", err)
	}
	sugar.Info("Server stopped")
}
