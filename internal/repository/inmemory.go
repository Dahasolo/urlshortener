package repository

import "sync"

type InMemoryURLRepo struct {
	urls map[string]string
	mu   sync.RWMutex
}

func NewInMemoryURLRepo() *InMemoryURLRepo {
	return &InMemoryURLRepo{
		urls: make(map[string]string),
	}
}

func (r *InMemoryURLRepo) Save(id, url string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.urls[id] = url
}

func (r *InMemoryURLRepo) Get(id string) (string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	url, ok := r.urls[id]
	return url, ok
}
