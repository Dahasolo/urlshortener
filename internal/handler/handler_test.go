package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/Dahasolo/urlshortener/internal/service"
)

// мок-реализация интерфейса repository.URLRepository
type mockRepo struct{ store map[string]string }

func newMockRepo() *mockRepo {
	return &mockRepo{store: make(map[string]string)}
}

func (m *mockRepo) Save(id, url string) {
	m.store[id] = url
}

func (m *mockRepo) Get(id string) (string, bool) {
	url, ok := m.store[id]
	return url, ok
}

func TestShortenHandler(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		body           string
		expectedCode   int
		expectLocation bool // ожидаем ли тело с короткой ссылкой
	}{
		{
			name:           "valid POST with URL",
			method:         http.MethodPost,
			body:           "https://example.com",
			expectedCode:   http.StatusCreated,
			expectLocation: true,
		},
		{
			name:         "empty body",
			method:       http.MethodPost,
			body:         "",
			expectedCode: http.StatusBadRequest,
		},
		{
			name:         "GET method not allowed",
			method:       http.MethodGet,
			body:         "https://example.com",
			expectedCode: http.StatusMethodNotAllowed,
		},
		{
			name:         "PUT method not allowed",
			method:       http.MethodPut,
			body:         "https://example.com",
			expectedCode: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Подготовка зависимостей
			repo := newMockRepo()
			svc := service.NewService(repo)
			handler := ShortenHandler(svc)

			// Создание фейкового запроса
			req := httptest.NewRequest(tt.method, "/", strings.NewReader(tt.body))
			w := httptest.NewRecorder()

			// Вызов хендлера
			handler(w, req)

			// Проверка статуса
			assert.Equal(t, tt.expectedCode, w.Code)

			// Если ожидаем короткую ссылку — проверяем формат
			if tt.expectLocation {
				assert.Regexp(t, `^http://localhost:8080/[a-zA-Z0-9]+$`, w.Body.String())
			}
		})
	}
}

func TestRedirectHandler(t *testing.T) {
	tests := []struct {
		name         string
		method       string
		path         string
		prepareRepo  func(r *mockRepo) // модификация мока перед тестом
		expectedCode int
		expectedLoc  string
	}{
		{
			name:   "existing ID", // redirect
			method: http.MethodGet,
			path:   "/abc123",
			prepareRepo: func(r *mockRepo) {
				r.Save("abc123", "https://example.com")
			},
			expectedCode: http.StatusTemporaryRedirect,
			expectedLoc:  "https://example.com",
		},
		{
			name:         "non-existing ID", // 404
			method:       http.MethodGet,
			path:         "/notfound",
			prepareRepo:  nil,
			expectedCode: http.StatusNotFound,
		},
		{
			name:         "empty ID", // 400
			method:       http.MethodGet,
			path:         "/",
			prepareRepo:  nil,
			expectedCode: http.StatusBadRequest,
		},
		{
			name:   "POST to /{id}", // 405
			method: http.MethodPost,
			path:   "/abc123",
			prepareRepo: func(r *mockRepo) {
				r.Save("abc123", "https://example.com")
			},
			expectedCode: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Подготовка зависимостей
			repo := newMockRepo()
			if tt.prepareRepo != nil {
				tt.prepareRepo(repo)
			}
			svc := service.NewService(repo)
			handler := RedirectHandler(svc)

			// Создание фейкового запроса
			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()

			// Вызов хендлера
			handler(w, req)

			// Проверка статуса
			assert.Equal(t, tt.expectedCode, w.Code)

			// Если ожидаем, что должен быть заголовок Location - проверяем
			if tt.expectedLoc != "" {
				assert.Equal(t, tt.expectedLoc, w.Header().Get("Location"))
			}
		})
	}
}
