package config

import (
	"flag"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strings"
)

// Config содержит параметры запуска сервиса.
type Config struct {
	ServerAddress   string // адрес запуска HTTP-сервера (-a)
	BaseURL         string // базовый URL для коротких ссылок (-b)
	FileStoragePath string // путь к файлу для хранения данных в формате JSON (-f)
	LogLevel        string // уровень логирования (-l)
	DatabaseDSN     string // строка подключения к БД (-d)
}

// configField описывает источник параметра.
type сonfigField struct {
	envVar       string // переменная окружения
	flagName     string // флаг командной строки
	defaultValue string // значение по умолчанию
	usage        string // описание
	optional     bool   // true - разрешено пустое значение
}

// configFields определяет параметры конфигурации.
var configFields = []сonfigField{
	{"SERVER_ADDRESS", "a", ":8080", "адрес запуска HTTP-сервера", false},
	{"BASE_URL", "b", "http://localhost:8080/", "базовый адрес для коротких URL", false},
	{"FILE_STORAGE_PATH", "f", "./storage.json", "путь к файлу для хранения данных", true},
	{"LOG_LEVEL", "l", "info", "уровень логирования", false},
	{"DATABASE_DSN", "d", "", "строка подключения к базе данных", true},
}

// Load загружает конфигурацию с учётом приоритета:
// 1. Переменные окружения
// 2. Флаги командной строки
// 3. Значения по умолчанию
func Load() (*Config, error) {
	flagVars := make(map[string]*string)
	for _, field := range configFields {
		flagVars[field.flagName] = flag.String(field.flagName, field.defaultValue, field.usage)
	}

	flag.Parse()

	cfg := &Config{}
	for _, field := range configFields {
		value, err := getEnvOrFlag(field.envVar, *flagVars[field.flagName], field.optional)
		if err != nil {
			return nil, err
		}
		switch field.envVar {
		case "SERVER_ADDRESS":
			cfg.ServerAddress = value
		case "BASE_URL":
			cfg.BaseURL = value
		case "FILE_STORAGE_PATH":
			cfg.FileStoragePath = value
		case "LOG_LEVEL":
			cfg.LogLevel = value
		case "DATABASE_DSN":
			cfg.DatabaseDSN = value
		}
	}

	return cfg, cfg.Validate()
}

// getEnvOrFlag возвращает значение переменной окружения, если она установлена.
// Иначе - значение флага командной строки.
func getEnvOrFlag(envVar, flagValue string, optional bool) (string, error) {
	if value, ok := os.LookupEnv(envVar); ok {
		if value == "" && !optional {
			return "", fmt.Errorf("%s cannot be empty", envVar)
		}
		return value, nil
	}

	if flagValue == "" && !optional {
		return "", fmt.Errorf("%s flag cannot be empty", envVar)
	}

	return flagValue, nil
}

// Validate проверяет корректность полей конфигурации.
func (c *Config) Validate() error {
	// Валидация BaseURL
	if _, err := url.Parse(c.BaseURL); err != nil {
		return fmt.Errorf("invalid base URL %q: %w", c.BaseURL, err)
	}

	// Валидация уровня логирования
	var lvl slog.Level
	if err := lvl.UnmarshalText([]byte(strings.ToUpper(c.LogLevel))); err != nil {
		return fmt.Errorf("invalid log level %q: %w", c.LogLevel, err)
	}

	return nil
}
