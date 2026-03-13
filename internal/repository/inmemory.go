package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/Dahasolo/urlshortener/internal/service"
)

// urlEntry представляет одну запись для сериализации в JSON.
type urlEntry struct {
	ShortURL    string `json:"short_url"`
	OriginalURL string `json:"original_url"`
	UserID      string `json:"user_id"`
	IsDeleted   bool   `json:"is_deleted"`
}

// InMemoryURLRepo — in-memory реализация репозитория для хранения коротких URL.
type InMemoryURLRepo struct {
	urls     map[string]urlEntry
	urlToID  map[string]string
	mu       sync.Mutex
	file     *os.File
	filePath string
}

// NewInMemoryURLRepo создаёт новый экземпляр InMemoryURLRepo.
func NewInMemoryURLRepo(filePath string) (*InMemoryURLRepo, error) {
	repo := &InMemoryURLRepo{
		urls:     make(map[string]urlEntry),
		urlToID:  make(map[string]string),
		filePath: filePath,
	}

	if filePath == "" {
		return repo, nil
	}

	// Создание директории, если она не существует
	if dir := filepath.Dir(filePath); dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory %q: %w", dir, err)
		}
	}

	// Открываем файл
	file, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %q: %w", filePath, err)
	}
	repo.file = file

	// Получаем размер файла
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("failed to stat file %q: %w", filePath, err)
	}

	// Загрузка данных (если файл не пустой)
	if info.Size() > 0 {
		data := make([]byte, info.Size())
		if _, err := io.ReadFull(file, data); err != nil {
			_ = file.Close()
			return nil, fmt.Errorf("failed to read file %q: %w", filePath, err)
		}

		// десериализация данных из JSON
		var entries []urlEntry
		if err := json.Unmarshal(data, &entries); err != nil {
			_ = file.Close()
			return nil, fmt.Errorf("failed to unmarshal JSON from %q: %w", filePath, err)
		}

		// загрузка данных в память
		repo.mu.Lock()
		for _, entry := range entries {
			repo.urls[entry.ShortURL] = entry
			key := entry.OriginalURL + "|" + entry.UserID
			repo.urlToID[key] = entry.ShortURL
		}
		repo.mu.Unlock()
	}

	return repo, nil
}

// saveToFile сохраняет все данные в файл.
func (r *InMemoryURLRepo) saveToFile() error {
	// преобразование данных для сериализации
	entries := make([]urlEntry, 0, len(r.urls))
	for _, entry := range r.urls {
		entries = append(entries, entry)
	}

	// сериализация в JSON
	jsonData, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	// Запись в файл
	if _, err := r.file.Seek(0, 0); err != nil {
		return fmt.Errorf("failed to seek to beginning of file %q: %w", r.filePath, err)
	}
	if err := r.file.Truncate(0); err != nil {
		return fmt.Errorf("failed to truncate file %q: %w", r.filePath, err)
	}
	if _, err := r.file.Write(jsonData); err != nil {
		return fmt.Errorf("failed to write data to file %q: %w", r.filePath, err)
	}
	if err := r.file.Sync(); err != nil {
		return fmt.Errorf("failed to sync file %q: %w", r.filePath, err)
	}

	return nil
}

// Save сохраняет URL по заданному ID.
func (r *InMemoryURLRepo) Save(ctx context.Context, id, url, userID string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// проверка дубликата URL
	key := url + "|" + userID
	if existingID, exists := r.urlToID[key]; exists {
		return &service.ErrURLAlreadyExists{
			ExistingID:  existingID,
			OriginalURL: url,
		}
	}

	// проверка дубликата ID
	if _, exists := r.urls[id]; exists {
		return fmt.Errorf("ID %q for URL %q already exists", id, url)
	}

	entry := urlEntry{
		ShortURL:    id,
		OriginalURL: url,
		UserID:      userID,
		IsDeleted:   false,
	}

	// сохранение в память
	r.urls[id] = entry
	r.urlToID[key] = id

	// запись в файл
	if r.file != nil {
		return r.saveToFile()
	}

	return nil
}

// SaveMany сохраняет несколько коротких URL.
func (r *InMemoryURLRepo) SaveMany(ctx context.Context, entries []service.BatchEntry, userID string) error {
	if len(entries) == 0 {
		return nil
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	for _, entry := range entries {
		// проверка дубликата URL
		key := entry.OriginalURL + "|" + userID
		if _, exists := r.urlToID[key]; exists {
			continue
		}
		// проверка дубликата ID
		if _, exists := r.urls[entry.ID]; exists {
			continue
		}
		// сохранение в память
		newEntry := urlEntry{
			ShortURL:    entry.ID,
			OriginalURL: entry.OriginalURL,
			UserID:      userID,
			IsDeleted:   false,
		}
		r.urls[entry.ID] = newEntry
		r.urlToID[key] = entry.ID
	}

	// запись в файл
	if r.file != nil {
		return r.saveToFile()
	}

	return nil
}

// Get возвращает URL по ID или пустую строку с false, если ID не найден.
func (r *InMemoryURLRepo) Get(ctx context.Context, id string) (service.ResolveResult, bool) {
	select {
	case <-ctx.Done():
		return service.ResolveResult{}, false
	default:
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	entry, ok := r.urls[id]
	if !ok {
		return service.ResolveResult{}, false
	}

	return service.ResolveResult{
		OriginalURL: entry.OriginalURL,
		IsDeleted:   entry.IsDeleted,
	}, true
}

// GetExistingID возвращает существующий ID по оригинальному URL.
func (r *InMemoryURLRepo) GetExistingID(ctx context.Context, url string, userID string) (string, bool) {
	select {
	case <-ctx.Done():
		return "", false
	default:
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	key := url + "|" + userID
	id, exists := r.urlToID[key]
	return id, exists
}

// GetUserURLs возвращает все URL пользователя.
func (r *InMemoryURLRepo) GetUserURLs(ctx context.Context, userID string) ([]service.URLRecord, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	var records []service.URLRecord
	for shortID, entry := range r.urls {
		if entry.IsDeleted {
			continue
		}
		key := entry.OriginalURL + "|" + userID
		if storedID, exists := r.urlToID[key]; exists && storedID == shortID {
			records = append(records, service.URLRecord{
				ShortURL:    shortID,
				OriginalURL: entry.OriginalURL,
			})
		}
	}
	return records, nil
}

// MarkAsDeleted помечает указанные short_url как удалённые для конкретного пользователя.
func (r *InMemoryURLRepo) MarkAsDeleted(ctx context.Context, ids []string, userID string) error {
	if len(ids) == 0 {
		return nil
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	for _, id := range ids {
		entry, exists := r.urls[id]
		if !exists {
			continue
		}

		key := entry.OriginalURL + "|" + userID
		storedID, ok := r.urlToID[key]

		// если записи нет или id не совпадает - пропускаем
		if !ok || storedID != id {
			continue
		}

		// сохранение в память с обновленным флагом
		entry.IsDeleted = true
		r.urls[id] = entry
	}

	// запись в файл
	if r.file != nil {
		return r.saveToFile()
	}

	return nil
}

// Close - корректное закрытие файла при завершении программы.
func (r *InMemoryURLRepo) Close() error {
	if r.file != nil {
		err := r.file.Close()
		r.file = nil
		return err
	}
	return nil
}

// Ping проверяет доступность хранилища.
func (r *InMemoryURLRepo) Ping(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}
