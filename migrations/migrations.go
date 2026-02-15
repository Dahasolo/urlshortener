package migrations

import (
	"database/sql"
	"embed"
	"log/slog"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed *.sql
var migrationsFS embed.FS

// ApplyMigrations применяет миграции к базе данных через отдельное соединение.
func ApplyMigrations(dsn string, logger *slog.Logger) error {
	// соединение только для миграций
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		return err
	}

	src, err := iofs.New(migrationsFS, ".")
	if err != nil {
		return err
	}

	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return err
	}

	m, err := migrate.NewWithInstance("iofs", src, "postgres", driver)
	if err != nil {
		return err
	}
	defer m.Close()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return err
	}

	logger.Info("database migrations applied successfully")
	return nil
}
