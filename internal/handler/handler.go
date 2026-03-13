package handler

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/Dahasolo/urlshortener/internal/auth"
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

// UserURLResponse описывает элемент ответа для списка URL пользователя.
type UserURLResponse struct {
	ShortURL    string `json:"short_url"`
	OriginalURL string `json:"original_url"`
}

// getUserIDFromRequest извлекает или создаёт userID из куки.
func getUserIDFromRequest(r *http.Request, w http.ResponseWriter, secretKey string, logger *slog.Logger) (string, bool, error) {

	userID, err := auth.GetUserIDFromRequest(r, secretKey)
	if err != nil {
		userID, err = auth.GenerateUserID()
		if err != nil {
			logger.Error("failed to generate user ID", "error", err)
			return "", false, err
		}

		if err := auth.SetAuthCookie(w, userID, secretKey); err != nil {
			logger.Error("failed to set auth cookie", "error", err)
		}

		return userID, true, nil
	}

	return userID, false, nil
}

// buildShortURL строит полный короткий URL.
func buildShortURL(baseURL, id string) string {
	shortURL, _ := url.JoinPath(baseURL, id)
	return shortURL
}

// handleShortenError обрабатывает ошибки сокращения URL.
func handleShortenError(w http.ResponseWriter, logger *slog.Logger,
	err error, originalURL, baseURL string, isJSON bool) {

	var alreadyExists *service.ErrURLAlreadyExists
	if errors.As(err, &alreadyExists) {
		logger.Info("URL already exists", "original", alreadyExists.OriginalURL, "existing_id", alreadyExists.ExistingID)

		shortURL := buildShortURL(baseURL, alreadyExists.ExistingID)

		if isJSON {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(ShortenResponse{Result: shortURL})
		} else {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusConflict)
			w.Write([]byte(shortURL))
		}
		return
	}

	logger.Error("failed to shorten URL", "url", originalURL, "error", err)
	http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}

// validateURL проверяет корректность URL.
func validateURL(urlStr string) bool {
	return govalidator.IsURL(urlStr)
}

// ShortenHandler обрабатывает запросы на сокращение URL из тела запроса (текст).
func ShortenHandler(svc *service.Service, baseURL, secretKey string, logger *slog.Logger) http.HandlerFunc {
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

		userID, _, err := getUserIDFromRequest(r, w, secretKey, logger)
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		id, err := svc.Shorten(r.Context(), originalURL, userID)
		if err != nil {
			logger.Error("shorten failed", "url", originalURL, "user_id", userID, "error", err)
			handleShortenError(w, logger, err, originalURL, baseURL, false)
			return
		}

		shortURL := buildShortURL(baseURL, id)
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

		res, ok := svc.Resolve(r.Context(), id)
		if !ok {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		if res.IsDeleted {
			w.WriteHeader(http.StatusGone)
			return
		}

		w.Header().Set("Location", res.OriginalURL)
		w.WriteHeader(http.StatusTemporaryRedirect)
	}
}

// ShortenJSONHandler обрабатывает запросы на сокращение URL из JSON-тела запроса.
func ShortenJSONHandler(svc *service.Service, baseURL, secretKey string, logger *slog.Logger) http.HandlerFunc {
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

		if !validateURL(req.URL) {
			http.Error(w, "invalid URL format", http.StatusBadRequest)
			return
		}

		userID, _, err := getUserIDFromRequest(r, w, secretKey, logger)
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		id, err := svc.Shorten(r.Context(), req.URL, userID)
		if err != nil {
			logger.Error("shorten failed", "url", req.URL, "user_id", userID, "error", err)
			handleShortenError(w, logger, err, req.URL, baseURL, true)
			return
		}

		shortURL := buildShortURL(baseURL, id)
		logger.Info("URL shortened from JSON", "original", req.URL, "short", shortURL, "id", id)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)

		resp := ShortenResponse{Result: shortURL}
		json.NewEncoder(w).Encode(resp)
	}
}

// BatchShortenHandler обрабатывает множественное сокращение URL.
func BatchShortenHandler(svc *service.Service, baseURL, secretKey string, logger *slog.Logger) http.HandlerFunc {
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
			if !validateURL(req.OriginalURL) {
				http.Error(w, "invalid URL format", http.StatusBadRequest)
				return
			}
		}

		userID, _, err := getUserIDFromRequest(r, w, secretKey, logger)
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		svcRequests := make([]service.BatchRequest, 0, len(requests))
		for _, req := range requests {
			svcRequests = append(svcRequests, service.BatchRequest{
				CorrelationID: req.CorrelationID,
				OriginalURL:   req.OriginalURL,
			})
		}

		results, err := svc.BatchShorten(r.Context(), svcRequests, userID)
		if err != nil {
			logger.Error("batch shorten failed", "error", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		responses := make([]batchShortenResponse, 0, len(results))
		for _, res := range results {
			shortURL := buildShortURL(baseURL, res.ID)
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

// UserURLsHandler возвращает все URL пользователя.
func UserURLsHandler(svc *service.Service, baseURL, secretKey string, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(auth.CookieName)

		// Если куки нет - создаём нового пользователя
		if errors.Is(err, http.ErrNoCookie) {
			userID, genErr := auth.GenerateUserID()
			if genErr != nil {
				logger.Error("failed to generate user ID", "error", genErr)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}
			if setErr := auth.SetAuthCookie(w, userID, secretKey); setErr != nil {
				logger.Error("failed to set auth cookie", "error", setErr)
			}
			serveUserURLs(w, r, svc, baseURL, userID, logger)
			return
		}

		// Ошибка чтения куки - 401
		if err != nil {
			logger.Warn("failed to read cookie", "error", err)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// Если кука есть - проверяем подпись
		userID, verifyErr := auth.VerifyToken(cookie.Value, secretKey)
		if verifyErr != nil {
			logger.Warn("invalid auth token", "error", verifyErr)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		serveUserURLs(w, r, svc, baseURL, userID, logger)
	}
}

// serveUserURLs записывает список сокращённых URL пользователя в HTTP-ответ.
func serveUserURLs(w http.ResponseWriter, r *http.Request, svc *service.Service,
	baseURL, userID string, logger *slog.Logger) {

	urls, err := svc.GetUserURLs(r.Context(), userID)
	if err != nil {
		logger.Error("failed to get user URLs", "user_id", userID, "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if len(urls) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	responses := make([]UserURLResponse, 0, len(urls))
	for _, rec := range urls {
		responses = append(responses, UserURLResponse{
			ShortURL:    buildShortURL(baseURL, rec.ShortURL),
			OriginalURL: rec.OriginalURL,
		})
	}
	json.NewEncoder(w).Encode(responses)
}

// DeleteUserURLsHandler обрабатывает запросы на удаление URL.
func DeleteUserURLsHandler(svc *service.Service, secretKey string, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		contentType := r.Header.Get("Content-Type")
		if contentType != "application/json" {
			http.Error(w, "Content-Type must be application/json", http.StatusBadRequest)
			return
		}

		var ids []string
		if err := json.NewDecoder(r.Body).Decode(&ids); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}

		if len(ids) == 0 {
			http.Error(w, "empty batch", http.StatusBadRequest)
			return
		}
		for _, id := range ids {
			if id == "" {
				http.Error(w, "empty ID in batch", http.StatusBadRequest)
				return
			}
		}

		userID, _, err := getUserIDFromRequest(r, w, secretKey, logger)
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		svc.DeleteUserURLs(ids, userID)

		w.WriteHeader(http.StatusAccepted)
	}
}
