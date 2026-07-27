package config

import (
	"log/slog"
	"os"
	"strings"
)

type Config struct {
	AppEnv   string
	Port     string
	DBDriver string
	DBDSN    string
	LogLevel slog.Level
}

func Load() Config {
	return Config{
		AppEnv:   getenv("APP_ENV", "development"),
		Port:     getenv("PORT", "8080"),
		DBDriver: getenv("DB_DRIVER", "pgx"),
		DBDSN:    firstNonEmpty(os.Getenv("DATABASE_URL"), os.Getenv("DB_DSN")),
		LogLevel: parseLogLevel(getenv("LOG_LEVEL", "INFO")),
	}
}

func getenv(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func parseLogLevel(raw string) slog.Level {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case "DEBUG":
		return slog.LevelDebug
	case "WARN":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
