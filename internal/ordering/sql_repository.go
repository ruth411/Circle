package ordering

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/ruth411/circle/internal/contracts"
	"github.com/ruth411/circle/internal/core/ingredient"
	"github.com/ruth411/circle/internal/core/recipe"
	"github.com/ruth411/circle/internal/platform/biztime"
	"github.com/ruth411/circle/internal/platform/events"
)

type SQLRepository struct {
	db *sql.DB
}

type sqlQueryer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func NewSQLRepository(db *sql.DB) *SQLRepository {
	return &SQLRepository{db: db}
}

func (r *SQLRepository) Get(ctx context.Context, locationID string, orderID string) (Order, error) {
	return getOrder(ctx, r.db, locationID, orderID, false)
}

func (r *SQLRepository) Create(ctx context.Context, order Order) (Order, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Order{}, err
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	if _, err = tx.ExecContext(ctx, `
INSERT INTO ordering.orders (
    id,
    location_id,
    check_id,
    snapshot_id,
    snapshot_version,
    business_date,
    status
)
VALUES ($1, $2, $3, $4, $5, $6, $7);
`, order.ID, order.LocationID, order.CheckID, order.SnapshotID, order.SnapshotVersion, order.BusinessDate.String(), string(order.Status)); err != nil {
		_ = tx.Rollback()
		tx = nil

		existing, loadErr := getOrder(ctx, r.db, order.LocationID, order.ID, false)
		if loadErr == nil &&
			existing.CheckID == order.CheckID &&
			existing.LocationID == order.LocationID &&
			existing.SnapshotID == order.SnapshotID &&
			existing.SnapshotVersion == order.SnapshotVersion &&
			existing.BusinessDate == order.BusinessDate {
			return existing, nil
		}
		if loadErr == nil {
			return Order{}, fmt.Errorf("%w: order %s already exists with different attributes", ErrInvalidOrder, order.ID)
		}
		return Order{}, err
	}

	if _, err = tx.ExecContext(ctx, `
INSERT INTO ordering.checks (
    id,
    location_id,
    order_id,
    status,
    total_minor,
    currency
)
VALUES ($1, $2, $3, $4, 0, '');
`, order.CheckID, order.LocationID, order.ID, string(order.Status)); err != nil {
		return Order{}, err
	}

	created, err := getOrder(ctx, tx, order.LocationID, order.ID, false)
	if err != nil {
		return Order{}, err
	}
	if err = tx.Commit(); err != nil {
		return Order{}, err
	}
	tx = nil
	return created, nil
}

func (r *SQLRepository) AddLine(ctx context.Context, locationID string, orderID string, line OrderLine) (OrderLine, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return OrderLine{}, err
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	order, err := getOrder(ctx, tx, locationID, orderID, true)
	if err != nil {
		return OrderLine{}, err
	}
	if order.Status != OrderStatusOpen {
		return OrderLine{}, fmt.Errorf("%w: order %s is not editable", ErrOrderNotEditable, order.ID)
	}

	if order.Currency != "" && order.Currency != line.Currency {
		return OrderLine{}, fmt.Errorf("%w: line currency %s does not match order currency %s", ErrInvalidOrder, line.Currency, order.Currency)
	}

	if line.LineID == "" {
		var count int
		if err = tx.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM ordering.order_lines
WHERE location_id = $1
  AND order_id = $2;
`, locationID, orderID).Scan(&count); err != nil {
			return OrderLine{}, err
		}
		line.LineID = fmt.Sprintf("%s-%d", orderID, count+1)
	}

	usageJSON, err := json.Marshal(line.IngredientUsage)
	if err != nil {
		return OrderLine{}, err
	}
	unitsJSON, err := json.Marshal(line.IngredientUnits)
	if err != nil {
		return OrderLine{}, err
	}
	if _, err = tx.ExecContext(ctx, `
INSERT INTO ordering.order_lines (
    location_id,
    order_id,
    line_id,
    menu_item_id,
    name,
    quantity,
    resolved_price_minor,
    currency,
    resolved_calories,
    resolved_protein_grams,
    resolved_carbs_grams,
    resolved_fat_grams,
    ingredient_usage_json,
    ingredient_units_json
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14);
`, locationID, orderID, line.LineID, line.MenuItemID, line.Name, line.Quantity, line.ResolvedPriceMinor, line.Currency, line.ResolvedMacros.Calories, line.ResolvedMacros.ProteinGrams, line.ResolvedMacros.CarbsGrams, line.ResolvedMacros.FatGrams, usageJSON, unitsJSON); err != nil {
		if isUniqueViolation(err) {
			return OrderLine{}, fmt.Errorf("%w: line id %s already exists", ErrInvalidOrder, line.LineID)
		}
		return OrderLine{}, err
	}

	for _, modifier := range line.SelectedModifiers {
		modifierUsageJSON, marshalErr := json.Marshal(modifier.IngredientUsage)
		if marshalErr != nil {
			return OrderLine{}, marshalErr
		}
		modifierUnitsJSON, marshalErr := json.Marshal(modifier.IngredientUnits)
		if marshalErr != nil {
			return OrderLine{}, marshalErr
		}
		if _, err = tx.ExecContext(ctx, `
INSERT INTO ordering.order_line_modifiers (
    location_id,
    order_id,
    line_id,
    modifier_id,
    name,
    price_delta_minor,
    currency,
    macro_delta_calories,
    macro_delta_protein_grams,
    macro_delta_carbs_grams,
    macro_delta_fat_grams,
    ingredient_usage_json,
    ingredient_units_json
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13);
`, locationID, orderID, line.LineID, modifier.ModifierID, modifier.Name, modifier.PriceDeltaMinor, modifier.Currency, modifier.MacroDelta.Calories, modifier.MacroDelta.ProteinGrams, modifier.MacroDelta.CarbsGrams, modifier.MacroDelta.FatGrams, modifierUsageJSON, modifierUnitsJSON); err != nil {
			return OrderLine{}, err
		}
	}

	result, err := tx.ExecContext(ctx, `
UPDATE ordering.checks
SET
    total_minor = total_minor + $3,
    currency = CASE WHEN currency = '' THEN $4 ELSE currency END,
    updated_at = NOW()
WHERE location_id = $1
  AND order_id = $2
  AND (currency = '' OR currency = $4);
`, locationID, orderID, line.ResolvedPriceMinor, line.Currency)
	if err != nil {
		return OrderLine{}, err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return OrderLine{}, err
	}
	if rowsAffected == 0 {
		return OrderLine{}, fmt.Errorf("%w: line currency %s does not match order currency %s", ErrInvalidOrder, line.Currency, order.Currency)
	}

	if err = tx.Commit(); err != nil {
		return OrderLine{}, err
	}
	tx = nil
	return cloneLine(line), nil
}

func (r *SQLRepository) StartClose(ctx context.Context, locationID string, orderID string, tender Tender) (Order, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Order{}, err
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	order, err := getOrder(ctx, tx, locationID, orderID, true)
	if err != nil {
		return Order{}, err
	}
	if order.Status == OrderStatusClosed {
		if err = tx.Commit(); err != nil {
			return Order{}, err
		}
		tx = nil
		return order, nil
	}
	if order.Status == OrderStatusClosing {
		return Order{}, ErrOrderAlreadyClosing
	}
	if tender.CheckID != order.CheckID {
		return Order{}, fmt.Errorf("%w: tender check id %s does not match order check id %s", ErrInvalidOrder, tender.CheckID, order.CheckID)
	}
	if order.Currency != "" && tender.Currency != order.Currency {
		return Order{}, fmt.Errorf("%w: tender currency %s does not match order currency %s", ErrInvalidOrder, tender.Currency, order.Currency)
	}
	if tender.AmountMinor < order.TotalMinor {
		return Order{}, fmt.Errorf("%w: tender amount %d is less than order total %d", ErrUnderpaidTender, tender.AmountMinor, order.TotalMinor)
	}

	if _, err = tx.ExecContext(ctx, `
INSERT INTO ordering.tenders (
    id,
    location_id,
    check_id,
    amount_minor,
    currency,
    kind,
    status
)
VALUES ($1, $2, $3, $4, $5, $6, 'pending');
`, tender.ID, locationID, tender.CheckID, tender.AmountMinor, tender.Currency, tender.Kind); err != nil {
		return Order{}, err
	}

	if _, err = tx.ExecContext(ctx, `
UPDATE ordering.orders
SET status = 'closing', updated_at = NOW()
WHERE location_id = $1
  AND id = $2;
`, locationID, orderID); err != nil {
		return Order{}, err
	}
	if _, err = tx.ExecContext(ctx, `
UPDATE ordering.checks
SET status = 'closing', updated_at = NOW()
WHERE location_id = $1
  AND order_id = $2;
`, locationID, orderID); err != nil {
		return Order{}, err
	}

	order.Status = OrderStatusClosing
	if err = tx.Commit(); err != nil {
		return Order{}, err
	}
	tx = nil
	return order, nil
}

func (r *SQLRepository) MarkTenderSucceeded(ctx context.Context, locationID string, orderID string, tenderID string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	order, err := getOrder(ctx, tx, locationID, orderID, true)
	if err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `
UPDATE ordering.tenders
SET status = 'succeeded', processed_at = NOW()
WHERE location_id = $1
  AND id = $2
  AND check_id = $3
  AND status IN ('pending', 'succeeded');
`, locationID, tenderID, order.CheckID)
	if err != nil {
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return fmt.Errorf("%w: tender %s is not pending for check %s", ErrInvalidOrder, tenderID, order.CheckID)
	}

	if err = tx.Commit(); err != nil {
		return err
	}
	tx = nil
	return nil
}

func (r *SQLRepository) FailClose(ctx context.Context, locationID string, orderID string, tenderID string) (err error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	order, err := getOrder(ctx, tx, locationID, orderID, true)
	if err != nil {
		return err
	}
	if order.Status == OrderStatusClosing {
		if _, err = tx.ExecContext(ctx, `
UPDATE ordering.orders
SET status = 'open', updated_at = NOW()
WHERE location_id = $1
  AND id = $2;
`, locationID, orderID); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `
UPDATE ordering.checks
SET status = 'open', updated_at = NOW()
WHERE location_id = $1
  AND order_id = $2;
`, locationID, orderID); err != nil {
			return err
		}
	}
	if _, err = tx.ExecContext(ctx, `
UPDATE ordering.tenders
SET status = 'failed', processed_at = NOW()
WHERE location_id = $1
  AND id = $2
  AND check_id = $3
  AND status = 'pending';
`, locationID, tenderID, order.CheckID); err != nil {
		return err
	}

	return tx.Commit()
}

func (r *SQLRepository) FinishClose(ctx context.Context, locationID string, orderID string, tenderID string, closedAt time.Time) (Order, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Order{}, err
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	order, err := getOrder(ctx, tx, locationID, orderID, true)
	if err != nil {
		return Order{}, err
	}
	if order.Status == OrderStatusClosed {
		if err = tx.Commit(); err != nil {
			return Order{}, err
		}
		tx = nil
		return order, nil
	}
	if order.Status != OrderStatusClosing {
		return Order{}, fmt.Errorf("%w: order %s is not ready to close", ErrInvalidOrder, order.ID)
	}

	closedAt = closedAt.UTC()
	if _, err = tx.ExecContext(ctx, `
UPDATE ordering.orders
SET status = 'closed', closed_at = $3, updated_at = NOW()
WHERE location_id = $1
  AND id = $2;
`, locationID, orderID, closedAt); err != nil {
		return Order{}, err
	}
	if _, err = tx.ExecContext(ctx, `
UPDATE ordering.checks
SET status = 'closed', closed_at = $3, updated_at = NOW()
WHERE location_id = $1
  AND order_id = $2;
`, locationID, orderID, closedAt); err != nil {
		return Order{}, err
	}
	var ready int
	if err = tx.QueryRowContext(ctx, `
SELECT 1
FROM ordering.tenders
WHERE location_id = $1
  AND id = $2
  AND check_id = $3
  AND status = 'succeeded';
`, locationID, tenderID, order.CheckID).Scan(&ready); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Order{}, fmt.Errorf("%w: tender %s is not succeeded for check %s", ErrInvalidOrder, tenderID, order.CheckID)
		}
		return Order{}, err
	}

	closed, err := getOrder(ctx, tx, locationID, orderID, false)
	if err != nil {
		return Order{}, err
	}
	closedOrder, err := ToClosedOrder(closed)
	if err != nil {
		return Order{}, err
	}
	payload, err := json.Marshal(closedOrder)
	if err != nil {
		return Order{}, err
	}
	if err = events.AppendSQL(ctx, tx, events.Event{
		ID:          "evt-order-closed-" + locationID + "-" + orderID,
		Name:        contracts.ClosedOrderEventName,
		AggregateID: orderID,
		LocationID:  locationID,
		Payload:     payload,
		OccurredAt:  closedAt,
	}); err != nil {
		return Order{}, err
	}
	if err = tx.Commit(); err != nil {
		return Order{}, err
	}
	tx = nil
	return closed, nil
}

type orderScanner interface {
	Scan(...any) error
}

func scanOrder(scanner orderScanner) (Order, error) {
	var order Order
	var businessDateRaw string
	var closedAt sql.NullTime
	err := scanner.Scan(
		&order.ID,
		&order.CheckID,
		&order.LocationID,
		&order.SnapshotID,
		&order.SnapshotVersion,
		&businessDateRaw,
		&order.Status,
		&order.TotalMinor,
		&order.Currency,
		&closedAt,
	)
	if err != nil {
		return Order{}, err
	}

	businessDate, err := biztime.Parse(businessDateRaw)
	if err != nil {
		return Order{}, err
	}
	order.BusinessDate = businessDate
	if closedAt.Valid {
		value := closedAt.Time.UTC()
		order.ClosedAt = &value
	}

	return order, nil
}

func getOrder(ctx context.Context, db sqlQueryer, locationID string, orderID string, forUpdate bool) (Order, error) {
	query := `
SELECT
    orders.id,
    orders.check_id,
    orders.location_id,
    orders.snapshot_id,
    orders.snapshot_version,
    orders.business_date,
    orders.status,
    checks.total_minor,
    checks.currency,
    orders.closed_at
FROM ordering.orders AS orders
JOIN ordering.checks AS checks
    ON checks.location_id = orders.location_id
   AND checks.order_id = orders.id
WHERE orders.location_id = $1
  AND orders.id = $2`
	if forUpdate {
		query += `
FOR UPDATE OF orders, checks`
	}
	query += ";"

	row := db.QueryRowContext(ctx, query, locationID, orderID)
	order, err := scanOrder(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Order{}, ErrOrderNotFound
		}
		return Order{}, err
	}

	lines, err := loadOrderLines(ctx, db, locationID, order.ID)
	if err != nil {
		return Order{}, err
	}
	order.Lines = lines
	return order, nil
}

func loadOrderLines(ctx context.Context, db sqlQueryer, locationID string, orderID string) ([]OrderLine, error) {
	rows, err := db.QueryContext(ctx, `
SELECT
    line_id,
    menu_item_id,
    name,
    quantity,
    resolved_price_minor,
    currency,
    resolved_calories,
    resolved_protein_grams,
    resolved_carbs_grams,
    resolved_fat_grams,
    ingredient_usage_json,
    ingredient_units_json
FROM ordering.order_lines
WHERE location_id = $1
  AND order_id = $2
ORDER BY created_at, line_id;
`, locationID, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var lines []OrderLine
	for rows.Next() {
		var line OrderLine
		var usageRaw []byte
		var unitsRaw []byte
		if err := rows.Scan(&line.LineID, &line.MenuItemID, &line.Name, &line.Quantity, &line.ResolvedPriceMinor, &line.Currency, &line.ResolvedMacros.Calories, &line.ResolvedMacros.ProteinGrams, &line.ResolvedMacros.CarbsGrams, &line.ResolvedMacros.FatGrams, &usageRaw, &unitsRaw); err != nil {
			return nil, err
		}
		if err := decodeUsage(usageRaw, &line.IngredientUsage); err != nil {
			return nil, err
		}
		if err := decodeUnits(unitsRaw, &line.IngredientUnits); err != nil {
			return nil, err
		}
		lines = append(lines, line)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for i := range lines {
		modifiers, err := loadLineModifiers(ctx, db, locationID, orderID, lines[i].LineID)
		if err != nil {
			return nil, err
		}
		lines[i].SelectedModifiers = modifiers
	}

	return lines, nil
}

func loadLineModifiers(ctx context.Context, db sqlQueryer, locationID string, orderID string, lineID string) ([]recipe.SnapshotModifier, error) {
	rows, err := db.QueryContext(ctx, `
SELECT
    modifier_id,
    name,
    price_delta_minor,
    currency,
    macro_delta_calories,
    macro_delta_protein_grams,
    macro_delta_carbs_grams,
    macro_delta_fat_grams,
    ingredient_usage_json,
    ingredient_units_json
FROM ordering.order_line_modifiers
WHERE location_id = $1
  AND order_id = $2
  AND line_id = $3
ORDER BY created_at, modifier_id;
`, locationID, orderID, lineID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var modifiers []recipe.SnapshotModifier
	for rows.Next() {
		var modifier recipe.SnapshotModifier
		var usageRaw []byte
		var unitsRaw []byte
		if err := rows.Scan(&modifier.ModifierID, &modifier.Name, &modifier.PriceDeltaMinor, &modifier.Currency, &modifier.MacroDelta.Calories, &modifier.MacroDelta.ProteinGrams, &modifier.MacroDelta.CarbsGrams, &modifier.MacroDelta.FatGrams, &usageRaw, &unitsRaw); err != nil {
			return nil, err
		}
		if err := decodeUsage(usageRaw, &modifier.IngredientUsage); err != nil {
			return nil, err
		}
		if err := decodeUnits(unitsRaw, &modifier.IngredientUnits); err != nil {
			return nil, err
		}
		modifiers = append(modifiers, modifier)
	}
	return modifiers, rows.Err()
}

func decodeUsage(raw []byte, out *map[string]float64) error {
	if len(raw) == 0 {
		*out = nil
		return nil
	}
	return json.Unmarshal(raw, out)
}

func decodeUnits(raw []byte, out *map[string]ingredient.Unit) error {
	if len(raw) == 0 {
		*out = nil
		return nil
	}
	return json.Unmarshal(raw, out)
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
