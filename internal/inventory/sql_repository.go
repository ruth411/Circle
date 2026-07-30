package inventory

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/ruth411/circle/internal/contracts"
	"github.com/ruth411/circle/internal/core/ingredient"
)

const (
	movementTypeDepletion   = "depletion"
	movementSourceClosedOrd = "closed_order"
)

type SQLRepository struct {
	db *sql.DB
}

type sqlQueryer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func NewSQLRepository(db *sql.DB) *SQLRepository {
	return &SQLRepository{db: db}
}

func (r *SQLRepository) RecordDepletion(ctx context.Context, order contracts.ClosedOrder) ([]Movement, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	usage, units, err := aggregateUsageAndUnits(order)
	if err != nil {
		return nil, err
	}
	ingredientIDs := sortedIngredientIDs(usage)
	if len(ingredientIDs) == 0 {
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return nil, nil
	}

	var movements []Movement
	for i, ingredientID := range ingredientIDs {
		unit, ok := units[ingredientID]
		if !ok {
			return nil, fmt.Errorf("%w: missing unit snapshot for ingredient %s", ErrInvalidClosedOrderData, ingredientID)
		}

		movement := Movement{
			ID:           fmt.Sprintf("%s-%d", order.OrderID, i+1),
			LocationID:   order.LocationID,
			OrderID:      order.OrderID,
			IngredientID: ingredientID,
			Quantity:     -usage[ingredientID],
			Unit:         unit,
			OccurredAt:   order.ClosedAt.UTC(),
		}

		result, err := tx.ExecContext(ctx, `
INSERT INTO inventory.inventory_movements (
    id,
    location_id,
    ingredient_id,
    movement_type,
    source_type,
    source_id,
    quantity_base_units,
    unit,
    occurred_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (location_id, source_type, source_id, ingredient_id) DO NOTHING;
`, movement.ID, movement.LocationID, movement.IngredientID, movementTypeDepletion, movementSourceClosedOrd, movement.OrderID, movement.Quantity, string(movement.Unit), movement.OccurredAt)
		if err != nil {
			return nil, err
		}
		rowsAffected, err := result.RowsAffected()
		if err != nil {
			return nil, err
		}
		if rowsAffected == 1 {
			movements = append(movements, movement)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return movements, nil
}

func (r *SQLRepository) ListMovements(ctx context.Context, locationID string) ([]Movement, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT
    id,
    location_id,
    source_id,
    ingredient_id,
    quantity_base_units,
    unit,
    occurred_at
FROM inventory.inventory_movements
WHERE location_id = $1
ORDER BY occurred_at, created_at, id;
`, locationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var movements []Movement
	for rows.Next() {
		var movement Movement
		var unit string
		if err := rows.Scan(&movement.ID, &movement.LocationID, &movement.OrderID, &movement.IngredientID, &movement.Quantity, &unit, &movement.OccurredAt); err != nil {
			return nil, err
		}
		movement.Unit = ingredient.Unit(unit)
		movements = append(movements, movement)
	}
	return movements, rows.Err()
}
