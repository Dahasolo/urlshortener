package main

import (
	"flag"
	"log"
	"os"

	"github.com/Dahasolo/urlshortener/internal/app"
	"github.com/Dahasolo/urlshortener/internal/config"
	"github.com/Dahasolo/urlshortener/internal/handler/middleware"
	"github.com/Dahasolo/urlshortener/internal/logger"
	"github.com/Dahasolo/urlshortener/internal/repository"
	"github.com/Dahasolo/urlshortener/internal/router"
	"github.com/Dahasolo/urlshortener/internal/service"
)

var flagLogLevel string

func init() {
	flag.StringVar(&flagLogLevel, "l", "info", "уровень логирования")
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal("config load error:", err)
	}

	logger.InitFromEnvAndFlag(flagLogLevel)

	repo := repository.NewInMemoryURLRepo(cfg.FileStoragePath)
	if cfg.FileStoragePath != "" {
		if err := repo.LoadFromFile(); err != nil {
			logger.Log.Error("failed to load data from file", "error", err, "path", cfg.FileStoragePath)
		} else {
			logger.Log.Info("data loaded from file", "path", cfg.FileStoragePath)
		}
	}

	svc := service.NewService(repo)
	r := router.NewRouter(svc, cfg.BaseURL)

	routerWithLogging := logger.HTTPLogger(middleware.GzipMiddleware(r))

	logger.Log.Info("running server on", "address", cfg.ServerAddress)
	if err := app.Run(cfg.ServerAddress, routerWithLogging); err != nil {
		logger.Log.Error("server error", "error", err)
		os.Exit(1)
	}
}
