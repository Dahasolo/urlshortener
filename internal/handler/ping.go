package handler

import (
	"net/http"

	"github.com/Dahasolo/urlshortener/internal/service"
)

// PingHandler проверяет соединение с базой данных.
func PingHandler(svc *service.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := svc.Ping(); err != nil {
			http.Error(w, "storage connection failed", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}
