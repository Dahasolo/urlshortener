package service

import (
	"crypto/rand"

	"github.com/Dahasolo/urlshortener/internal/repository"
)

const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

type Service struct {
	repo repository.URLRepository
}

func NewService(repo repository.URLRepository) *Service {
	return &Service{repo: repo}
}

// Генерация 8-символьного случайного ID из letters.
func (s *Service) generateID() string {
	b := make([]byte, 8)
	_, err := rand.Read(b)
	if err != nil {
		return "FallbackID" // заглушка
	}
	for i := range b {
		// b[i] % 52 - индекс в letters
		b[i] = letters[int(b[i])%len(letters)]
	}
	return string(b)
}

// Cокращение URL и сохранение в репозиторий.
func (s *Service) Shorten(url string) string {
	id := s.generateID()
	s.repo.Save(id, url)
	return id
}

// Поиск оригинального URL по ID.
func (s *Service) Resolve(id string) (string, bool) {
	return s.repo.Get(id)
}
