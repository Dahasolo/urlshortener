package repository

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

// urlEntry представляет одну запись для сериализации в JSON.
type urlEntry struct {
	ShortURL    string `json:"short_url"`
	OriginalURL string `json:"original_url"`
}

// InMemoryURLRepo — in-memory реализация репозитория для хранения коротких URL.
type InMemoryURLRepo struct {
	urls     map[string]string
	mu       sync.Mutex
	file     *os.File
	filePath string
}

// NewInMemoryURLRepo создаёт новый экземпляр InMemoryURLRepo.
func NewInMemoryURLRepo(filePath string) (*InMemoryURLRepo, error) {
	repo := &InMemoryURLRepo{
		urls:     make(map[string]string),
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
			repo.urls[entry.ShortURL] = entry.OriginalURL
		}
		repo.mu.Unlock()
	}

	return repo, nil
}

// saveToFile сохраняет все данные в файл.
func (r *InMemoryURLRepo) saveToFile() error {
	// преобразование данных для сериализации
	entries := make([]urlEntry, 0, len(r.urls))
	for shortURL, originalURL := range r.urls {
		entries = append(entries, urlEntry{
			ShortURL:    shortURL,
			OriginalURL: originalURL,
		})
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
func (r *InMemoryURLRepo) Save(id, url string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// сохранение в память
	if _, exists := r.urls[id]; exists {
		return fmt.Errorf("ID %q for URL %q already exists", id, url)
	}
	r.urls[id] = url

	// запись в файл
	if r.file != nil {
		return r.saveToFile()
	}

	return nil
}

// Get возвращает URL по ID или пустую строку с false, если ID не найден.
func (r *InMemoryURLRepo) Get(id string) (string, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	url, ok := r.urls[id]
	return url, ok
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
func (r *InMemoryURLRepo) Ping() error {
	return nil
}
