package repository

import (
	"context"
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
	err = repo1.Save(context.Background(), "4rSPg8ap", "https://yandex.ru", "test-user-id")
	require.NoError(t, err)
	err = repo1.Save(context.Background(), "dG56Hqxm", "https://practicum.yandex.ru", "test-user-id")
	require.NoError(t, err)

	// принудительное закрытие первого репозитория
	require.NoError(t, repo1.Close())

	// создание второго репозитория - данные загружаются автоматически
	repo2, err := NewInMemoryURLRepo(filePath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = repo2.Close() })

	// проверка восстановления данных
	res1, ok1 := repo2.Get(context.Background(), "4rSPg8ap")
	assert.True(t, ok1)
	assert.Equal(t, "https://yandex.ru", res1.OriginalURL)
	assert.False(t, res1.IsDeleted)

	res2, ok2 := repo2.Get(context.Background(), "dG56Hqxm")
	assert.True(t, ok2)
	assert.Equal(t, "https://practicum.yandex.ru", res2.OriginalURL)
	assert.False(t, res2.IsDeleted)
}

func TestInMemoryURLRepo_MarkAsDeleted(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		setup         func(*InMemoryURLRepo)
		ids           []string
		userID        string
		expectDeleted map[string]bool
	}{
		{
			name: "delete existing url",
			setup: func(repo *InMemoryURLRepo) {
				_ = repo.Save(ctx, "abc123", "https://example.com", "user-1")
			},
			ids:    []string{"abc123"},
			userID: "user-1",
			expectDeleted: map[string]bool{
				"abc123": true,
			},
		},
		{
			name: "delete wrong user",
			setup: func(repo *InMemoryURLRepo) {
				_ = repo.Save(ctx, "abc123", "https://example.com", "user-1")
			},
			ids:    []string{"abc123"},
			userID: "user-2",
			expectDeleted: map[string]bool{
				"abc123": false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, err := NewInMemoryURLRepo("")
			require.NoError(t, err)

			tt.setup(repo)

			err = repo.MarkAsDeleted(ctx, tt.ids, tt.userID)
			require.NoError(t, err)

			for shortURL, expectDeleted := range tt.expectDeleted {
				res, ok := repo.Get(ctx, shortURL)
				require.True(t, ok, "URL должен существовать: %s", shortURL)
				assert.Equal(t, expectDeleted, res.IsDeleted,
					"URL %s: expected is_deleted=%v", shortURL, expectDeleted)
			}
		})
	}
}
