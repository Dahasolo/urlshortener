package logger

import (
	"net/http"
	"time"

	"go.uber.org/zap"
)

// Log - глобальный синглтон логгера.
var Log *zap.Logger = zap.NewNop()

// Initialize инициализирует синглтон логера с необходимым уровнем логирования.
func Initialize(level string) error {
	lvl, err := zap.ParseAtomicLevel(level)
	if err != nil {
		return err
	}

	cfg := zap.NewProductionConfig()
	cfg.Level = lvl

	zl, err := cfg.Build()
	if err != nil {
		return err
	}

	Log = zl
	return nil
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
			zap.String("uri", r.RequestURI),
			zap.String("method", r.Method),
			zap.Int("status", data.status),
			zap.Int("size", data.size),
			zap.Duration("duration", duration),
		)
	})
}
