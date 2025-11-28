package main

import (
	"log"
	"net/http"

	"github.com/arsykor/go-url-shortener/internal/handler"
)

func main() {
	var handlerShortener handler.Shortener

	mux := http.NewServeMux()
	mux.HandleFunc("/", handlerShortener.HandlerShortener)

	log.Printf("Server starting on http://localhost:8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal(err)
	}
}
