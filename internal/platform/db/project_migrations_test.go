package db

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestProjectMigrationsIncludePhase1Tables(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}

	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
	migrations, err := LoadMigrations(filepath.Join(root, "migrations"))
	if err != nil {
		t.Fatalf("LoadMigrations returned error: %v", err)
	}

	if len(migrations) < 2 {
		t.Fatalf("migration count = %d, want at least 2", len(migrations))
	}

	var combined string
	for _, migration := range migrations {
		combined += migration.SQL
	}

	requiredTables := []string{
		"CREATE TABLE IF NOT EXISTS tenancy.organizations",
		"CREATE TABLE IF NOT EXISTS tenancy.restaurants",
		"CREATE TABLE IF NOT EXISTS tenancy.locations",
		"CREATE TABLE IF NOT EXISTS identity.users",
		"CREATE TABLE IF NOT EXISTS identity.roles",
		"CREATE TABLE IF NOT EXISTS identity.user_roles",
		"CREATE TABLE IF NOT EXISTS identity.sessions",
		"CREATE TABLE IF NOT EXISTS ingredient.ingredients",
		"CREATE TABLE IF NOT EXISTS ingredient.ingredient_units",
		"CREATE TABLE IF NOT EXISTS ingredient.ingredient_yield_factors",
		"CREATE TABLE IF NOT EXISTS recipe.recipes",
		"CREATE TABLE IF NOT EXISTS recipe.recipe_lines",
		"ALTER COLUMN location_id DROP NOT NULL",
	}

	for _, table := range requiredTables {
		if !strings.Contains(combined, table) {
			t.Fatalf("combined migrations missing %q", table)
		}
	}
}
