package recipe

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestMapRecipeWriteErrorTranslatesUniqueViolation(t *testing.T) {
	err := mapRecipeWriteError(&pgconn.PgError{Code: "23505"})
	if !errors.Is(err, ErrRecipeAlreadyExists) {
		t.Fatalf("err = %v, want ErrRecipeAlreadyExists", err)
	}
}

func TestMapMenuItemWriteErrorTranslatesUniqueViolation(t *testing.T) {
	err := mapMenuItemWriteError(&pgconn.PgError{Code: "23505"})
	if !errors.Is(err, ErrMenuItemAlreadyExists) {
		t.Fatalf("err = %v, want ErrMenuItemAlreadyExists", err)
	}
}

type fakeRecipeExecQueryer struct {
	err error
}

func (f fakeRecipeExecQueryer) ExecContext(context.Context, string, ...any) (sql.Result, error) {
	return nil, f.err
}

func (f fakeRecipeExecQueryer) QueryContext(context.Context, string, ...any) (*sql.Rows, error) {
	panic("unexpected QueryContext call")
}

func (f fakeRecipeExecQueryer) QueryRowContext(context.Context, string, ...any) *sql.Row {
	panic("unexpected QueryRowContext call")
}

func TestUpsertRecipeUpdateMapsUniqueViolationToAlreadyExists(t *testing.T) {
	err := upsertRecipe(context.Background(), fakeRecipeExecQueryer{
		err: &pgconn.PgError{Code: "23505"},
	}, Recipe{ID: "rec-1", LocationID: "loc-1", Name: "Collision", YieldCount: 1}, true)
	if !errors.Is(err, ErrRecipeAlreadyExists) {
		t.Fatalf("err = %v, want ErrRecipeAlreadyExists", err)
	}
}
