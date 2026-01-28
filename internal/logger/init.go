package logger

import (
	"log"
	"os"
)

// InitFromEnvAndFlag инициализирует глобальный логгер,
// используя переменную окружения LOG_LEVEL или переданный флаг.
func InitFromEnvAndFlag(flagLevel string) {
	logLevel := flagLevel
	if envLevel := os.Getenv("LOG_LEVEL"); envLevel != "" {
		logLevel = envLevel
	}

	if err := Initialize(logLevel); err != nil {
		log.Fatalf("failed to initialize logger: %v", err)
	}
}
