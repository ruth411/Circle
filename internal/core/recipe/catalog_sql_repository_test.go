package recipe

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/ruth411/circle/internal/core/ingredient"
)

type fakeExecQueryer struct {
	callCount int
	failAt    int
	err       error
}

func (f *fakeExecQueryer) ExecContext(context.Context, string, ...any) (sql.Result, error) {
	f.callCount++
	if f.callCount == f.failAt {
		return nil, f.err
	}
	return fakeSQLResult(1), nil
}

func (f *fakeExecQueryer) QueryContext(context.Context, string, ...any) (*sql.Rows, error) {
	panic("unexpected QueryContext call")
}

func (f *fakeExecQueryer) QueryRowContext(context.Context, string, ...any) *sql.Row {
	panic("unexpected QueryRowContext call")
}

type fakeSQLResult int64

func (r fakeSQLResult) LastInsertId() (int64, error) {
	return 0, errors.New("unsupported")
}

func (r fakeSQLResult) RowsAffected() (int64, error) {
	return int64(r), nil
}

func TestReplaceModifierGroupsMapsGroupUniqueViolationToAlreadyExists(t *testing.T) {
	db := &fakeExecQueryer{
		failAt: 2,
		err:    &pgconn.PgError{Code: "23505"},
	}

	err := replaceModifierGroups(context.Background(), db, sampleMenuItemForSQL())
	if !errors.Is(err, ErrMenuItemAlreadyExists) {
		t.Fatalf("err = %v, want ErrMenuItemAlreadyExists", err)
	}
}

func TestReplaceModifierGroupsMapsModifierUniqueViolationToAlreadyExists(t *testing.T) {
	db := &fakeExecQueryer{
		failAt: 3,
		err:    &pgconn.PgError{Code: "23505"},
	}

	err := replaceModifierGroups(context.Background(), db, sampleMenuItemForSQL())
	if !errors.Is(err, ErrMenuItemAlreadyExists) {
		t.Fatalf("err = %v, want ErrMenuItemAlreadyExists", err)
	}
}

func sampleMenuItemForSQL() MenuItem {
	return MenuItem{
		ID:         "item-1",
		LocationID: "loc-1",
		RecipeID:   "rec-1",
		Name:       "Chicken Bowl",
		PriceMinor: 995,
		Currency:   "USD",
		ModifierGroups: []ModifierGroup{{
			ID:                 "grp-1",
			Name:               "Protein",
			SelectionMin:       1,
			SelectionMax:       1,
			Required:           true,
			Exclusive:          true,
			DefaultModifierIDs: []string{"mod-1"},
			Modifiers: []Modifier{{
				ID:              "mod-1",
				Name:            "Chicken",
				PriceDeltaMinor: 0,
				Currency:        "USD",
				IngredientDeltas: []IngredientDelta{{
					IngredientID: "chicken",
					Quantity:     1,
					Unit:         ingredient.UnitEach,
				}},
			}},
		}},
	}
}
