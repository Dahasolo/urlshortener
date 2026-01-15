package repository

import (
	"errors"
	"sync"
)

var ErrAlreadyExists = errors.New("ID already exists")

// InMemoryURLRepo — in-memory реализация репозитория для хранения коротких URL.
type InMemoryURLRepo struct {
	urls map[string]string
	mu   sync.Mutex
}

// NewInMemoryURLRepo создаёт новый экземпляр InMemoryURLRepo.
func NewInMemoryURLRepo() *InMemoryURLRepo {
	return &InMemoryURLRepo{
		urls: make(map[string]string),
	}
}

// Save сохраняет URL по заданному ID.
func (r *InMemoryURLRepo) Save(id, url string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.urls[id]; exists {
		return ErrAlreadyExists
	}
	r.urls[id] = url
	return nil
}

// Get возвращает URL по ID или пустую строку с false, если ID не найден.
func (r *InMemoryURLRepo) Get(id string) (string, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	url, ok := r.urls[id]
	return url, ok
}
