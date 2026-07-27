package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"log/slog"

	"github.com/ruth411/circle/internal/core/ingredient"
	"github.com/ruth411/circle/internal/identity"
	"github.com/ruth411/circle/internal/platform/config"
	"github.com/ruth411/circle/internal/platform/db"
	"github.com/ruth411/circle/internal/platform/httpapi"
	"github.com/ruth411/circle/internal/tenancy"
	projectmigrations "github.com/ruth411/circle/migrations"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	cfg := config.Load()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: cfg.LogLevel,
	}))
	if cfg.DBDSN == "" {
		logger.Error("database configuration missing", "env", "DATABASE_URL")
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	database, err := db.Open(cfg.DBDriver, cfg.DBDSN)
	if err != nil {
		logger.Error("database open failed", "err", err)
		os.Exit(1)
	}
	defer database.Close()

	readyCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	if err := database.PingContext(readyCtx); err != nil {
		logger.Error("database ping failed", "err", err)
		os.Exit(1)
	}

	migrations, err := db.LoadMigrationsFS(projectmigrations.FS, ".")
	if err != nil {
		logger.Error("migration load failed", "err", err)
		os.Exit(1)
	}
	if err := db.ApplyMigrations(readyCtx, database.ExecContext, migrations); err != nil {
		logger.Error("migration apply failed", "err", err)
		os.Exit(1)
	}

	addr := ":" + cfg.Port
	ingredientService := ingredient.NewService(ingredient.NewSQLRepository(database))
	sessionValidator := identity.NewSQLSessionValidator(database)

	server := &http.Server{
		Addr: addr,
		Handler: httpapi.NewServerWithDependencies(logger, httpapi.Dependencies{
			IngredientService:    ingredientService,
			SessionValidator:     sessionValidator,
			OrganizationResolver: tenancy.NewSQLOrganizationResolver(database),
		}),
		ReadHeaderTimeout: 5 * time.Second,
	}

	logger.Info("circle listening", "addr", addr, "env", cfg.AppEnv)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("server stopped", "err", err)
		os.Exit(1)
	}
}
