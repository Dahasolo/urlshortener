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
func NewRouter(svc *service.Service, baseURL string, appLogger *slog.Logger) http.Handler {
	r := chi.NewRouter()

	r.Use(logger.HTTPLogger(appLogger))
	r.Use(middleware.GzipMiddleware)

	r.Post("/", handler.ShortenHandler(svc, baseURL, appLogger))
	r.Get("/{id}", handler.RedirectHandler(svc, appLogger))
	r.Post("/api/shorten", handler.ShortenJSONHandler(svc, baseURL, appLogger))

	return r
}
