package logger

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"
)

// Log - глобальный логгер slog.
var Log *slog.Logger = slog.New(slog.NewTextHandler(os.Stderr, nil))

// Initialize инициализирует синглтон логера с необходимым уровнем логирования.
func Initialize(level string) error {
	lvl, err := parseLevel(level)
	if err != nil {
		return err
	}

	h := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: lvl})
	Log = slog.New(h)
	slog.SetDefault(Log)
	return nil
}

// parseLevel парсит строковый уровень в slog.Leveler.
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
	http.ResponseWriter // встраиваем оригинальный http.ResponseWriter
	responseData        *responseData
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

// HTTPLogger - middleware, который логирует каждый запрос и ответ.
func HTTPLogger(h http.Handler) http.Handler {
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

		Log.Info("handled request",
			slog.String("uri", r.RequestURI),
			slog.String("method", r.Method),
			slog.Int("status", status),
			slog.Int("size", data.size),
			slog.Duration("duration", duration),
		)
	})
}
