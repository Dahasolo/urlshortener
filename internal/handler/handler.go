package handler

import (
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/Dahasolo/urlshortener/internal/service"
	"github.com/go-chi/chi/v5"
)

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
			http.Error(w, "internal server error", http.StatusInternalServerError)
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
