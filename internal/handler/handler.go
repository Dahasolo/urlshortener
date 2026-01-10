package handler

import (
	"io"
	"net/http"

	"github.com/Dahasolo/urlshortener/internal/service"
)

func ShortenHandler(svc *service.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "only POST", http.StatusMethodNotAllowed)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		url := string(body)
		if url == "" {
			http.Error(w, "empty", http.StatusBadRequest)
			return
		}
		id := svc.Shorten(url)
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("http://localhost:8080/" + id))
	}
}

func RedirectHandler(svc *service.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "only GET", http.StatusMethodNotAllowed)
			return
		}
		id := r.URL.Path[1:]
		if id == "" {
			http.Error(w, "empty ID", http.StatusBadRequest)
			return
		}
		url, ok := svc.Resolve(id)
		if !ok {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Location", url)
		w.WriteHeader(http.StatusTemporaryRedirect)
	}
}
