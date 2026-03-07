package router

import (
	"log/slog"
	"net/http"

	"github.com/Dahasolo/urlshortener/internal/handler"
	"github.com/Dahasolo/urlshortener/internal/handler/middleware"
	"github.com/Dahasolo/urlshortener/internal/logger"
	"github.com/Dahasolo/urlshortener/internal/service"

	"github.com/go-chi/chi/v5"
)

// NewRouter создаёт и настраивает роутер приложения.
func NewRouter(svc *service.Service, baseURL, secretKey string, appLogger *slog.Logger) http.Handler {
	r := chi.NewRouter()

	r.Use(logger.HTTPLogger(appLogger))
	r.Use(middleware.GzipMiddleware)

	r.Post("/", handler.ShortenHandler(svc, baseURL, secretKey, appLogger))
	r.Get("/{id}", handler.RedirectHandler(svc, appLogger))
	r.Post("/api/shorten", handler.ShortenJSONHandler(svc, baseURL, secretKey, appLogger))
	r.Get("/ping", handler.PingHandler(svc))
	r.Post("/api/shorten/batch", handler.BatchShortenHandler(svc, baseURL, secretKey, appLogger))
	r.Get("/api/user/urls", handler.UserURLsHandler(svc, baseURL, secretKey, appLogger))

	return r
}
