package service

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

// URLRepository определяет операции для работы с хранилищем коротких URL.
type URLRepository interface {
	Save(ctx context.Context, id, url string, userID string) error
	SaveMany(ctx context.Context, entries []BatchEntry, userID string) error
	Get(ctx context.Context, id string) (ResolveResult, bool)
	GetExistingID(ctx context.Context, url string, userID string) (string, bool)
	GetUserURLs(ctx context.Context, userID string) ([]URLRecord, error)
	MarkAsDeleted(ctx context.Context, ids []string, userID string) error
	Close() error
	Ping(ctx context.Context) error
}

// URLRecord - структура для ответа хендлера /api/user/urls.
type URLRecord struct {
	ShortURL    string `json:"short_url"`
	OriginalURL string `json:"original_url"`
}

// ResolveResult - данные для редиректа.
type ResolveResult struct {
	OriginalURL string
	IsDeleted   bool
}

// deleteRequest - запрос на удаление URL.
type deleteRequest struct {
	ids    []string
	userID string
}

// Service реализует сервис сокращения URL.
type Service struct {
	repo     URLRepository
	logger   *slog.Logger
	ctx      context.Context
	cancel   context.CancelFunc
	deleteCh chan deleteRequest
	wg       sync.WaitGroup
}

// NewService создаёт новый экземпляр Service с заданным репозиторием.
func NewService(repo URLRepository, logger *slog.Logger) *Service {
	ctx, cancel := context.WithCancel(context.Background())

	svc := &Service{
		repo:     repo,
		logger:   logger,
		ctx:      ctx,
		cancel:   cancel,
		deleteCh: make(chan deleteRequest, 100),
	}

	svc.wg.Add(1)
	go svc.deleteWorker()

	return svc
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

// deleteWorker - воркер для обработки запросов на удаление URL.
func (s *Service) deleteWorker() {
	defer s.wg.Done()

	var batch []deleteRequest
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case req, ok := <-s.deleteCh:
			if !ok {
				if len(batch) > 0 {
					s.flushBatch(batch)
				}
				return
			}

			batch = append(batch, req)

			if len(batch) >= 10 {
				s.flushBatch(batch)
				batch = nil
			}

		case <-ticker.C:
			if len(batch) > 0 {
				s.flushBatch(batch)
				batch = nil
			}

		case <-s.ctx.Done():
			s.logger.Info("delete worker stopping", "reason", s.ctx.Err())
			if len(batch) > 0 {
				s.flushBatch(batch)
			}
			return
		}
	}
}

// flushBatch выполняет пакетное обновление для накопленных запросов.
func (s *Service) flushBatch(requests []deleteRequest) {
	// группировка по userID
	byUser := make(map[string][]string)
	for _, req := range requests {
		byUser[req.userID] = append(byUser[req.userID], req.ids...)
	}

	for userID, ids := range byUser {
		ctx, cancel := context.WithTimeout(s.ctx, 3*time.Second)
		if err := s.repo.MarkAsDeleted(ctx, ids, userID); err != nil {
			s.logger.Error("batch delete failed",
				"user_id", userID,
				"ids_count", len(ids),
				"error", err)
		}
		cancel()
	}

	s.logger.Debug("batch delete completed", "total_requests", len(requests))
}

// DeleteUserURLs — асинхронное удаление URL.
func (s *Service) DeleteUserURLs(ids []string, userID string) {
	select {
	case s.deleteCh <- deleteRequest{ids: ids, userID: userID}:
		s.logger.Debug("delete request queued", "user_id", userID, "ids_count", len(ids))

	case <-s.ctx.Done():
		s.logger.Info("delete request dropped, service stopping", "user_id", userID)

	default:
		s.logger.Info("delete queue full, dropping request", "user_id", userID, "queue_size", len(s.deleteCh))
	}
}

// Shorten сокращает URL и сохраняет в репозиторий.
func (s *Service) Shorten(ctx context.Context, url string, userID string) (string, error) {
	if url == "" {
		return "", fmt.Errorf("empty URL not allowed")
	}

	const maxRetries = 10
	for range maxRetries {
		id, err := s.generateID()
		if err != nil {
			return "", fmt.Errorf("generate ID failed: %w", err)
		}

		err = s.repo.Save(ctx, id, url, userID)
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
func (s *Service) BatchShorten(ctx context.Context, requests []BatchRequest, userID string) ([]BatchResult, error) {
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

	if err := s.repo.SaveMany(ctx, entries, userID); err != nil {
		return nil, fmt.Errorf("failed to save batch: %w", err)
	}

	for i := range results {
		if origURL := requests[i].OriginalURL; origURL != "" {
			if existingID, exists := s.repo.GetExistingID(ctx, origURL, userID); exists {
				results[i].ID = existingID
			}
		}
	}

	return results, nil
}

// GetUserURLs возвращает все URL пользователя.
func (s *Service) GetUserURLs(ctx context.Context, userID string) ([]URLRecord, error) {
	return s.repo.GetUserURLs(ctx, userID)
}

// Resolve возвращает оригинальный URL по короткому ID.
func (s *Service) Resolve(ctx context.Context, id string) (ResolveResult, bool) {
	return s.repo.Get(ctx, id)
}

// Ping проверяет доступность хранилища через репозиторий.
func (s *Service) Ping(ctx context.Context) error {
	if s.repo == nil {
		return fmt.Errorf("repository is nil")
	}
	return s.repo.Ping(ctx)
}

// Close завершает работу сервиса, останавливая фоновые горутины.
func (s *Service) Close() error {
	close(s.deleteCh)
	s.wg.Wait()
	s.cancel()

	if s.repo != nil {
		return s.repo.Close()
	}

	return nil
}
