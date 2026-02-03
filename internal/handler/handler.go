package handler

import (
	"encoding/json"
	"io"
	"log"
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

func ShortenHandler(svc *service.Service, baseURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
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
			log.Printf("Shorten failed: url=%q err=%v", originalURL, err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		shortURL, _ := url.JoinPath(baseURL, id)
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(shortURL))
	}
}

func RedirectHandler(svc *service.Service) http.HandlerFunc {
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

func ShortenJSONHandler(svc *service.Service, baseURL string) http.HandlerFunc {
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
			log.Printf("Shorten failed: url=%q err=%v", req.URL, err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		shortURL, _ := url.JoinPath(baseURL, id)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)

		resp := ShortenResponse{Result: shortURL}
		json.NewEncoder(w).Encode(resp)
	}
}
