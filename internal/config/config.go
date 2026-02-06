package config

import (
	"errors"
	"flag"
	"fmt"
	"net/url"
	"os"
)

// Config содержит параметры запуска сервиса.
type Config struct {
	ServerAddress   string // адрес запуска HTTP-сервера (-a)
	BaseURL         string // базовый URL для коротких ссылок (-b)
	FileStoragePath string // путь к файлу для хранения данных в формате JSON (-f)
}

// Load загружает конфигурацию с учётом приоритета:
// 1. Переменные окружения (SERVER_ADDRESS, BASE_URL, FILE_STORAGE_PATH)
// 2. Флаги командной строки (-a, -b, -f)
// 3. Значения по умолчанию
func Load() (*Config, error) {
	// Значения по умолчанию
	serverAddrDefault := ":8080"
	baseURLDefault := "http://localhost:8080/"
	fileStoragePathDefault := "./storage.json"

	// Флаги командной строки
	var serverAddrFlag, baseURLFlag, fileStoragePathFlag string
	flag.StringVar(&serverAddrFlag, "a", serverAddrDefault, "адрес запуска HTTP-сервера")
	flag.StringVar(&baseURLFlag, "b", baseURLDefault, "базовый адрес для коротких URL")
	flag.StringVar(&fileStoragePathFlag, "f", fileStoragePathDefault, "путь к файлу для хранения данных")

	flag.Parse()

	// Переменные окружения
	serverAddr := getEnvOrDefault("SERVER_ADDRESS", serverAddrFlag)
	baseURL := getEnvOrDefault("BASE_URL", baseURLFlag)
	fileStoragePath := getEnvOrDefault("FILE_STORAGE_PATH", fileStoragePathFlag)

	cfg := &Config{
		ServerAddress:   serverAddr,
		BaseURL:         baseURL,
		FileStoragePath: fileStoragePath,
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return cfg, nil
}

// getEnvOrDefault возвращает значение переменной окружения, если пустое - fallback.
func getEnvOrDefault(envVar, fallback string) string {
	if value := os.Getenv(envVar); value != "" {
		return value
	}
	return fallback
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
		return fmt.Errorf("invalid base URL %q: %w", c.BaseURL, err)
	}

	return nil
}
