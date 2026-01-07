package service

import (
	"crypto/rand"

	"github.com/Dahasolo/urlshortener/internal/repository"
)

// Base52
const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

// генерация 8-символьного случайного ID из letters.
func generateID() string {
	b := make([]byte, 8)

	_, err := rand.Read(b)
	if err != nil {
		// заглушка
		return "FallBackID"
	}

	for i := range b {
		// b[i] % 52 - индекс в letters
		b[i] = letters[int(b[i])%len(letters)]
	}

	return string(b)
}

func Shorten(url string) string {
	id := generateID() // ← теперь "EwHXdJfB", безопасно и надёжно
	repository.Save(id, url)
	return id
}

func Resolve(id string) (string, bool) {
	return repository.Get(id)
}
