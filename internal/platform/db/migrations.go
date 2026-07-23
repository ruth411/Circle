package db

import (
	"context"
	"database/sql"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

type Migration struct {
	Name string
	SQL  string
}

type ExecContextFunc func(context.Context, string, ...any) (sql.Result, error)

func LoadMigrations(dir string) ([]Migration, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var names []string
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".sql" {
			continue
		}
		names = append(names, entry.Name())
	}
	slices.Sort(names)

	migrations := make([]Migration, 0, len(names))
	for _, name := range names {
		content, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		migrations = append(migrations, Migration{
			Name: name,
			SQL:  string(content),
		})
	}

	return migrations, nil
}

func LoadMigrationsFS(fsys fs.FS, dir string) ([]Migration, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, err
	}

	var names []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		names = append(names, entry.Name())
	}
	slices.Sort(names)

	migrations := make([]Migration, 0, len(names))
	for _, name := range names {
		content, err := fs.ReadFile(fsys, filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		migrations = append(migrations, Migration{
			Name: name,
			SQL:  string(content),
		})
	}

	return migrations, nil
}

func ApplyMigrations(ctx context.Context, exec ExecContextFunc, migrations []Migration) error {
	for _, migration := range migrations {
		if strings.TrimSpace(migration.SQL) == "" {
			continue
		}
		if _, err := exec(ctx, migration.SQL); err != nil {
			return err
		}
	}
	return nil
}
