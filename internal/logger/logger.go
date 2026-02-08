package logger

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"
)

// NewLogger создаёт новый экземпляр slog.Logger с указанным уровнем логирования.
func NewLogger(level string) (*slog.Logger, error) {
	lvl, err := parseLevel(level)
	if err != nil {
		return nil, err
	}

	h := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: lvl})
	return slog.New(h), nil
}

// parseLevel преобразует строковое представление уровня логирования в slog.Leveler.
func parseLevel(levelStr string) (slog.Leveler, error) {
	var lvl slog.Level
	if err := lvl.UnmarshalText([]byte(strings.ToUpper(levelStr))); err != nil {
		return nil, fmt.Errorf("invalid log level %q: %w", levelStr, err)
	}
	return &lvl, nil
}

// responseData хранит сведения об ответе: статус и размер тела.
type responseData struct {
	status int
	size   int
}

// loggingResponseWriter оборачивает оригинальный ResponseWriter.
type loggingResponseWriter struct {
	http.ResponseWriter
	responseData *responseData
}

// Write записывает ответ и считает, сколько байт было записано.
func (r *loggingResponseWriter) Write(b []byte) (int, error) {
	size, err := r.ResponseWriter.Write(b)
	r.responseData.size += size
	return size, err
}

// WriteHeader записывает код статуса.
func (r *loggingResponseWriter) WriteHeader(statusCode int) {
	r.ResponseWriter.WriteHeader(statusCode)
	r.responseData.status = statusCode
}

// HTTPLogger возвращает middleware логирования.
func HTTPLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			data := &responseData{
				status: 0,
				size:   0,
			}

			lrw := &loggingResponseWriter{
				ResponseWriter: w,
				responseData:   data,
			}
			h.ServeHTTP(lrw, r)

			// Если статус не был установлен явно — считаем, что 200 OK
			status := data.status
			if status == 0 {
				status = http.StatusOK
			}

			duration := time.Since(start)

			logger.Info("handled request",
				slog.String("uri", r.RequestURI),
				slog.String("method", r.Method),
				slog.Int("status", status),
				slog.Int("size", data.size),
				slog.Duration("duration", duration),
			)
		})
	}
}
