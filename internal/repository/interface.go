package repository

// Интерфейс для работы с хранилищем URL.
type URLRepository interface {
	Save(id, url string)
	Get(id string) (string, bool)
}
