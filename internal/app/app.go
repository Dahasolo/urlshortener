package app

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/Dahasolo/urlshortener/internal/config"
	"github.com/Dahasolo/urlshortener/internal/logger"
	"github.com/Dahasolo/urlshortener/internal/repository"
	"github.com/Dahasolo/urlshortener/internal/router"
	"github.com/Dahasolo/urlshortener/internal/service"
)

// App инкапсулирует все зависимости приложения.
type App struct {
	cfg     *config.Config
	logger  *slog.Logger
	repo    service.URLRepository
	svc     *service.Service
	router  http.Handler
	cleanup func()
}

// NewApp создает инициализированное приложение.
func NewApp(cfg *config.Config) (*App, error) {
	// Инициализация логгера
	logger, err := logger.NewLogger(cfg.LogLevel)
	if err != nil {
		return nil, fmt.Errorf("failed to create logger: %w", err)
	}

	logger.Info("starting app initialization")

	// Инициализация репозитория
	var repo service.URLRepository
	if cfg.DatabaseDSN != "" {
		repo, err = repository.NewPostgresURLRepo(cfg.DatabaseDSN, logger)
		if err != nil {
			return nil, fmt.Errorf("failed to create PostgreSQL repository: %w", err)
		}
		logger.Info("repository initialized with PostgreSQL")
	} else {
		repo, err = repository.NewInMemoryURLRepo(cfg.FileStoragePath)
		if err != nil {
			return nil, fmt.Errorf("failed to create repository: %w", err)
		}
		if cfg.FileStoragePath != "" {
			logger.Info("repository initialized with file storage", "path", cfg.FileStoragePath)
		}
	}

	// Инициализация сервиса
	svc := service.NewService(repo)

	// Инициализация роутера
	r := router.NewRouter(svc, cfg.BaseURL, logger)

	app := &App{
		cfg:    cfg,
		logger: logger,
		repo:   repo,
		svc:    svc,
		router: r,
		cleanup: func() {
			if err := repo.Close(); err != nil {
				logger.Error("failed to close repository file", "error", err)
			}
		},
	}

	return app, nil
}

// NewAppFromFlags создаёт приложение, загружая конфигурацию из флагов и переменных окружения.
func NewAppFromFlags() (*App, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("config load error: %w", err)
	}

	return NewApp(cfg)
}

// Run запускает HTTP сервер.
func (a *App) Run() error {
	a.logger.Info("running server on", "address", a.cfg.ServerAddress)
	defer a.cleanup()
	return http.ListenAndServe(a.cfg.ServerAddress, a.router)
}
