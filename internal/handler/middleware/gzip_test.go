package middleware

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Dahasolo/urlshortener/internal/mocks"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// mockHandler создаёт мок с заданным contentType
func mockHandler(t *testing.T, body, contentType string) *mocks.HandlerMock {
	t.Helper()
	mockHandler := mocks.NewHandlerMock(t)
	mockHandler.EXPECT().
		ServeHTTP(mock.Anything, mock.Anything).
		Run(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", contentType)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(body))
		})
	return mockHandler
}

// gzipRequest создаёт сжатый запрос
func gzipRequest(t *testing.T, method, url, body string) *http.Request {
	t.Helper()
	buf := bytes.NewBuffer(nil)
	zw := gzip.NewWriter(buf)
	_, err := zw.Write([]byte(body))
	require.NoError(t, err)
	require.NoError(t, zw.Close())

	r := httptest.NewRequest(method, url, buf)
	r.Header.Set("Content-Encoding", "gzip")
	return r
}

func TestGzipMiddleware(t *testing.T) {
	const (
		reqJSON  = `{"url":"https://example.com"}`
		respJSON = `{"result":"http://localhost:8080/abc123"}`
		respText = "http://localhost:8080/abc123"
	)

	tests := []struct {
		name                  string
		setup                 func() (*mocks.HandlerMock, *http.Request)
		expectedGzip          bool
		expectedCode          int
		handlerShouldBeCalled bool // должен ли вызываться Handler
		desc                  string
	}{
		{
			name: "decompress_request",
			setup: func() (*mocks.HandlerMock, *http.Request) {
				return mockHandler(t, respJSON, "application/json"), gzipRequest(t, "POST", "/api/shorten", reqJSON)
			},
			expectedGzip:          false,
			expectedCode:          http.StatusOK,
			handlerShouldBeCalled: true,
			desc:                  "распаковывает сжатый запрос",
		},
		{
			name: "compress_json",
			setup: func() (*mocks.HandlerMock, *http.Request) {
				r := httptest.NewRequest("POST", "/api/shorten", bytes.NewBufferString(reqJSON))
				r.Header.Set("Accept-Encoding", "gzip")
				return mockHandler(t, respJSON, "application/json"), r
			},
			expectedGzip:          true,
			expectedCode:          http.StatusOK,
			handlerShouldBeCalled: true,
			desc:                  "сжимает JSON-ответ",
		},
		{
			name: "skip_text",
			setup: func() (*mocks.HandlerMock, *http.Request) {
				r := httptest.NewRequest("POST", "/", bytes.NewBufferString("https://example.com"))
				r.Header.Set("Accept-Encoding", "gzip")
				return mockHandler(t, respText, "text/plain"), r
			},
			expectedGzip:          false,
			expectedCode:          http.StatusOK,
			handlerShouldBeCalled: true,
			desc:                  "не сжимает текстовый ответ",
		},
		{
			name: "bidirectional_compression",
			setup: func() (*mocks.HandlerMock, *http.Request) {
				r := gzipRequest(t, "POST", "/api/shorten", reqJSON)
				r.Header.Set("Accept-Encoding", "gzip")
				return mockHandler(t, respJSON, "application/json"), r
			},
			expectedGzip:          true,
			expectedCode:          http.StatusOK,
			handlerShouldBeCalled: true,
			desc:                  "двустороннее сжатие",
		},
		{
			name: "invalid_gzip",
			setup: func() (*mocks.HandlerMock, *http.Request) {
				r := httptest.NewRequest("POST", "/api/shorten", bytes.NewBufferString(reqJSON))
				r.Header.Set("Content-Encoding", "gzip") // несжатые данные с заголовком сжатия
				return new(mocks.HandlerMock), r
			},
			expectedGzip:          false,
			expectedCode:          http.StatusBadRequest,
			handlerShouldBeCalled: false,
			desc:                  "ошибка на невалидных данных",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockHandler, req := tt.setup()
			mw := GzipMiddleware(mockHandler)

			w := httptest.NewRecorder()
			mw.ServeHTTP(w, req)

			require.Equal(t, tt.expectedCode, w.Code, tt.desc)

			if tt.handlerShouldBeCalled {
				mockHandler.AssertCalled(t, "ServeHTTP", mock.Anything, mock.Anything)
			} else {
				mockHandler.AssertNotCalled(t, "ServeHTTP", mock.Anything, mock.Anything)
			}

			if tt.expectedGzip {
				require.Equal(t, "gzip", w.Header().Get("Content-Encoding"), tt.desc)

				zr, err := gzip.NewReader(w.Body)
				require.NoError(t, err)
				body, err := io.ReadAll(zr)
				require.NoError(t, err)
				require.JSONEq(t, respJSON, string(body), tt.desc)
			} else {
				require.Empty(t, w.Header().Get("Content-Encoding"), tt.desc)
			}
		})
	}
}

func TestShouldCompress(t *testing.T) {
	tests := []struct {
		ct       string
		expected bool
		desc     string
	}{
		{"application/json", true, "JSON сжимается"},
		{"application/json; charset=utf-8", true, "JSON с charset сжимается"},
		{"text/html", true, "HTML сжимается"},
		{"text/plain", false, "текст не сжимается"},
		{"image/png", false, "изображение не сжимается"},
	}

	for _, tt := range tests {
		t.Run(tt.ct, func(t *testing.T) {
			require.Equal(t, tt.expected, shouldCompress(tt.ct), tt.desc)
		})
	}
}
