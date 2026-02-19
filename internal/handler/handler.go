package handler

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/Dahasolo/urlshortener/internal/service"
	"github.com/asaskevich/govalidator"
	"github.com/go-chi/chi/v5"
)

// ShortenRequest описывает запрос пользователя.
type ShortenRequest struct {
	URL string `json:"url"`
}

// ShortenResponse описывает ответ сервера.
type ShortenResponse struct {
	Result string `json:"result"`
}

// batchShortenRequest описывает элемент запроса для множественного сокращения.
type batchShortenRequest struct {
	CorrelationID string `json:"correlation_id"`
	OriginalURL   string `json:"original_url"`
}

// batchShortenResponse описывает элемент ответа для множественного сокращения.
type batchShortenResponse struct {
	CorrelationID string `json:"correlation_id"`
	ShortURL      string `json:"short_url"`
}

// ShortenHandler обрабатывает запросы на сокращение URL из тела запроса (текст).
func ShortenHandler(svc *service.Service, baseURL string, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			logger.Error("failed to read body", "error", err)
			http.Error(w, "failed to read body", http.StatusBadRequest)
			return
		}

		originalURL := strings.TrimSpace(string(body))
		if originalURL == "" {
			http.Error(w, "empty URL", http.StatusBadRequest)
			return
		}

		if _, err = url.Parse(originalURL); err != nil {
			http.Error(w, "not valid URL", http.StatusBadRequest)
			return
		}

		id, err := svc.Shorten(originalURL)
		if err != nil {
			logger.Error("failed to shorten URL", "url", originalURL, "error", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		shortURL, _ := url.JoinPath(baseURL, id)

		logger.Info("URL shortened successfully", "original", originalURL, "short", shortURL, "id", id)

		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(shortURL))
	}
}

// RedirectHandler обрабатывает запросы на получение оригинального URL по короткому идентификатору.
func RedirectHandler(svc *service.Service, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")

		if id == "" {
			http.Error(w, "empty ID", http.StatusBadRequest)
			return
		}

		fullURL, ok := svc.Resolve(id)
		if !ok {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Location", fullURL)
		w.WriteHeader(http.StatusTemporaryRedirect)
	}
}

// ShortenJSONHandler обрабатывает запросы на сокращение URL из JSON-тела запроса.
func ShortenJSONHandler(svc *service.Service, baseURL string, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		contentType := r.Header.Get("Content-Type")
		if contentType != "application/json" {
			http.Error(w, "Content-Type must be application/json", http.StatusBadRequest)
			return
		}

		var req ShortenRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}

		if !govalidator.IsURL(req.URL) {
			http.Error(w, "invalid URL format", http.StatusBadRequest)
			return
		}

		id, err := svc.Shorten(req.URL)
		if err != nil {
			logger.Error("failed to shorten URL from JSON request", "url", req.URL, "error", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		shortURL, _ := url.JoinPath(baseURL, id)

		logger.Info("URL shortened from JSON", "original", req.URL, "short", shortURL, "id", id)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)

		resp := ShortenResponse{Result: shortURL}
		json.NewEncoder(w).Encode(resp)
	}
}

// BatchShortenHandler обрабатывает множественное сокращение URL.
func BatchShortenHandler(svc *service.Service, baseURL string, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		contentType := r.Header.Get("Content-Type")
		if contentType != "application/json" {
			http.Error(w, "Content-Type must be application/json", http.StatusBadRequest)
			return
		}

		var requests []batchShortenRequest
		if err := json.NewDecoder(r.Body).Decode(&requests); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}

		if len(requests) == 0 {
			http.Error(w, "empty batch", http.StatusBadRequest)
			return
		}
		for _, req := range requests {
			if !govalidator.IsURL(req.OriginalURL) {
				http.Error(w, "invalid URL format", http.StatusBadRequest)
				return
			}
		}

		svcRequests := make([]service.BatchRequest, 0, len(requests))
		for _, req := range requests {
			svcRequests = append(svcRequests, service.BatchRequest{
				CorrelationID: req.CorrelationID,
				OriginalURL:   req.OriginalURL,
			})
		}

		results, err := svc.BatchShorten(svcRequests)
		if err != nil {
			logger.Error("batch shorten failed", "error", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		responses := make([]batchShortenResponse, 0, len(results))
		for _, res := range results {
			shortURL, _ := url.JoinPath(baseURL, res.ID)
			responses = append(responses, batchShortenResponse{
				CorrelationID: res.CorrelationID,
				ShortURL:      shortURL,
			})
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(responses)
	}
}
