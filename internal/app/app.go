package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/Dahasolo/urlshortener/internal/config"
	"github.com/Dahasolo/urlshortener/internal/logger"
	"github.com/Dahasolo/urlshortener/internal/repository"
	"github.com/Dahasolo/urlshortener/internal/router"
	"github.com/Dahasolo/urlshortener/internal/service"
	"golang.org/x/sync/errgroup"
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
	svc := service.NewService(repo, logger)

	// Инициализация роутера
	r := router.NewRouter(svc, cfg.BaseURL, cfg.SecretKey, logger)

	app := &App{
		cfg:    cfg,
		logger: logger,
		repo:   repo,
		svc:    svc,
		router: r,
		cleanup: func() {
			if svc != nil {
				if err := svc.Close(); err != nil {
					logger.Error("failed to close service", "error", err)
				}
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

// Run запускает HTTP сервер с graceful shutdown.
func (a *App) Run() error {
	a.logger.Info("running server on", "address", a.cfg.ServerAddress)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	g, ctx := errgroup.WithContext(ctx)
	server := &http.Server{Addr: a.cfg.ServerAddress, Handler: a.router}

	g.Go(func() error {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("server failed: %w", err)
		}
		return nil
	})

	g.Go(func() error {
		<-ctx.Done()
		a.logger.Info("shutting down HTTP server")

		shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("server shutdown failed: %w", err)
		}
		return nil
	})

	g.Go(func() error {
		defer a.logger.Info("server has been shutdown")
		<-ctx.Done()
		a.cleanup()
		return nil
	})

	if err := g.Wait(); err != nil {
		if errors.Is(err, context.Canceled) {
			return nil
		}
		return err
	}

	a.logger.Info("application stopped gracefully")
	return nil
}
