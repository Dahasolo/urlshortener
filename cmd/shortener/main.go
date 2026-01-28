package main

import (
	"flag"
	"log"

	"github.com/Dahasolo/urlshortener/internal/app"
	"github.com/Dahasolo/urlshortener/internal/config"
	"github.com/Dahasolo/urlshortener/internal/logger"
	"github.com/Dahasolo/urlshortener/internal/repository"
	"github.com/Dahasolo/urlshortener/internal/router"
	"github.com/Dahasolo/urlshortener/internal/service"
	"go.uber.org/zap"
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

	repo := repository.NewInMemoryURLRepo()
	svc := service.NewService(repo)
	r := router.NewRouter(svc, cfg.BaseURL)

	routerWithLogging := logger.HTTPLogger(r)
	logger.Log.Info("running server on", zap.String("address", cfg.ServerAddress))

	if err := app.Run(cfg.ServerAddress, routerWithLogging); err != nil {
		logger.Log.Fatal("server error:", zap.Error(err))
	}
}
