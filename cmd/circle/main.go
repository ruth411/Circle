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
	"github.com/ruth411/circle/internal/core/recipe"
	"github.com/ruth411/circle/internal/diner"
	"github.com/ruth411/circle/internal/identity"
	"github.com/ruth411/circle/internal/inventory"
	"github.com/ruth411/circle/internal/ordering"
	"github.com/ruth411/circle/internal/platform/config"
	"github.com/ruth411/circle/internal/platform/db"
	"github.com/ruth411/circle/internal/platform/events"
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
	recipeRepository := recipe.NewSQLRepository(database)
	outbox := events.NewSQLOutbox(database)
	orderingService := ordering.NewServiceWithDependencies(ordering.NewSQLRepository(database), recipeRepository, ordering.MockProvider{})
	inventoryService := inventory.NewService(inventory.NewSQLRepository(database))
	inventoryProcessor := inventory.NewProcessor(outbox, inventoryService)
	dinerService := diner.NewServiceWithRepository(diner.NewSQLRepository(database))
	dinerProcessor := diner.NewProcessor(outbox, dinerService)
	sessionValidator := identity.NewSQLSessionValidator(database)

	go runInventoryProcessor(ctx, logger, inventoryProcessor)
	go runDinerProcessor(ctx, logger, dinerProcessor)

	server := &http.Server{
		Addr: addr,
		Handler: httpapi.NewServerWithDependencies(logger, httpapi.Dependencies{
			IngredientService:    ingredientService,
			DinerService:         dinerService,
			OrderingService:      orderingService,
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

func runInventoryProcessor(ctx context.Context, logger *slog.Logger, processor *inventory.Processor) {
	if processor == nil {
		return
	}

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			processed, err := processor.ProcessPendingClosedOrders(ctx, 100)
			if err != nil && !errors.Is(err, context.Canceled) {
				logger.Warn("inventory processor failed", "err", err)
				continue
			}
			if processed > 0 {
				logger.Info("inventory processor applied events", "count", processed)
			}
		}
	}
}

func runDinerProcessor(ctx context.Context, logger *slog.Logger, processor *diner.Processor) {
	if processor == nil {
		return
	}

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			processed, err := processor.ProcessPendingClosedOrders(ctx, 100)
			if err != nil && !errors.Is(err, context.Canceled) {
				logger.Warn("diner processor failed", "err", err)
				continue
			}
			if processed > 0 {
				logger.Info("diner processor issued tokens", "count", processed)
			}
		}
	}
}
