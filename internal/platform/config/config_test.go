package config

import "testing"

func TestLoadDefaults(t *testing.T) {
	t.Setenv("APP_ENV", "")
	t.Setenv("PORT", "")
	t.Setenv("DB_DRIVER", "")
	t.Setenv("DB_DSN", "")
	t.Setenv("LOG_LEVEL", "")

	cfg := Load()

	if cfg.AppEnv != "development" {
		t.Fatalf("AppEnv = %q, want development", cfg.AppEnv)
	}
	if cfg.Port != "8080" {
		t.Fatalf("Port = %q, want 8080", cfg.Port)
	}
	if cfg.DBDriver != "postgres" {
		t.Fatalf("DBDriver = %q, want postgres", cfg.DBDriver)
	}
	if cfg.DBDSN != "" {
		t.Fatalf("DBDSN = %q, want empty", cfg.DBDSN)
	}
	if cfg.LogLevel != 0 {
		t.Fatalf("LogLevel = %v, want 0", cfg.LogLevel)
	}
}

func TestLoadOverrides(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	t.Setenv("PORT", "9999")
	t.Setenv("DB_DRIVER", "sqlite3")
	t.Setenv("DB_DSN", "file:test.db")
	t.Setenv("LOG_LEVEL", "debug")

	cfg := Load()

	if cfg.AppEnv != "test" {
		t.Fatalf("AppEnv = %q, want test", cfg.AppEnv)
	}
	if cfg.Port != "9999" {
		t.Fatalf("Port = %q, want 9999", cfg.Port)
	}
	if cfg.DBDriver != "sqlite3" {
		t.Fatalf("DBDriver = %q, want sqlite3", cfg.DBDriver)
	}
	if cfg.DBDSN != "file:test.db" {
		t.Fatalf("DBDSN = %q, want file:test.db", cfg.DBDSN)
	}
	if cfg.LogLevel.String() != "DEBUG" {
		t.Fatalf("LogLevel = %s, want DEBUG", cfg.LogLevel.String())
	}
}
