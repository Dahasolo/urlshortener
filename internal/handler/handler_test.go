package handler

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Dahasolo/urlshortener/internal/mocks"
	"github.com/Dahasolo/urlshortener/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

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
			handler := ShortenHandler(svc, "http://localhost:8080/")

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
			handler := RedirectHandler(svc)

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
