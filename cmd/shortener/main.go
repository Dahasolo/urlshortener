package main

import (
	"log"

	"github.com/Dahasolo/urlshortener/internal/app"
	"github.com/Dahasolo/urlshortener/internal/config"
	"github.com/Dahasolo/urlshortener/internal/repository"
	"github.com/Dahasolo/urlshortener/internal/router"
	"github.com/Dahasolo/urlshortener/internal/service"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal("config load error:", err)
	}

	repo := repository.NewInMemoryURLRepo()
	svc := service.NewService(repo)

	r := router.NewRouter(svc, cfg.BaseURL)

	if err := app.Run(cfg.ServerAddress, r); err != nil {
		log.Fatal("server error:", err)
	}
}
