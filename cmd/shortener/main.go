package main

import (
	"net/http"

	"github.com/Dahasolo/urlshortener/internal/handler"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", handler.ShortenHandler)
	mux.HandleFunc("/{id}", handler.RedirectHandler)
	http.ListenAndServe(":8080", mux)
}
