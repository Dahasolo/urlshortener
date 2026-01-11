package config

import (
	"flag"
	"strings"
)

// Config содержит параметры запуска сервиса.
type Config struct {
	ServerAddress string // адрес запуска HTTP-сервера (-a)
	BaseURL       string // базовый URL для коротких ссылок (-b)
}

// MustLoad парсит флаги командной строки и возвращает конфигурацию.
func MustLoad() *Config {
	var cfg Config

	flag.StringVar(&cfg.ServerAddress, "a", ":8080", "адрес запуска HTTP-сервера")
	flag.StringVar(&cfg.BaseURL, "b", "http://localhost:8080/", "базовый адрес для коротких URL")

	flag.Parse()

	// Гарантируем, что BaseURL заканчивается на "/"
	if !strings.HasSuffix(cfg.BaseURL, "/") {
		cfg.BaseURL += "/"
	}

	return &cfg
}
