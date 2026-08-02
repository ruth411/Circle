package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ruth411/circle/internal/platform/config"
	"github.com/ruth411/circle/internal/platform/db"
	"github.com/ruth411/circle/internal/provisioning"
	projectmigrations "github.com/ruth411/circle/migrations"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "provision-location":
		if err := runProvisionLocation(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	default:
		usage()
		os.Exit(2)
	}
}

func runProvisionLocation(args []string) error {
	fs := flag.NewFlagSet("provision-location", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var input provisioning.ProvisionLocationInput
	var sessionTTL string

	fs.StringVar(&input.OrganizationID, "organization-id", "", "organization id")
	fs.StringVar(&input.OrganizationName, "organization-name", "", "organization name")
	fs.StringVar(&input.RestaurantID, "restaurant-id", "", "restaurant id")
	fs.StringVar(&input.RestaurantName, "restaurant-name", "", "restaurant name")
	fs.StringVar(&input.LocationID, "location-id", "", "location id")
	fs.StringVar(&input.LocationName, "location-name", "", "location name")
	fs.StringVar(&input.TimezoneName, "timezone", "", "location timezone name")
	fs.StringVar(&input.Currency, "currency", "", "location currency")
	fs.StringVar(&input.RoleID, "role-id", "", "role id")
	fs.StringVar(&input.RoleName, "role-name", "store_manager", "role name")
	fs.StringVar(&input.UserID, "user-id", "", "user id")
	fs.StringVar(&input.Email, "email", "", "user email")
	fs.StringVar(&input.DisplayName, "display-name", "", "user display name")
	fs.StringVar(&input.SessionID, "session-id", "", "session id")
	fs.StringVar(&sessionTTL, "session-ttl", "720h", "session ttl, for example 720h")

	if err := fs.Parse(args); err != nil {
		return err
	}

	ttl, err := time.ParseDuration(sessionTTL)
	if err != nil {
		return fmt.Errorf("parse session ttl: %w", err)
	}
	input.SessionTTL = ttl
	passwordHash, err := resolvePasswordHash(os.Getenv("CIRCLE_ADMIN_PASSWORD_HASH"))
	if err != nil {
		return err
	}
	input.PasswordHash = passwordHash

	cfg := config.Load()
	if cfg.DBDSN == "" {
		return fmt.Errorf("database configuration missing: set DATABASE_URL")
	}

	database, err := db.Open(cfg.DBDriver, cfg.DBDSN)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer database.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := database.PingContext(ctx); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}

	migrations, err := db.LoadMigrationsFS(projectmigrations.FS, ".")
	if err != nil {
		return fmt.Errorf("load migrations: %w", err)
	}
	if err := db.ApplyMigrations(ctx, database.ExecContext, migrations); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}

	service := provisioning.NewService(provisioning.NewSQLRepository(database), nil)
	provisioned, err := service.ProvisionLocation(ctx, input)
	if err != nil {
		return err
	}

	return json.NewEncoder(os.Stdout).Encode(map[string]any{
		"organization_id": provisioned.OrganizationID,
		"restaurant_id":   provisioned.RestaurantID,
		"location_id":     provisioned.LocationID,
		"role_id":         provisioned.RoleID,
		"user_id":         provisioned.UserID,
		"session": map[string]any{
			"id":              provisioned.Session.ID,
			"user_id":         provisioned.Session.UserID,
			"organization_id": provisioned.Session.OrganizationID,
			"scope_type":      provisioned.Session.ScopeType,
			"location_id":     provisioned.Session.LocationID,
			"expires_at":      provisioned.Session.ExpiresAt.UTC().Format(time.RFC3339),
		},
	})
}

func resolvePasswordHash(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("password hash is required: set CIRCLE_ADMIN_PASSWORD_HASH")
	}
	return value, nil
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: CIRCLE_ADMIN_PASSWORD_HASH=... circle-admin provision-location [flags]")
}
