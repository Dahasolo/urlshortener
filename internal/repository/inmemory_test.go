package repository

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInMemoryURLRepo_SaveAndLoadFromFile(t *testing.T) {
	filePath := t.TempDir() + "/storage.json"

	// создание первого репозитория
	repo1, err := NewInMemoryURLRepo(filePath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = repo1.Close() })

	// сохранение данных
	err = repo1.Save("4rSPg8ap", "https://yandex.ru")
	require.NoError(t, err)
	err = repo1.Save("dG56Hqxm", "https://practicum.yandex.ru")
	require.NoError(t, err)

	// принудительное закрытие первого репозитория
	require.NoError(t, repo1.Close())

	// создание второго репозитория - данные загружаются автоматически
	repo2, err := NewInMemoryURLRepo(filePath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = repo2.Close() })

	// проверка восстановления данных
	url1, ok1 := repo2.Get("4rSPg8ap")
	assert.True(t, ok1)
	assert.Equal(t, "https://yandex.ru", url1)

	url2, ok2 := repo2.Get("dG56Hqxm")
	assert.True(t, ok2)
	assert.Equal(t, "https://practicum.yandex.ru", url2)
}
