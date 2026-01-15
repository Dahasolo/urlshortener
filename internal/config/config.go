package config

import (
	"errors"
	"flag"
	"fmt"
	"net/url"
)

// Config содержит параметры запуска сервиса.
type Config struct {
	ServerAddress string // адрес запуска HTTP-сервера (-a)
	BaseURL       string // базовый URL для коротких ссылок (-b)
}

// Load парсит флаги командной строки и возвращает Config.
func Load() (*Config, error) {
	var cfg Config

	flag.StringVar(&cfg.ServerAddress, "a", ":8080", "адрес запуска HTTP-сервера")
	flag.StringVar(&cfg.BaseURL, "b", "http://localhost:8080/", "базовый адрес для коротких URL")

	flag.Parse()

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &cfg, nil
}

// Validate проверяет корректность полей конфигурации.
func (c *Config) Validate() error {
	if c.ServerAddress == "" {
		return errors.New("server address is required")
	}

	if c.BaseURL == "" {
		return errors.New("base URL is required")
	}

	if _, err := url.Parse(c.BaseURL); err != nil {
		return fmt.Errorf("invalid base URL: %w", err)
	}

	return nil
}
