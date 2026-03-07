package service

import (
	"crypto/rand"
	"errors"
	"fmt"
)

const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

// URLRepository определяет операции для работы с хранилищем коротких URL.
type URLRepository interface {
	Save(id, url string, userID string) error
	SaveMany(entries []BatchEntry, userID string) error
	Get(id string) (string, bool)
	GetExistingID(url string, userID string) (string, bool)
	GetUserURLs(userID string) ([]URLRecord, error)
	Close() error
	Ping() error
}

// URLRecord - структура для ответа хендлера /api/user/urls.
type URLRecord struct {
	ShortURL    string `json:"short_url"`
	OriginalURL string `json:"original_url"`
}

// Service реализует сервис сокращения URL.
type Service struct {
	repo URLRepository
}

// NewService создаёт новый экземпляр Service с заданным репозиторием.
func NewService(repo URLRepository) *Service {
	return &Service{repo: repo}
}

// BatchEntry - одна запись для множественной вставки.
type BatchEntry struct {
	ID          string
	OriginalURL string
}

// BatchRequest - запрос на сокращение одного URL в батче.
type BatchRequest struct {
	CorrelationID string
	OriginalURL   string
}

// BatchResult - результат сокращения одного URL в батче.
type BatchResult struct {
	CorrelationID string
	ID            string
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
func (s *Service) Shorten(url string, userID string) (string, error) {
	if url == "" {
		return "", fmt.Errorf("empty URL not allowed")
	}

	const maxRetries = 10
	for range maxRetries {
		id, err := s.generateID()
		if err != nil {
			return "", fmt.Errorf("generate ID failed: %w", err)
		}

		err = s.repo.Save(id, url, userID)
		if err == nil {
			return id, nil
		}

		var alreadyExists *ErrURLAlreadyExists
		if errors.As(err, &alreadyExists) {
			return "", err
		}
	}

	return "", fmt.Errorf("failed to generate unique ID after %d attempts", maxRetries)
}

// BatchShorten сокращает несколько URL за один вызов.
func (s *Service) BatchShorten(requests []BatchRequest, userID string) ([]BatchResult, error) {
	if len(requests) == 0 {
		return nil, fmt.Errorf("empty batch not allowed")
	}

	entries := make([]BatchEntry, 0, len(requests))
	results := make([]BatchResult, 0, len(requests))
	generatedIDs := make(map[string]struct{}, len(requests))

	for _, req := range requests {
		if req.OriginalURL == "" {
			continue
		}

		const maxRetries = 10
		var id string
		var err error

		for range maxRetries {
			id, err = s.generateID()
			if err != nil {
				return nil, fmt.Errorf("generate ID failed: %w", err)
			}
			if _, exists := generatedIDs[id]; !exists {
				generatedIDs[id] = struct{}{}
				break
			}
		}
		if id == "" {
			return nil, fmt.Errorf("failed to generate unique ID after %d attempts", maxRetries)
		}

		entries = append(entries, BatchEntry{ID: id, OriginalURL: req.OriginalURL})
		results = append(results, BatchResult{CorrelationID: req.CorrelationID, ID: id})
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("no URLs were shortened")
	}

	if err := s.repo.SaveMany(entries, userID); err != nil {
		return nil, fmt.Errorf("failed to save batch: %w", err)
	}

	for i := range results {
		if origURL := requests[i].OriginalURL; origURL != "" {
			if existingID, exists := s.repo.GetExistingID(origURL, userID); exists {
				results[i].ID = existingID
			}
		}
	}

	return results, nil
}

// GetUserURLs возвращает все URL пользователя.
func (s *Service) GetUserURLs(userID string) ([]URLRecord, error) {
	return s.repo.GetUserURLs(userID)
}

// Resolve возвращает оригинальный URL по короткому ID.
func (s *Service) Resolve(id string) (string, bool) {
	return s.repo.Get(id)
}

// Ping проверяет доступность хранилища через репозиторий.
func (s *Service) Ping() error {
	if s.repo == nil {
		return fmt.Errorf("repository is nil")
	}
	return s.repo.Ping()
}

// Close закрывает соединение с хранилищем через репозиторий.
func (s *Service) Close() error {
	if s.repo == nil {
		return fmt.Errorf("repository is nil")
	}
	return s.repo.Close()
}
