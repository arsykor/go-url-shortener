package main

import (
	"net/http"

	"github.com/arsykor/go-url-shortener/internal/config"
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

	urlRepo := repository.NewInMemoryURLRepository()
	shortenerService := service.NewShortenerService(urlRepo, cfg.BaseURL)
	shortenerHandler := handler.NewShortener(shortenerService)

	r := shortenerHandler.Router(sugar)

	sugar.Infow(
		"Starting server",
		"addr", cfg.ServerAddress,
	)
	if err := http.ListenAndServe(cfg.ServerAddress, r); err != nil {
		sugar.Fatalw(err.Error(), "event", "start server")
	}
}
