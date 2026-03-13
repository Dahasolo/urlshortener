package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Dahasolo/urlshortener/internal/auth"
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
				repo.On("Save", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
			}
			logger := testLogger(t)
			svc := service.NewService(repo, logger)
			handler := ShortenHandler(svc, "http://localhost:8080/", "test-secret-key", logger)

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
				r.On("Get", mock.Anything, "abc123").Return(service.ResolveResult{
					OriginalURL: "https://example.com",
					IsDeleted:   false,
				}, true).Once()
			},
			expectedCode: http.StatusTemporaryRedirect,
			expectedLoc:  "https://example.com",
		},
		{
			name: "non-existing ID", // 404
			path: "/notfound",
			prepareRepo: func(r *mocks.URLRepository) {
				r.On("Get", mock.Anything, "notfound").Return(service.ResolveResult{}, false).Once()
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
			logger := testLogger(t)
			svc := service.NewService(repo, logger)
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
				r.On("Save", mock.Anything, mock.Anything, "https://example.com", mock.Anything).Return(nil).Once()
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
				r.On("Save", mock.Anything, mock.Anything, "https://example.com/path/to/page", mock.Anything).Return(nil).Once()
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
			logger := testLogger(t)
			svc := service.NewService(repo, logger)
			handler := ShortenJSONHandler(svc, "http://localhost:8080/", "test-secret-key", logger)

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
				r.On("SaveMany", mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
				r.On("GetExistingID", mock.Anything, "https://example.com", mock.Anything).Return("", false).Once()
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
				r.On("SaveMany", mock.Anything, mock.MatchedBy(func(entries []service.BatchEntry) bool {
					return len(entries) == 2
				}), mock.Anything).Return(nil).Once()
				r.On("GetExistingID", mock.Anything, "https://example1.com", mock.Anything).Return("", false).Once()
				r.On("GetExistingID", mock.Anything, "https://example2.com", mock.Anything).Return("", false).Once()
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
			logger := testLogger(t)
			svc := service.NewService(repo, logger)
			handler := BatchShortenHandler(svc, "http://localhost:8080/", "test-secret-key", logger)

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

func TestShortenHandler_DuplicateURL(t *testing.T) {
	tests := []struct {
		name         string
		body         string
		existingID   string
		originalURL  string
		expectedCode int
		expectedBody string
	}{
		{
			name:         "duplicate url",
			body:         "https://example.com",
			existingID:   "abc123",
			originalURL:  "https://example.com",
			expectedCode: http.StatusConflict,
			expectedBody: "http://localhost:8080/abc123",
		},
		{
			name:         "duplicate url with whitespace",
			body:         "  https://example.com  ",
			existingID:   "xyz789",
			originalURL:  "https://example.com",
			expectedCode: http.StatusConflict,
			expectedBody: "http://localhost:8080/xyz789",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Подготовка зависимостей
			repo := &mocks.URLRepository{}

			// при ошибке дубликата сервис вернёт ErrURLAlreadyExists
			repo.On("Save", mock.Anything, mock.Anything, tt.originalURL, mock.Anything).
				Return(&service.ErrURLAlreadyExists{
					ExistingID:  tt.existingID,
					OriginalURL: tt.originalURL,
				}).Once()

			logger := testLogger(t)
			svc := service.NewService(repo, logger)
			handler := ShortenHandler(svc, "http://localhost:8080/", "test-secret-key", logger)

			// Создание фейкового запроса
			req := newChiRequest(http.MethodPost, "/", strings.NewReader(tt.body), nil)
			w := httptest.NewRecorder()

			// Вызов хендлера
			handler(w, req)

			// Проверка статуса
			assert.Equal(t, tt.expectedCode, w.Code)

			// Проверка формата
			assert.Equal(t, "text/plain", w.Header().Get("Content-Type"))
			assert.Equal(t, tt.expectedBody, w.Body.String())

			repo.AssertExpectations(t)
		})
	}
}

func TestShortenJSONHandler_DuplicateURL(t *testing.T) {
	tests := []struct {
		name         string
		body         string
		existingID   string
		originalURL  string
		expectedCode int
	}{
		{
			name:         "duplicate url json",
			body:         `{"url":"https://example.com"}`,
			existingID:   "json123",
			originalURL:  "https://example.com",
			expectedCode: http.StatusConflict,
		},
		{
			name:         "duplicate url json with path",
			body:         `{"url":"https://example.com/path/to/page"}`,
			existingID:   "path456",
			originalURL:  "https://example.com/path/to/page",
			expectedCode: http.StatusConflict,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Подготовка зависимостей
			repo := &mocks.URLRepository{}

			// при ошибке дубликата сервис вернёт ErrURLAlreadyExists
			repo.On("Save", mock.Anything, mock.Anything, tt.originalURL, mock.Anything).
				Return(&service.ErrURLAlreadyExists{
					ExistingID:  tt.existingID,
					OriginalURL: tt.originalURL,
				}).Once()

			logger := testLogger(t)
			svc := service.NewService(repo, logger)
			handler := ShortenJSONHandler(svc, "http://localhost:8080/", "test-secret-key", logger)

			// Создание фейкового запроса
			req := newChiRequest(http.MethodPost, "/api/shorten", strings.NewReader(tt.body), nil)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			// Вызов хендлера
			handler(w, req)

			// Проверка статуса ответа
			assert.Equal(t, tt.expectedCode, w.Code)

			// Проверка формата
			assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

			var resp ShortenResponse
			err := json.NewDecoder(w.Body).Decode(&resp)
			require.NoError(t, err)

			expectedURL := fmt.Sprintf("http://localhost:8080/%s", tt.existingID)
			assert.Equal(t, expectedURL, resp.Result)

			repo.AssertExpectations(t)
		})
	}
}

func TestShortenHandler_ServiceError(t *testing.T) {
	tests := []struct {
		name         string
		body         string
		mockError    error
		expectedCode int
	}{
		{
			name:         "database error",
			body:         "https://example.com",
			mockError:    fmt.Errorf("database connection failed"),
			expectedCode: http.StatusInternalServerError,
		},
		{
			name:         "generic error",
			body:         "https://test.com",
			mockError:    errors.New("unexpected error"),
			expectedCode: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Подготовка зависимостей
			repo := &mocks.URLRepository{}

			// непредвиденная ошибка
			repo.On("Save", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return(tt.mockError).Times(10)

			logger := testLogger(t)
			svc := service.NewService(repo, logger)
			handler := ShortenHandler(svc, "http://localhost:8080/", "test-secret-key", logger)

			// Создание фейкового запроса
			req := newChiRequest(http.MethodPost, "/", strings.NewReader(tt.body), nil)
			w := httptest.NewRecorder()

			// Вызов хендлера
			handler(w, req)

			// Проверка статуса ответа
			assert.Equal(t, tt.expectedCode, w.Code)

			repo.AssertExpectations(t)
		})
	}
}

func TestUserURLsHandler(t *testing.T) {
	const testSecretKey = "test-secret-key"
	const testBaseURL = "http://localhost:8080/"

	tests := []struct {
		name         string
		cookie       string
		prepareRepo  func(r *mocks.URLRepository)
		expectedCode int
		expectCookie bool
		expectJSON   bool
	}{
		{
			name: "new user no urls",
			prepareRepo: func(r *mocks.URLRepository) {
				r.On("GetUserURLs", mock.Anything, mock.Anything).Return([]service.URLRecord{}, nil).Once()
			},
			expectedCode: http.StatusNoContent,
			expectCookie: true,
		},
		{
			name: "existing user with urls",
			prepareRepo: func(r *mocks.URLRepository) {
				r.On("GetUserURLs", mock.Anything, mock.Anything).Return([]service.URLRecord{
					{ShortURL: "abc123", OriginalURL: "https://example.com"},
				}, nil).Once()
			},
			expectedCode: http.StatusOK,
			expectCookie: true,
			expectJSON:   true,
		},
		{
			name: "valid cookie with empty urls list",
			cookie: func() string {
				userID := "test-user-12345"
				token, _ := auth.SignToken(userID, testSecretKey)
				return fmt.Sprintf("%s=%s", auth.CookieName, token)
			}(),
			prepareRepo: func(r *mocks.URLRepository) {
				r.On("GetUserURLs", mock.Anything, "test-user-12345").Return([]service.URLRecord{}, nil).Once()
			},
			expectedCode: http.StatusNoContent,
		},
		{
			name:         "invalid cookie",
			cookie:       "user_auth=invalid.signature",
			prepareRepo:  func(r *mocks.URLRepository) {},
			expectedCode: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Подготовка зависимостей
			repo := &mocks.URLRepository{}
			if tt.prepareRepo != nil {
				tt.prepareRepo(repo)
			}
			logger := testLogger(t)
			svc := service.NewService(repo, logger)
			handler := UserURLsHandler(svc, testBaseURL, testSecretKey, logger)

			// Создание фейкового запроса
			req := httptest.NewRequest(http.MethodGet, "/api/user/urls", nil)
			if tt.cookie != "" {
				req.Header.Set("Cookie", tt.cookie)
			}
			w := httptest.NewRecorder()

			// Вызов хендлера
			handler(w, req)

			// Проверка статуса ответа
			assert.Equal(t, tt.expectedCode, w.Code)

			// Если ожидаем куки - проверяем
			if tt.expectCookie {
				resp := w.Result()
				defer resp.Body.Close()
				cookies := resp.Cookies()
				require.NotEmpty(t, cookies)
				assert.Equal(t, "user_auth", cookies[0].Name)
			} else {
				resp := w.Result()
				defer resp.Body.Close()
				for _, c := range resp.Cookies() {
					assert.NotEqual(t, "user_auth", c.Name)
				}
			}

			// Если ожидаем валидный JSON - проверяем формат
			if tt.expectJSON {
				assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
				var resp []UserURLResponse
				err := json.NewDecoder(w.Body).Decode(&resp)
				require.NoError(t, err)
				assert.NotEmpty(t, resp)
			} else {
				assert.Empty(t, w.Body.String())
			}

			repo.AssertExpectations(t)
		})
	}
}

func TestDeleteUserURLsHandler(t *testing.T) {
	const testSecretKey = "test-secret-key"

	tests := []struct {
		name         string
		body         string
		contentType  string
		cookie       string
		prepareRepo  func(r *mocks.URLRepository)
		expectedCode int
	}{
		{
			name:        "valid delete request",
			body:        `["abc123", "xyz789"]`,
			contentType: "application/json",
			cookie: func() string {
				userID := "test-user-12345"
				token, _ := auth.SignToken(userID, testSecretKey)
				return fmt.Sprintf("%s=%s", auth.CookieName, token)
			}(),
			prepareRepo: func(r *mocks.URLRepository) {
				r.On("Close").Return(nil).Once()
				r.On("MarkAsDeleted", mock.Anything, mock.Anything, mock.Anything).
					Return(nil).Once()
			},
			expectedCode: http.StatusAccepted,
		},
		{
			name:         "invalid content type",
			body:         `["abc123"]`,
			contentType:  "text/plain",
			prepareRepo:  func(r *mocks.URLRepository) { r.On("Close").Return(nil).Once() },
			expectedCode: http.StatusBadRequest,
		},
		{
			name:        "empty batch",
			body:        `[]`,
			contentType: "application/json",
			cookie: func() string {
				userID := "test-user-12345"
				token, _ := auth.SignToken(userID, testSecretKey)
				return fmt.Sprintf("%s=%s", auth.CookieName, token)
			}(),
			prepareRepo:  func(r *mocks.URLRepository) { r.On("Close").Return(nil).Once() },
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

			logger := testLogger(t)
			svc := service.NewService(repo, logger)

			handler := DeleteUserURLsHandler(svc, testSecretKey, logger)

			// Создание фейкового запроса
			req := httptest.NewRequest(http.MethodDelete, "/api/user/urls",
				strings.NewReader(tt.body))
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}
			if tt.cookie != "" {
				req.Header.Set("Cookie", tt.cookie)
			}
			w := httptest.NewRecorder()

			// Вызов хендлера
			handler(w, req)

			// Проверка статуса
			assert.Equal(t, tt.expectedCode, w.Code)

			// время на обработку запроса
			if tt.expectedCode == http.StatusAccepted {
				time.Sleep(50 * time.Millisecond)
			}
			svc.Close()
			repo.AssertExpectations(t)
		})
	}
}

func TestRedirectHandler_DeletedURL(t *testing.T) {
	tests := []struct {
		name         string
		path         string
		prepareRepo  func(r *mocks.URLRepository)
		expectedCode int
		expectedLoc  string
	}{
		{
			name: "deleted url",
			path: "/deleted123",
			prepareRepo: func(r *mocks.URLRepository) {
				r.On("Close").Return(nil).Once()
				r.On("Get", mock.Anything, "deleted123").
					Return(service.ResolveResult{
						OriginalURL: "https://example.com",
						IsDeleted:   true,
					}, true).Once()
			},
			expectedCode: http.StatusGone,
		},
		{
			name: "active url",
			path: "/active123",
			prepareRepo: func(r *mocks.URLRepository) {
				r.On("Close").Return(nil).Once()
				r.On("Get", mock.Anything, "active123").
					Return(service.ResolveResult{
						OriginalURL: "https://example.com",
						IsDeleted:   false,
					}, true).Once()
			},
			expectedCode: http.StatusTemporaryRedirect,
			expectedLoc:  "https://example.com",
		},
		{
			name: "non existing url",
			path: "/notfound",
			prepareRepo: func(r *mocks.URLRepository) {
				r.On("Close").Return(nil).Once()
				r.On("Get", mock.Anything, "notfound").
					Return(service.ResolveResult{}, false).Once()
			},
			expectedCode: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Подготовка зависимостей
			repo := &mocks.URLRepository{}
			if tt.prepareRepo != nil {
				tt.prepareRepo(repo)
			}

			logger := testLogger(t)
			svc := service.NewService(repo, logger)

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
			svc.Close()
			repo.AssertExpectations(t)
		})
	}
}
