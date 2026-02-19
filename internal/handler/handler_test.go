package handler

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Dahasolo/urlshortener/internal/mocks"
	"github.com/Dahasolo/urlshortener/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// testLogger создаёт тестовый логгер для использования в тестах.
func testLogger(t *testing.T) *slog.Logger {
	t.Helper()
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// newChiRequest создаёт *http.Request с контекстом Chi, содержащим указанные URL-параметры.
func newChiRequest(method, path string, body io.Reader, params map[string]string) *http.Request {
	req := httptest.NewRequest(method, path, body)
	routeCtx := chi.NewRouteContext()
	for k, v := range params {
		routeCtx.URLParams.Add(k, v)
	}
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx)
	return req.WithContext(ctx)
}

func TestShortenHandler(t *testing.T) {
	tests := []struct {
		name           string
		body           string
		expectedCode   int
		expectLocation bool // ожидаем ли тело с короткой ссылкой
		expectSave     bool
	}{
		{
			name:           "valid POST with URL",
			body:           "https://example.com",
			expectedCode:   http.StatusCreated,
			expectLocation: true,
			expectSave:     true,
		},
		{
			name:           "valid POST with whitespace",
			body:           "	https://example.com ",
			expectedCode:   http.StatusCreated,
			expectLocation: true,
			expectSave:     true,
		},
		{
			name:         "empty body",
			expectedCode: http.StatusBadRequest,
			expectSave:   false,
		},
		{
			name:         "body is whitespace only",
			body:         " \t\n",
			expectedCode: http.StatusBadRequest,
			expectSave:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Подготовка зависимостей
			repo := &mocks.URLRepository{}
			if tt.expectSave {
				repo.On("Save", mock.Anything, mock.Anything).Return(nil).Once()
			}
			svc := service.NewService(repo)
			logger := testLogger(t)
			handler := ShortenHandler(svc, "http://localhost:8080/", logger)

			// Создание фейкового запроса
			req := newChiRequest(http.MethodPost, "/", strings.NewReader(tt.body), nil)
			w := httptest.NewRecorder()

			// Вызов хендлера
			handler(w, req)

			// Проверка статуса
			assert.Equal(t, tt.expectedCode, w.Code)

			// Если ожидаем короткую ссылку — проверяем формат
			if tt.expectLocation {
				assert.Regexp(t, `^http://localhost:8080/[a-zA-Z0-9]+$`, w.Body.String())
			}
			repo.AssertExpectations(t)
		})
	}
}

func TestRedirectHandler(t *testing.T) {
	tests := []struct {
		name         string
		path         string
		prepareRepo  func(r *mocks.URLRepository) // модификация мока перед тестом
		expectedCode int
		expectedLoc  string
	}{
		{
			name: "existing ID", // redirect
			path: "/abc123",
			prepareRepo: func(r *mocks.URLRepository) {
				r.On("Get", "abc123").Return("https://example.com", true).Once()
			},
			expectedCode: http.StatusTemporaryRedirect,
			expectedLoc:  "https://example.com",
		},
		{
			name: "non-existing ID", // 404
			path: "/notfound",
			prepareRepo: func(r *mocks.URLRepository) {
				r.On("Get", "notfound").Return("", false).Once()
			},
			expectedCode: http.StatusNotFound,
		},
		{
			name:         "empty ID", // 400
			path:         "/",
			prepareRepo:  nil,
			expectedCode: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Подготовка зависимостей
			repo := &mocks.URLRepository{}
			if tt.prepareRepo != nil {
				tt.prepareRepo(repo)
			}
			svc := service.NewService(repo)
			logger := testLogger(t)
			handler := RedirectHandler(svc, logger)

			// Создание фейкового запроса
			var params map[string]string
			if len(tt.path) > 1 {
				params = map[string]string{"id": tt.path[1:]}
			}
			req := newChiRequest(http.MethodGet, tt.path, nil, params)
			w := httptest.NewRecorder()

			// Вызов хендлера
			handler(w, req)

			// Проверка статуса
			assert.Equal(t, tt.expectedCode, w.Code)

			// Если ожидаем, что должен быть заголовок Location - проверяем
			if tt.expectedLoc != "" {
				assert.Equal(t, tt.expectedLoc, w.Header().Get("Location"))
			}
			repo.AssertExpectations(t)
		})
	}
}

func TestShortenJSONHandler(t *testing.T) {
	tests := []struct {
		name         string
		body         string
		contentType  string
		prepareRepo  func(r *mocks.URLRepository)
		expectedCode int
		expectJSON   bool // ожидаем ли валидный JSON в ответе
	}{
		{
			name:        "valid JSON",
			body:        `{"url": "https://example.com"}`,
			contentType: "application/json",
			prepareRepo: func(r *mocks.URLRepository) {
				r.On("Save", mock.Anything, "https://example.com").Return(nil).Once()
			},
			expectedCode: http.StatusCreated,
			expectJSON:   true,
		},
		{
			name:         "invalid Content-Type",
			body:         `{"url": "https://example.com"}`,
			contentType:  "text/plain",
			expectedCode: http.StatusBadRequest,
		},
		{
			name:         "missing Content-Type",
			body:         `{"url": "https://example.com"}`,
			expectedCode: http.StatusBadRequest,
		},
		{
			name:         "empty JSON",
			body:         `{}`,
			contentType:  "application/json",
			expectedCode: http.StatusBadRequest,
		},
		{
			name:         "missing url field",
			body:         `{"some_field": "https://example.com"}`,
			contentType:  "application/json",
			expectedCode: http.StatusBadRequest,
		},
		{
			name:         "invalid JSON syntax",
			body:         `{"url": "https://example.com"`,
			contentType:  "application/json",
			expectedCode: http.StatusBadRequest,
		},
		{
			name:         "relative path",
			body:         `{"url": "/path"}`,
			contentType:  "application/json",
			expectedCode: http.StatusBadRequest,
		},
		{
			name:        "URL with path",
			body:        `{"url": "https://example.com/path/to/page"}`,
			contentType: "application/json",
			prepareRepo: func(r *mocks.URLRepository) {
				r.On("Save", mock.Anything, "https://example.com/path/to/page").Return(nil).Once()
			},
			expectedCode: http.StatusCreated,
			expectJSON:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Подготовка зависимостей
			repo := &mocks.URLRepository{}
			if tt.prepareRepo != nil {
				tt.prepareRepo(repo)
			}
			svc := service.NewService(repo)
			logger := testLogger(t)
			handler := ShortenJSONHandler(svc, "http://localhost:8080/", logger)

			// Создание фейкового запроса
			req := newChiRequest(http.MethodPost, "/api/shorten", strings.NewReader(tt.body), nil)
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}
			w := httptest.NewRecorder()

			// Вызов хендлера
			handler(w, req)

			// Проверка статуса
			assert.Equal(t, tt.expectedCode, w.Code)

			// Если ожидаем валидный JSON - проверяем формат
			if tt.expectJSON {
				assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
				assert.Regexp(t, `^{"result":"http://localhost:8080/[a-zA-Z0-9]+"}`, w.Body.String())
			}

			repo.AssertExpectations(t)
		})
	}
}

func TestBatchShortenHandler(t *testing.T) {
	tests := []struct {
		name          string
		body          string
		contentType   string
		prepareRepo   func(r *mocks.URLRepository)
		expectedCode  int
		expectJSON    bool
		expectResults int
	}{
		{
			name:        "valid single url",
			body:        `[{"correlation_id":"a","original_url":"https://example.com"}]`,
			contentType: "application/json",
			prepareRepo: func(r *mocks.URLRepository) {
				r.On("SaveMany", mock.Anything).Return(nil).Once()
			},
			expectedCode:  http.StatusCreated,
			expectJSON:    true,
			expectResults: 1,
		},
		{
			name: "valid multiple urls",
			body: `[
				{"correlation_id":"a","original_url":"https://example1.com"},
				{"correlation_id":"b","original_url":"https://example2.com"}
			]`,
			contentType: "application/json",
			prepareRepo: func(r *mocks.URLRepository) {
				r.On("SaveMany", mock.MatchedBy(func(entries []service.BatchEntry) bool {
					return len(entries) == 2
				})).Return(nil).Once()
			},
			expectedCode:  http.StatusCreated,
			expectJSON:    true,
			expectResults: 2,
		},
		{
			name:          "empty batch",
			body:          `[]`,
			contentType:   "application/json",
			expectedCode:  http.StatusBadRequest,
			expectResults: 0,
		},
		{
			name:          "invalid content type",
			body:          `[{"correlation_id":"a","original_url":"https://example.com"}]`,
			contentType:   "text/plain",
			expectedCode:  http.StatusBadRequest,
			expectResults: 0,
		},
		{
			name:          "invalid json",
			body:          `[{"correlation_id":"a"`,
			contentType:   "application/json",
			expectedCode:  http.StatusBadRequest,
			expectResults: 0,
		},
		{
			name:          "invalid url",
			body:          `[{"correlation_id":"a","original_url":"invalid-url"}]`,
			contentType:   "application/json",
			expectedCode:  http.StatusBadRequest,
			expectResults: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Подготовка зависимостей
			repo := &mocks.URLRepository{}
			if tt.prepareRepo != nil {
				tt.prepareRepo(repo)
			}
			svc := service.NewService(repo)
			logger := testLogger(t)
			handler := BatchShortenHandler(svc, "http://localhost:8080/", logger)

			// Создание фейкового запроса
			req := newChiRequest(http.MethodPost, "/api/shorten/batch", strings.NewReader(tt.body), nil)
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}
			w := httptest.NewRecorder()

			// Вызов хендлера
			handler(w, req)

			// Проверка статуса
			assert.Equal(t, tt.expectedCode, w.Code)

			// Если ожидаем валидный JSON - проверяем формат
			if tt.expectJSON && tt.expectedCode == http.StatusCreated {
				assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

				var responses []struct {
					CorrelationID string `json:"correlation_id"`
					ShortURL      string `json:"short_url"`
				}
				err := json.NewDecoder(w.Body).Decode(&responses)
				require.NoError(t, err)
				assert.Len(t, responses, tt.expectResults)

				for _, resp := range responses {
					assert.Regexp(t, `^http://localhost:8080/[a-zA-Z0-9]+$`, resp.ShortURL)
				}
			}

			repo.AssertExpectations(t)
		})
	}
}
