package main

import (
	"net/http"

	"github.com/arsykor/go-url-shortener/internal/config"
	"github.com/arsykor/go-url-shortener/internal/database"
	"github.com/arsykor/go-url-shortener/internal/handler"
	"github.com/arsykor/go-url-shortener/internal/repository"
	"github.com/arsykor/go-url-shortener/internal/service"
	"go.uber.org/zap"
)

func main() {
	logger, err := zap.NewDevelopment()
	if err != nil {
		panic(err)
	}
	defer logger.Sync()

	sugar := logger.Sugar()
	cfg := config.Load()

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

	shortenerService := service.NewShortenerService(urlRepo, cfg.BaseURL)
	shortenerHandler := handler.NewShortener(shortenerService, db)

	r := shortenerHandler.Router(sugar)

	sugar.Infow(
		"Starting server",
		"addr", cfg.ServerAddress,
	)
	if err := http.ListenAndServe(cfg.ServerAddress, r); err != nil {
		sugar.Fatalw(err.Error(), "event", "start server")
	}
}
