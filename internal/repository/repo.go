package repository

import "sync"

var urls = make(map[string]string)
var mu sync.RWMutex

func Save(id, url string) {
	mu.Lock()
	defer mu.Unlock()
	urls[id] = url
}

func Get(id string) (string, bool) {
	mu.RLock()
	defer mu.RUnlock()
	url, ok := urls[id]
	return url, ok
}
