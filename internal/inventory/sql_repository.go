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
	movementTypeReceipt     = "receipt"
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
			ID:           movementID(movementSourceClosedOrd, order.OrderID, i+1),
			LocationID:   order.LocationID,
			SourceType:   movementSourceClosedOrd,
			SourceID:     order.OrderID,
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
`, movement.ID, movement.LocationID, movement.IngredientID, movementTypeDepletion, movement.SourceType, movement.SourceID, movement.Quantity, string(movement.Unit), movement.OccurredAt)
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

func (r *SQLRepository) RecordReceipt(ctx context.Context, receipt contracts.PurchaseReceipt) ([]Movement, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	movements, err := RecordReceiptSQL(ctx, tx, receipt)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return movements, nil
}

func RecordReceiptSQL(ctx context.Context, db sqlQueryer, receipt contracts.PurchaseReceipt) ([]Movement, error) {
	usage, units, err := aggregateReceiptLines(receipt)
	if err != nil {
		return nil, err
	}
	ingredientIDs := sortedIngredientIDs(usage)

	var movements []Movement
	for i, ingredientID := range ingredientIDs {
		unit, ok := units[ingredientID]
		if !ok {
			return nil, fmt.Errorf("%w: missing unit snapshot for ingredient %s", ErrInvalidReceiptData, ingredientID)
		}
		movement := Movement{
			ID:           movementID(receipt.SourceType, receipt.SourceID, i+1),
			LocationID:   receipt.LocationID,
			SourceType:   receipt.SourceType,
			SourceID:     receipt.SourceID,
			IngredientID: ingredientID,
			Quantity:     usage[ingredientID],
			Unit:         unit,
			OccurredAt:   receipt.OccurredAt.UTC(),
		}

		result, err := db.ExecContext(ctx, `
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
`, movement.ID, movement.LocationID, movement.IngredientID, movementTypeReceipt, movement.SourceType, movement.SourceID, movement.Quantity, string(movement.Unit), movement.OccurredAt)
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
	return movements, nil
}

func (r *SQLRepository) ListMovements(ctx context.Context, locationID string) ([]Movement, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT
    id,
    location_id,
    source_type,
    source_id,
    ingredient_id,
    quantity_base_units,
    unit,
    occurred_at
FROM inventory.inventory_movements
WHERE location_id = $1
ORDER BY occurred_at DESC, created_at DESC, id DESC;
`, locationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var movements []Movement
	for rows.Next() {
		var movement Movement
		var unit string
		if err := rows.Scan(&movement.ID, &movement.LocationID, &movement.SourceType, &movement.SourceID, &movement.IngredientID, &movement.Quantity, &unit, &movement.OccurredAt); err != nil {
			return nil, err
		}
		if movement.SourceType == movementSourceClosedOrd {
			movement.OrderID = movement.SourceID
		}
		movement.Unit = ingredient.Unit(unit)
		movements = append(movements, movement)
	}
	return movements, rows.Err()
}

func (r *SQLRepository) OnHand(ctx context.Context, locationID string) ([]OnHandItem, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT
    ingredients.location_id,
    ingredients.id,
    ingredients.name,
    ingredients.base_unit,
    COALESCE(SUM(movements.quantity_base_units), 0)
FROM ingredient.ingredients AS ingredients
LEFT JOIN inventory.inventory_movements AS movements
  ON movements.location_id = ingredients.location_id
 AND movements.ingredient_id = ingredients.id
WHERE ingredients.location_id = $1
GROUP BY ingredients.location_id, ingredients.id, ingredients.name, ingredients.base_unit
ORDER BY ingredients.name, ingredients.id;
`, locationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []OnHandItem
	for rows.Next() {
		var item OnHandItem
		var unit string
		if err := rows.Scan(&item.LocationID, &item.IngredientID, &item.IngredientName, &unit, &item.OnHandQuantity); err != nil {
			return nil, err
		}
		item.BaseUnit = ingredient.Unit(unit)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *SQLRepository) ListOrganizationMovements(ctx context.Context, organizationID string, locationID string) ([]Movement, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT
    movements.id,
    movements.location_id,
    movements.source_type,
    movements.source_id,
    movements.ingredient_id,
    movements.quantity_base_units,
    movements.unit,
    movements.occurred_at
FROM inventory.inventory_movements AS movements
JOIN tenancy.locations AS locations
  ON locations.id = movements.location_id
JOIN tenancy.restaurants AS restaurants
  ON restaurants.id = locations.restaurant_id
WHERE restaurants.organization_id = $1
  AND ($2 = '' OR movements.location_id = $2)
ORDER BY movements.occurred_at DESC, movements.created_at DESC, movements.id DESC;
`, organizationID, locationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var movements []Movement
	for rows.Next() {
		var movement Movement
		var unit string
		if err := rows.Scan(&movement.ID, &movement.LocationID, &movement.SourceType, &movement.SourceID, &movement.IngredientID, &movement.Quantity, &unit, &movement.OccurredAt); err != nil {
			return nil, err
		}
		if movement.SourceType == movementSourceClosedOrd {
			movement.OrderID = movement.SourceID
		}
		movement.Unit = ingredient.Unit(unit)
		movements = append(movements, movement)
	}
	return movements, rows.Err()
}

func (r *SQLRepository) OrganizationOnHand(ctx context.Context, organizationID string, locationID string) ([]OnHandItem, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT
    ingredients.location_id,
    ingredients.id,
    ingredients.name,
    ingredients.base_unit,
    COALESCE(SUM(movements.quantity_base_units), 0)
FROM ingredient.ingredients AS ingredients
JOIN tenancy.locations AS locations
  ON locations.id = ingredients.location_id
JOIN tenancy.restaurants AS restaurants
  ON restaurants.id = locations.restaurant_id
LEFT JOIN inventory.inventory_movements AS movements
  ON movements.location_id = ingredients.location_id
 AND movements.ingredient_id = ingredients.id
WHERE restaurants.organization_id = $1
  AND ($2 = '' OR ingredients.location_id = $2)
GROUP BY ingredients.location_id, ingredients.id, ingredients.name, ingredients.base_unit
ORDER BY ingredients.location_id, ingredients.name, ingredients.id;
`, organizationID, locationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []OnHandItem
	for rows.Next() {
		var item OnHandItem
		var unit string
		if err := rows.Scan(&item.LocationID, &item.IngredientID, &item.IngredientName, &unit, &item.OnHandQuantity); err != nil {
			return nil, err
		}
		item.BaseUnit = ingredient.Unit(unit)
		items = append(items, item)
	}
	return items, rows.Err()
}
