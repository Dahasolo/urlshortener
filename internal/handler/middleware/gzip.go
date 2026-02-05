package middleware

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
)

// compressReader реализует интерфейс io.ReadCloser
// и позволяет декомпрессировать получаемые от клиента данные
type compressReader struct {
	r  io.ReadCloser
	zr *gzip.Reader
}

func newCompressReader(r io.ReadCloser) (*compressReader, error) {
	zr, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}

	return &compressReader{
		r:  r,
		zr: zr,
	}, nil
}

func (c compressReader) Read(p []byte) (n int, err error) {
	return c.zr.Read(p)
}

func (c *compressReader) Close() error {
	if err := c.r.Close(); err != nil {
		return err
	}
	return c.zr.Close()
}

// shouldCompress определяет, стоит ли сжимать ответ по Content-Type.
func shouldCompress(contentType string) bool {
	return strings.HasPrefix(contentType, "application/json") ||
		strings.HasPrefix(contentType, "text/html")
}

// compressResponseWriter реализует интерфейс http.ResponseWriter
// и позволяет сжимать ответы для определённых типов контента
type compressResponseWriter struct {
	w              http.ResponseWriter
	zw             *gzip.Writer
	shouldCompress bool
}

func newCompressResponseWriter(w http.ResponseWriter) *compressResponseWriter {
	return &compressResponseWriter{
		w:  w,
		zw: gzip.NewWriter(w),
	}
}

func (c *compressResponseWriter) Header() http.Header {
	return c.w.Header()
}

func (c *compressResponseWriter) Write(p []byte) (int, error) {
	if c.shouldCompress {
		return c.zw.Write(p)
	}
	return c.w.Write(p)
}

func (c *compressResponseWriter) WriteHeader(statusCode int) {
	// Только для успешных ответов (2xx)
	if statusCode < 300 {
		c.shouldCompress = shouldCompress(c.w.Header().Get("Content-Type"))
		if c.shouldCompress {
			c.w.Header().Set("Content-Encoding", "gzip")
		}
	}
	c.w.WriteHeader(statusCode)
}

func (c *compressResponseWriter) Close() error {
	if c.shouldCompress {
		return c.zw.Close()
	}
	return nil
}

// GzipMiddleware - middleware-обработчик для сжатия ответов и распаковки запросов.
func GzipMiddleware(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contentEncoding := r.Header.Get("Content-Encoding")
		if strings.Contains(contentEncoding, "gzip") {
			cr, err := newCompressReader(r.Body)
			if err != nil {
				http.Error(w, "failed to decompress request", http.StatusBadRequest)
				return
			}
			r.Body = cr
			defer cr.Close()
		}
		ow := w
		acceptEncoding := r.Header.Get("Accept-Encoding")
		if strings.Contains(acceptEncoding, "gzip") {
			cw := newCompressResponseWriter(w)
			ow = cw
			defer cw.Close()
		}
		h.ServeHTTP(ow, r)
	})
}
