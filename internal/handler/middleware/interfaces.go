package middleware

import "net/http"

// Handler - интерфейс для тестирования.
type Handler interface {
	ServeHTTP(http.ResponseWriter, *http.Request)
}
