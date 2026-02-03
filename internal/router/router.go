package router

import (
	"net/http"

	"github.com/Dahasolo/urlshortener/internal/handler"
	"github.com/Dahasolo/urlshortener/internal/service"
	"github.com/go-chi/chi/v5"
)

func NewRouter(svc *service.Service, baseURL string) http.Handler {
	r := chi.NewRouter()
	r.Post("/", handler.ShortenHandler(svc, baseURL))
	r.Get("/{id}", handler.RedirectHandler(svc))
	r.Post("/api/shorten", handler.ShortenJSONHandler(svc, baseURL))
	return r
}
