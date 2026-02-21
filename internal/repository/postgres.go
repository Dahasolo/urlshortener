package repository

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/Dahasolo/urlshortener/internal/service"
	"github.com/Dahasolo/urlshortener/migrations"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// PostgresURLRepo - реализация репозитория для работы с PostgreSQL.
type PostgresURLRepo struct {
	db     *sql.DB
	logger *slog.Logger
}

// NewPostgresURLRepo создаёт новый экземпляр репозитория с подключением к PostgreSQL.
func NewPostgresURLRepo(dsn string, logger *slog.Logger) (*PostgresURLRepo, error) {
	logger.Info("connecting to PostgreSQL database", "dsn", dsn)

	// Создание пула соединений с БД
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	// Проверка подключения к БД
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	if err = db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Применение миграций
	if err := migrations.ApplyMigrations(dsn, logger); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to apply migrations: %w", err)
	}

	logger.Info("successfully connected to PostgreSQL")

	return &PostgresURLRepo{
		db:     db,
		logger: logger,
	}, nil
}

// Ping проверяет соединение с базой данных.
func (r *PostgresURLRepo) Ping() error {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	return r.db.PingContext(ctx)
}

// Save сохраняет короткий URL в БД.
func (r *PostgresURLRepo) Save(id, url string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	query := `
		INSERT INTO urls (id, original_url) VALUES ($1, $2) 
		ON CONFLICT (original_url) DO UPDATE SET id = urls.id 
		RETURNING id, (xmax = 0) AS inserted
	`

	var existingID string
	var inserted bool

	err := r.db.QueryRowContext(ctx, query, id, url).Scan(&existingID, &inserted)
	if err != nil {
		return fmt.Errorf("failed to save URL: %w", err)
	}

	if !inserted {
		return &service.ErrURLAlreadyExists{
			ExistingID:  existingID,
			OriginalURL: url,
		}
	}

	return nil
}

// SaveMany сохраняет несколько коротких URL в БД.
func (r *PostgresURLRepo) SaveMany(entries []service.BatchEntry) error {
	if len(entries) == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	args := make([]any, 0, len(entries)*2)
	var query strings.Builder
	query.WriteString("INSERT INTO urls (id, original_url) VALUES ")
	for i, entry := range entries {
		if i > 0 {
			query.WriteString(", ")
		}
		fmt.Fprintf(&query, "($%d, $%d)", i*2+1, i*2+2)
		args = append(args, entry.ID, entry.OriginalURL)
	}
	query.WriteString(` ON CONFLICT (original_url) DO UPDATE SET id = urls.id`)
	_, err := r.db.ExecContext(ctx, query.String(), args...)
	if err != nil {
		return fmt.Errorf("failed to save URL: %w", err)
	}

	return nil
}

// Get возвращает оригинальный URL по короткому идентификатору.
func (r *PostgresURLRepo) Get(id string) (string, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var originalURL string
	err := r.db.QueryRowContext(ctx, "SELECT original_url FROM urls WHERE id = $1", id).Scan(&originalURL)

	if err != nil {
		return "", false
	}

	return originalURL, true
}

// GetExistingID возвращает существующий ID по оригинальному URL.
func (r *PostgresURLRepo) GetExistingID(url string) (string, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var id string
	err := r.db.QueryRowContext(ctx, "SELECT id FROM urls WHERE original_url = $1", url).Scan(&id)
	if err != nil {
		return "", false
	}

	return id, true
}

// Close закрывает соединение с БД.
func (r *PostgresURLRepo) Close() error {
	if r.db != nil {
		return r.db.Close()
	}
	return nil
}
