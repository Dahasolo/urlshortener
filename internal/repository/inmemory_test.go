package repository

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInMemoryURLRepo_SaveAndLoadFromFile(t *testing.T) {
	filePath := t.TempDir() + "/storage.json"

	// сохранение данных
	repo1 := NewInMemoryURLRepo(filePath)
	err := repo1.Save("4rSPg8ap", "https://yandex.ru")
	require.NoError(t, err)
	err = repo1.Save("dG56Hqxm", "https://practicum.yandex.ru")
	require.NoError(t, err)

	// создание репозитория и загрузка данных
	repo2 := NewInMemoryURLRepo(filePath)
	err = repo2.LoadFromFile()
	require.NoError(t, err)

	// проверка восстановления данных
	url1, ok1 := repo2.Get("4rSPg8ap")
	assert.True(t, ok1)
	assert.Equal(t, "https://yandex.ru", url1)

	url2, ok2 := repo2.Get("dG56Hqxm")
	assert.True(t, ok2)
	assert.Equal(t, "https://practicum.yandex.ru", url2)
}
