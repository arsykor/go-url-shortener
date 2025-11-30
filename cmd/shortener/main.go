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

	r := shortenerHandler.Router()

	log.Printf("Server starting on http://localhost:8080")
	if err := http.ListenAndServe(":8080", r); err != nil {
		log.Fatal(err)
	}
}
