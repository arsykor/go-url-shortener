package main

import (
	"log"
	"net/http"

	"github.com/arsykor/go-url-shortener/internal/config"
	"github.com/arsykor/go-url-shortener/internal/handler"
	"github.com/arsykor/go-url-shortener/internal/repository"
	"github.com/arsykor/go-url-shortener/internal/service"
)

func main() {
	cfg := config.Load()

	urlRepo := repository.NewInMemoryURLRepository()
	shortenerService := service.NewShortenerService(urlRepo, cfg.BaseURL)
	shortenerHandler := handler.NewShortener(shortenerService)

	r := shortenerHandler.Router()

	log.Printf("Server starting on %s", cfg.ServerAddress)
	if err := http.ListenAndServe(cfg.ServerAddress, r); err != nil {
		log.Fatal(err)
	}
}
