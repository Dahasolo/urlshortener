package repository

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"maps"
	"os"
	"path/filepath"
	"sync"
)

var ErrAlreadyExists = errors.New("ID already exists")

// urlEntry представляет одну запись для сериализации в JSON
type urlEntry struct {
	ShortURL    string `json:"short_url"`
	OriginalURL string `json:"original_url"`
}

// InMemoryURLRepo — in-memory реализация репозитория для хранения коротких URL.
type InMemoryURLRepo struct {
	urls     map[string]string
	mu       sync.Mutex
	filePath string
}

// NewInMemoryURLRepo создаёт новый экземпляр InMemoryURLRepo.
func NewInMemoryURLRepo(filePath string) *InMemoryURLRepo {
	return &InMemoryURLRepo{
		urls:     make(map[string]string),
		filePath: filePath,
	}
}

// Save сохраняет URL по заданному ID.
func (r *InMemoryURLRepo) Save(id, url string) error {
	r.mu.Lock()

	// сохранение в память
	if _, exists := r.urls[id]; exists {
		r.mu.Unlock()
		return fmt.Errorf("ID %s for URL %q already exists: %w", id, url, ErrAlreadyExists)
	}
	r.urls[id] = url

	// копирование данных
	urlsCopy := make(map[string]string, len(r.urls))
	maps.Copy(urlsCopy, r.urls)
	r.mu.Unlock()

	// сохранение на диск
	if r.filePath != "" {
		if err := r.saveToFile(urlsCopy); err != nil {
			log.Printf("warning: failed to save data to %q: %v", r.filePath, err)
		}
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

// LoadFromFile загружает данные из файла при старте сервера
func (r *InMemoryURLRepo) LoadFromFile() error {
	if r.filePath == "" {
		return nil
	}

	data, err := os.ReadFile(r.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("info: file %q does not exist", r.filePath)
			return nil
		}
		return fmt.Errorf("failed to read file %q: %w", r.filePath, err)
	}

	// десериализация данных из JSON
	var entries []urlEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return fmt.Errorf("failed to unmarshal JSON from %q: %w", r.filePath, err)
	}

	// загрузка данных в память
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, entry := range entries {
		r.urls[entry.ShortURL] = entry.OriginalURL
	}

	return nil
}

// saveToFile сохраняет все данные в файл
func (r *InMemoryURLRepo) saveToFile(data map[string]string) error {
	// преобразование данных для сериализации
	entries := make([]urlEntry, 0, len(data))
	for shortURL, originalURL := range data {
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

	// Создаём директорию, если она не существует
	dir := filepath.Dir(r.filePath)
	if dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %q: %w", dir, err)
		}
	}

	// запись данных в файл
	if err := os.WriteFile(r.filePath, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write file %q: %w", r.filePath, err)
	}

	return nil
}
