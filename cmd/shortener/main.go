package main

import (
	"log"
	"net/http"

	"github.com/arsykor/go-url-shortener/internal/handler"
	"github.com/arsykor/go-url-shortener/internal/repository"
	"github.com/arsykor/go-url-shortener/internal/service"
)

func main() {
	urlRepo := repository.NewInMemoryURLRepository()
	shortenerService := service.NewShortenerService(urlRepo)
	shortenerHandler := handler.NewShortener(shortenerService)

	mux := http.NewServeMux()
	mux.HandleFunc("/", shortenerHandler.HandlerShortener)

	log.Printf("Server starting on http://localhost:8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal(err)
	}
}
