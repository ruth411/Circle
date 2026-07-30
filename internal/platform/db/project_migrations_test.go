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
		"CREATE TABLE IF NOT EXISTS recipe.menu_items",
		"CREATE TABLE IF NOT EXISTS recipe.modifier_groups",
		"CREATE TABLE IF NOT EXISTS recipe.modifiers",
		"CREATE TABLE IF NOT EXISTS recipe.modifier_ingredient_deltas",
		"CREATE TABLE IF NOT EXISTS recipe.menu_snapshots",
		"CREATE TABLE IF NOT EXISTS recipe.menu_snapshot_items",
		"CREATE TABLE IF NOT EXISTS recipe.menu_snapshot_modifier_groups",
		"CREATE TABLE IF NOT EXISTS recipe.menu_snapshot_modifiers",
		"CREATE OR REPLACE FUNCTION recipe.prevent_snapshot_mutation()",
		"ADD CONSTRAINT recipe_menu_items_location_recipe_fk",
		"CREATE TABLE IF NOT EXISTS ordering.orders",
		"CREATE TABLE IF NOT EXISTS ordering.order_lines",
		"CREATE TABLE IF NOT EXISTS ordering.order_line_modifiers",
		"CREATE TABLE IF NOT EXISTS ordering.checks",
		"CREATE TABLE IF NOT EXISTS ordering.tenders",
		"CREATE TABLE IF NOT EXISTS inventory.inventory_movements",
		"CREATE TABLE IF NOT EXISTS inventory.inventory_counts",
		"CREATE TABLE IF NOT EXISTS inventory.inventory_count_lines",
		"CREATE TABLE IF NOT EXISTS platform.outbox_event_deliveries",
		"CREATE TABLE IF NOT EXISTS platform.outbox_event_failures",
		"CREATE TABLE IF NOT EXISTS diner.receipt_tokens",
		"CREATE TABLE IF NOT EXISTS diner.receipt_token_items",
		"CREATE TABLE IF NOT EXISTS diner.claims",
		"CREATE TABLE IF NOT EXISTS diner.claim_items",
		"ALTER COLUMN location_id DROP NOT NULL",
	}

	for _, table := range requiredTables {
		if !strings.Contains(combined, table) {
			t.Fatalf("combined migrations missing %q", table)
		}
	}
}

func TestProjectMigrationsBackfillLegacyClosedOrderDeliveries(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}

	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
	migrations, err := LoadMigrations(filepath.Join(root, "migrations"))
	if err != nil {
		t.Fatalf("LoadMigrations returned error: %v", err)
	}

	var combined string
	for _, migration := range migrations {
		combined += migration.SQL
	}

	requiredSnippets := []string{
		"INSERT INTO platform.outbox_event_deliveries",
		"SELECT id, 'inventory', published_at",
		"WHERE published_at IS NOT NULL",
		"name = 'ordering.closed_order'",
	}

	for _, snippet := range requiredSnippets {
		if !strings.Contains(combined, snippet) {
			t.Fatalf("combined migrations missing %q", snippet)
		}
	}
}

func TestProjectMigrationsRepairRecentDinerBackfill(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}

	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
	migrations, err := LoadMigrations(filepath.Join(root, "migrations"))
	if err != nil {
		t.Fatalf("LoadMigrations returned error: %v", err)
	}

	var combined string
	for _, migration := range migrations {
		combined += migration.SQL
	}

	requiredSnippets := []string{
		"DELETE FROM platform.outbox_event_deliveries",
		"consumer_name = 'diner'",
		"platform.outbox_events.published_at = platform.outbox_event_deliveries.delivered_at",
		"occurred_at >= NOW() - INTERVAL '1 day'",
	}

	for _, snippet := range requiredSnippets {
		if !strings.Contains(combined, snippet) {
			t.Fatalf("combined migrations missing %q", snippet)
		}
	}
}

func TestProjectMigrationsHardenDinerConstraints(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}

	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
	migrations, err := LoadMigrations(filepath.Join(root, "migrations"))
	if err != nil {
		t.Fatalf("LoadMigrations returned error: %v", err)
	}

	var combined string
	for _, migration := range migrations {
		combined += migration.SQL
	}

	requiredSnippets := []string{
		"diner_receipt_tokens_location_fk",
		"diner_receipt_token_items_location_token_fk",
		"diner_claims_location_token_fk",
		"diner_claim_items_location_claim_fk",
		"diner_claim_items_location_token_item_fk",
	}

	for _, snippet := range requiredSnippets {
		if !strings.Contains(combined, snippet) {
			t.Fatalf("combined migrations missing %q", snippet)
		}
	}
}
