package service

import (
	"crypto/rand"
	"fmt"
)

const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

// URLRepository определяет операции для работы с хранилищем коротких URL.
type URLRepository interface {
	Save(id, url string) error
	Get(id string) (string, bool)
}

// Service реализует сервис сокращения URL.
type Service struct {
	repo URLRepository
}

// NewService создаёт новый экземпляр Service с заданным репозиторием.
func NewService(repo URLRepository) *Service {
	return &Service{repo: repo}
}

// generateID генерирует 8-символьный случайный ID из letters.
func (s *Service) generateID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}
	for i := range b {
		// b[i] % 52 - индекс в letters
		b[i] = letters[int(b[i])%len(letters)]
	}
	return string(b), nil
}

// Shorten сокращает URL и сохраняет в репозиторий.
func (s *Service) Shorten(url string) (string, error) {
	const maxRetries = 10
	for i := 0; i < maxRetries; i++ {
		id, err := s.generateID()
		if err != nil {
			return "", fmt.Errorf("generate ID failed: %w", err)
		}
		if err := s.repo.Save(id, url); err == nil {
			return id, nil
		}
	}
	return "", fmt.Errorf("failed to generate unique ID after %d attempts", maxRetries)
}

// Resolve возвращает оригинальный URL по короткому ID.
func (s *Service) Resolve(id string) (string, bool) {
	return s.repo.Get(id)
}
