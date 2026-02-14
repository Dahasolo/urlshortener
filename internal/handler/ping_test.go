package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Dahasolo/urlshortener/internal/mocks"
	"github.com/Dahasolo/urlshortener/internal/service"

	"github.com/stretchr/testify/assert"
)

func TestPingHandler(t *testing.T) {
	tests := []struct {
		name         string
		prepareRepo  func(r *mocks.URLRepository)
		expectedCode int
	}{
		{
			name: "successful_ping",
			prepareRepo: func(r *mocks.URLRepository) {
				r.On("Ping").Return(nil).Once()
			},
			expectedCode: http.StatusOK,
		},
		{
			name: "failed_ping",
			prepareRepo: func(r *mocks.URLRepository) {
				r.On("Ping").Return(assert.AnError).Once()
			},
			expectedCode: http.StatusInternalServerError,
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
			handler := PingHandler(svc)

			// Создание фейкового запроса
			req := httptest.NewRequest(http.MethodGet, "/ping", nil)
			w := httptest.NewRecorder()

			// Вызов хендлера
			handler(w, req)

			// Проверка статуса
			assert.Equal(t, tt.expectedCode, w.Code)

			repo.AssertExpectations(t)
		})
	}
}
