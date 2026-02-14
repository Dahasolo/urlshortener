package repository

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

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
	return nil
}

// Get возвращает оригинальный URL по короткому идентификатору.
func (r *PostgresURLRepo) Get(id string) (string, bool) {
	return "", false
}

// Close закрывает соединение с БД.
func (r *PostgresURLRepo) Close() error {
	if r.db != nil {
		return r.db.Close()
	}
	return nil
}
