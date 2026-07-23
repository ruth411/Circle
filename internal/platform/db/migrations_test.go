package db

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"testing/fstest"
)

func TestLoadMigrationsFSSortsByName(t *testing.T) {
	fsys := fstest.MapFS{
		"migrations/0002_b.sql": &fstest.MapFile{Data: []byte("SELECT 2;")},
		"migrations/0001_a.sql": &fstest.MapFile{Data: []byte("SELECT 1;")},
		"migrations/ignore.txt": &fstest.MapFile{Data: []byte("nope")},
	}

	migrations, err := LoadMigrationsFS(fsys, "migrations")
	if err != nil {
		t.Fatalf("LoadMigrationsFS returned error: %v", err)
	}
	if len(migrations) != 2 {
		t.Fatalf("count = %d, want 2", len(migrations))
	}
	if migrations[0].Name != "0001_a.sql" || migrations[1].Name != "0002_b.sql" {
		t.Fatalf("unexpected order: %#v", migrations)
	}
}

func TestApplyMigrationsExecutesNonEmptyStatements(t *testing.T) {
	var executed []string
	exec := func(_ context.Context, query string, _ ...any) (sql.Result, error) {
		executed = append(executed, strings.TrimSpace(query))
		return nil, nil
	}

	err := ApplyMigrations(context.Background(), exec, []Migration{
		{Name: "0001.sql", SQL: "CREATE SCHEMA example;"},
		{Name: "0002.sql", SQL: "   "},
		{Name: "0003.sql", SQL: "CREATE TABLE example.test (id INT);"},
	})
	if err != nil {
		t.Fatalf("ApplyMigrations returned error: %v", err)
	}

	if len(executed) != 2 {
		t.Fatalf("executed count = %d, want 2", len(executed))
	}
	if executed[0] != "CREATE SCHEMA example;" {
		t.Fatalf("first query = %q", executed[0])
	}
	if executed[1] != "CREATE TABLE example.test (id INT);" {
		t.Fatalf("second query = %q", executed[1])
	}
}
