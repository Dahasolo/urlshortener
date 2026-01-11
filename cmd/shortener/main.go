package main

import (
	"net/http"

	"github.com/Dahasolo/urlshortener/internal/handler"
	"github.com/Dahasolo/urlshortener/internal/repository"
	"github.com/Dahasolo/urlshortener/internal/service"
	"github.com/go-chi/chi/v5"
)

func main() {
	repo := repository.NewInMemoryURLRepo()
	svc := service.NewService(repo)

	// mux := http.NewServeMux()
	// mux.HandleFunc("/", handler.ShortenHandler(svc))
	// mux.HandleFunc("/{id}", handler.RedirectHandler(svc)) // ← ЭТО НЕ РАБОТАЛО!
	// http.ListenAndServe(":8080", mux)

	r := chi.NewRouter()
	r.Post("/", handler.ShortenHandler(svc))
	r.Get("/{id}", handler.RedirectHandler(svc))
	http.ListenAndServe(":8080", r)
}
