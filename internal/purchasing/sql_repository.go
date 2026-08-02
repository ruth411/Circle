package purchasing

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/ruth411/circle/internal/contracts"
	"github.com/ruth411/circle/internal/core/ingredient"
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

func (r *SQLRepository) CreateVendor(ctx context.Context, vendor Vendor) (Vendor, error) {
	_, err := r.db.ExecContext(ctx, `
INSERT INTO purchasing.vendors (
    location_id,
    id,
    name,
    status,
    external_ref
)
VALUES ($1, $2, $3, $4, $5);
`, vendor.LocationID, vendor.ID, vendor.Name, string(vendor.Status), vendor.ExternalRef)
	if err != nil {
		return Vendor{}, err
	}
	return r.GetVendor(ctx, vendor.LocationID, vendor.ID)
}

func (r *SQLRepository) UpdateVendor(ctx context.Context, vendor Vendor) (Vendor, error) {
	result, err := r.db.ExecContext(ctx, `
UPDATE purchasing.vendors
SET name = $3,
    status = $4,
    external_ref = $5,
    updated_at = NOW()
WHERE location_id = $1
  AND id = $2;
`, vendor.LocationID, vendor.ID, vendor.Name, string(vendor.Status), vendor.ExternalRef)
	if err != nil {
		return Vendor{}, err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return Vendor{}, err
	}
	if rowsAffected == 0 {
		return Vendor{}, ErrVendorNotFound
	}
	return r.GetVendor(ctx, vendor.LocationID, vendor.ID)
}

func (r *SQLRepository) GetVendor(ctx context.Context, locationID string, vendorID string) (Vendor, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT location_id, id, name, status, external_ref, created_at, updated_at
FROM purchasing.vendors
WHERE location_id = $1
  AND id = $2;
`, locationID, vendorID)
	vendor, err := scanVendor(row)
	if err == sql.ErrNoRows {
		return Vendor{}, ErrVendorNotFound
	}
	return vendor, err
}

func (r *SQLRepository) ListVendors(ctx context.Context, locationID string, search string) ([]Vendor, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT location_id, id, name, status, external_ref, created_at, updated_at
FROM purchasing.vendors
WHERE location_id = $1
  AND ($2 = '' OR name ILIKE '%' || $2 || '%' OR external_ref ILIKE '%' || $2 || '%')
ORDER BY name, id;
`, locationID, search)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var vendors []Vendor
	for rows.Next() {
		vendor, err := scanVendor(rows)
		if err != nil {
			return nil, err
		}
		vendors = append(vendors, vendor)
	}
	return vendors, rows.Err()
}

func (r *SQLRepository) CreateVendorItem(ctx context.Context, item VendorItem) (VendorItem, error) {
	_, err := r.db.ExecContext(ctx, `
INSERT INTO purchasing.vendor_items (
    location_id,
    id,
    vendor_id,
    ingredient_id,
    vendor_sku,
    name,
    purchase_unit,
    pack_quantity,
    ingredient_base_quantity,
    last_unit_cost,
    currency,
    status
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, ($10::NUMERIC / 10000.0), $11, $12);
`, item.LocationID, item.ID, item.VendorID, item.IngredientID, item.VendorSKU, item.Name, item.PurchaseUnit, item.PackQuantity, item.IngredientBaseQuantity, int64(item.LastUnitCost), item.Currency, string(item.Status))
	if err != nil {
		return VendorItem{}, err
	}
	return r.GetVendorItem(ctx, item.LocationID, item.ID)
}

func (r *SQLRepository) UpdateVendorItem(ctx context.Context, item VendorItem) (VendorItem, error) {
	result, err := r.db.ExecContext(ctx, `
UPDATE purchasing.vendor_items
SET vendor_id = $3,
    ingredient_id = $4,
    vendor_sku = $5,
    name = $6,
    purchase_unit = $7,
    pack_quantity = $8,
    ingredient_base_quantity = $9,
    last_unit_cost = ($10::NUMERIC / 10000.0),
    currency = $11,
    status = $12,
    updated_at = NOW()
WHERE location_id = $1
  AND id = $2;
`, item.LocationID, item.ID, item.VendorID, item.IngredientID, item.VendorSKU, item.Name, item.PurchaseUnit, item.PackQuantity, item.IngredientBaseQuantity, int64(item.LastUnitCost), item.Currency, string(item.Status))
	if err != nil {
		return VendorItem{}, err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return VendorItem{}, err
	}
	if rowsAffected == 0 {
		return VendorItem{}, ErrVendorItemNotFound
	}
	return r.GetVendorItem(ctx, item.LocationID, item.ID)
}

func (r *SQLRepository) GetVendorItem(ctx context.Context, locationID string, itemID string) (VendorItem, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT
    location_id,
    id,
    vendor_id,
    ingredient_id,
    vendor_sku,
    name,
    purchase_unit,
    pack_quantity,
    ingredient_base_quantity,
    ROUND(last_unit_cost * 10000)::BIGINT,
    currency,
    status,
    created_at,
    updated_at
FROM purchasing.vendor_items
WHERE location_id = $1
  AND id = $2;
`, locationID, itemID)
	item, err := scanVendorItem(row)
	if err == sql.ErrNoRows {
		return VendorItem{}, ErrVendorItemNotFound
	}
	return item, err
}

func (r *SQLRepository) ListVendorItems(ctx context.Context, locationID string, search string) ([]VendorItem, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT
    location_id,
    id,
    vendor_id,
    ingredient_id,
    vendor_sku,
    name,
    purchase_unit,
    pack_quantity,
    ingredient_base_quantity,
    ROUND(last_unit_cost * 10000)::BIGINT,
    currency,
    status,
    created_at,
    updated_at
FROM purchasing.vendor_items
WHERE location_id = $1
  AND ($2 = '' OR name ILIKE '%' || $2 || '%' OR vendor_sku ILIKE '%' || $2 || '%')
ORDER BY name, id;
`, locationID, search)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []VendorItem
	for rows.Next() {
		item, err := scanVendorItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *SQLRepository) CreatePurchaseOrder(ctx context.Context, po PurchaseOrder) (PurchaseOrder, error) {
	_, err := r.db.ExecContext(ctx, `
INSERT INTO purchasing.purchase_orders (
    location_id,
    id,
    po_number,
    vendor_id,
    status,
    notes
)
VALUES ($1, $2, $3, $4, $5, $6);
`, po.LocationID, po.ID, po.PONumber, po.VendorID, string(po.Status), po.Notes)
	if err != nil {
		return PurchaseOrder{}, err
	}
	return r.GetPurchaseOrder(ctx, po.LocationID, po.ID)
}

func (r *SQLRepository) GetPurchaseOrder(ctx context.Context, locationID string, poID string) (PurchaseOrder, error) {
	po, err := getPurchaseOrder(ctx, r.db, locationID, poID)
	if err != nil {
		if err == sql.ErrNoRows {
			return PurchaseOrder{}, ErrPurchaseOrderNotFound
		}
		return PurchaseOrder{}, err
	}
	return po, nil
}

func (r *SQLRepository) ListPurchaseOrders(ctx context.Context, locationID string) ([]PurchaseOrder, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT
    location_id,
    id,
    po_number,
    vendor_id,
    status,
    ordered_at,
    notes,
    created_at,
    updated_at
FROM purchasing.purchase_orders
WHERE location_id = $1
ORDER BY created_at, id;
`, locationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []PurchaseOrder
	for rows.Next() {
		po, err := scanPurchaseOrder(rows)
		if err != nil {
			return nil, err
		}
		orders = append(orders, po)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range orders {
		lines, err := loadPurchaseOrderLines(ctx, r.db, orders[i].LocationID, orders[i].ID)
		if err != nil {
			return nil, err
		}
		orders[i].Lines = lines
	}
	return orders, nil
}

func (r *SQLRepository) AddPurchaseOrderLine(ctx context.Context, locationID string, poID string, line PurchaseOrderLine) (PurchaseOrderLine, error) {
	_, err := r.db.ExecContext(ctx, `
INSERT INTO purchasing.purchase_order_lines (
    location_id,
    id,
    purchase_order_id,
    vendor_item_id,
    ordered_quantity,
    ordered_unit_cost,
    currency,
    received_quantity
)
VALUES ($1, $2, $3, $4, $5, ($6::NUMERIC / 10000.0), $7, $8);
`, locationID, line.ID, poID, line.VendorItemID, line.OrderedQuantity, int64(line.OrderedUnitCost), line.Currency, line.ReceivedQuantity)
	if err != nil {
		return PurchaseOrderLine{}, err
	}
	return getPurchaseOrderLine(ctx, r.db, locationID, line.ID)
}

func (r *SQLRepository) UpdatePurchaseOrderLine(ctx context.Context, locationID string, poID string, line PurchaseOrderLine) (PurchaseOrderLine, error) {
	result, err := r.db.ExecContext(ctx, `
UPDATE purchasing.purchase_order_lines
SET vendor_item_id = $4,
    ordered_quantity = $5,
    ordered_unit_cost = ($6::NUMERIC / 10000.0),
    currency = $7,
    updated_at = NOW()
WHERE location_id = $1
  AND purchase_order_id = $2
  AND id = $3;
`, locationID, poID, line.ID, line.VendorItemID, line.OrderedQuantity, int64(line.OrderedUnitCost), line.Currency)
	if err != nil {
		return PurchaseOrderLine{}, err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return PurchaseOrderLine{}, err
	}
	if rowsAffected == 0 {
		return PurchaseOrderLine{}, ErrPurchaseOrderNotFound
	}
	return getPurchaseOrderLine(ctx, r.db, locationID, line.ID)
}

func (r *SQLRepository) RemovePurchaseOrderLine(ctx context.Context, locationID string, poID string, lineID string) error {
	_, err := r.db.ExecContext(ctx, `
DELETE FROM purchasing.purchase_order_lines
WHERE location_id = $1
  AND purchase_order_id = $2
  AND id = $3;
`, locationID, poID, lineID)
	return err
}

func (r *SQLRepository) SubmitPurchaseOrder(ctx context.Context, locationID string, poID string) (PurchaseOrder, error) {
	result, err := r.db.ExecContext(ctx, `
UPDATE purchasing.purchase_orders
SET status = 'submitted',
    ordered_at = COALESCE(ordered_at, NOW()),
    updated_at = NOW()
WHERE location_id = $1
  AND id = $2;
`, locationID, poID)
	if err != nil {
		return PurchaseOrder{}, err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return PurchaseOrder{}, err
	}
	if rowsAffected == 0 {
		return PurchaseOrder{}, ErrPurchaseOrderNotFound
	}
	return r.GetPurchaseOrder(ctx, locationID, poID)
}

func (r *SQLRepository) CancelPurchaseOrder(ctx context.Context, locationID string, poID string) (PurchaseOrder, error) {
	result, err := r.db.ExecContext(ctx, `
UPDATE purchasing.purchase_orders
SET status = 'cancelled',
    updated_at = NOW()
WHERE location_id = $1
  AND id = $2;
`, locationID, poID)
	if err != nil {
		return PurchaseOrder{}, err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return PurchaseOrder{}, err
	}
	if rowsAffected == 0 {
		return PurchaseOrder{}, ErrPurchaseOrderNotFound
	}
	return r.GetPurchaseOrder(ctx, locationID, poID)
}

func (r *SQLRepository) Receive(ctx context.Context, planned PlannedReceipt) (Receipt, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Receipt{}, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	result, err := tx.ExecContext(ctx, `
INSERT INTO purchasing.receipts (
    location_id,
    id,
    purchase_order_id,
    received_at,
    received_by,
    notes
)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (location_id, id) DO NOTHING;
`, planned.Receipt.LocationID, planned.Receipt.ID, planned.Receipt.PurchaseOrderID, planned.Receipt.ReceivedAt.UTC(), planned.Receipt.ReceivedBy, planned.Receipt.Notes)
	if err != nil {
		return Receipt{}, err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return Receipt{}, err
	}
	if rowsAffected == 0 {
		existing, err := getReceipt(ctx, tx, planned.Receipt.LocationID, planned.Receipt.ID)
		if err != nil {
			return Receipt{}, err
		}
		if existing.PurchaseOrderID != planned.Receipt.PurchaseOrderID {
			return Receipt{}, fmt.Errorf("%w: receipt %s already exists for a different purchase order", ErrInvalidPurchase, existing.ID)
		}
		return existing, nil
	}

	for _, line := range planned.Receipt.Lines {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO purchasing.receipt_lines (
    location_id,
    id,
    receipt_id,
    purchase_order_line_id,
    ingredient_id,
    received_quantity,
    received_unit_cost,
    currency,
    inventory_quantity,
    inventory_unit
)
VALUES ($1, $2, $3, $4, $5, $6, ($7::NUMERIC / 10000.0), $8, $9, $10);
`, line.LocationID, line.ID, line.ReceiptID, line.PurchaseOrderLineID, line.IngredientID, line.ReceivedQuantity, int64(line.ReceivedUnitCost), line.Currency, line.InventoryQuantity, string(line.InventoryUnit)); err != nil {
			return Receipt{}, err
		}
		result, err := tx.ExecContext(ctx, `
UPDATE purchasing.purchase_order_lines
SET received_quantity = received_quantity + $4,
    updated_at = NOW()
WHERE location_id = $1
  AND purchase_order_id = $2
  AND id = $3
  AND received_quantity + $4 <= ordered_quantity;
`, line.LocationID, planned.Receipt.PurchaseOrderID, line.PurchaseOrderLineID, line.ReceivedQuantity)
		if err != nil {
			return Receipt{}, err
		}
		rowsAffected, err := result.RowsAffected()
		if err != nil {
			return Receipt{}, err
		}
		if rowsAffected == 0 {
			return Receipt{}, fmt.Errorf("%w: purchase order line %s cannot be over-received", ErrInvalidPurchase, line.PurchaseOrderLineID)
		}
	}

	for _, update := range planned.CostUpdates {
		if err := ingredient.ApplyReceivedCostSQL(ctx, tx, ingredient.CostUpdate{
			LocationID:      update.LocationID,
			IngredientID:    update.IngredientID,
			CostPerBaseUnit: update.CostPerBaseUnit,
			ReceivedAt:      update.ReceivedAt,
		}); err != nil {
			return Receipt{}, err
		}
	}
	if err := updatePurchaseOrderStatus(ctx, tx, planned.Receipt.LocationID, planned.Receipt.PurchaseOrderID); err != nil {
		return Receipt{}, err
	}
	payload, err := json.Marshal(planned.InventoryReceipt)
	if err != nil {
		return Receipt{}, err
	}
	if err := events.AppendSQL(ctx, tx, events.Event{
		ID:          "evt-purchase-receipt-" + planned.Receipt.LocationID + "-" + planned.Receipt.ID,
		Name:        contracts.PurchaseReceiptEventName,
		AggregateID: planned.Receipt.ID,
		LocationID:  planned.Receipt.LocationID,
		Payload:     payload,
		OccurredAt:  planned.InventoryReceipt.OccurredAt.UTC(),
	}); err != nil {
		return Receipt{}, err
	}

	receipt, err := getReceipt(ctx, tx, planned.Receipt.LocationID, planned.Receipt.ID)
	if err != nil {
		return Receipt{}, err
	}
	if err := tx.Commit(); err != nil {
		return Receipt{}, err
	}
	return receipt, nil
}

func (r *SQLRepository) ListReceipts(ctx context.Context, locationID string) ([]Receipt, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT location_id, id, purchase_order_id, received_at, received_by, notes, created_at
FROM purchasing.receipts
WHERE location_id = $1
ORDER BY received_at, created_at, id;
`, locationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var receipts []Receipt
	for rows.Next() {
		receipt, err := scanReceipt(rows)
		if err != nil {
			return nil, err
		}
		receipts = append(receipts, receipt)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range receipts {
		lines, err := loadReceiptLines(ctx, r.db, receipts[i].LocationID, receipts[i].ID)
		if err != nil {
			return nil, err
		}
		receipts[i].Lines = lines
	}
	return receipts, nil
}

func (r *SQLRepository) GetReceipt(ctx context.Context, locationID string, receiptID string) (Receipt, error) {
	receipt, err := getReceipt(ctx, r.db, locationID, receiptID)
	if err != nil {
		if err == sql.ErrNoRows {
			return Receipt{}, ErrReceiptNotFound
		}
		return Receipt{}, err
	}
	return receipt, nil
}

func updatePurchaseOrderStatus(ctx context.Context, db sqlQueryer, locationID string, poID string) error {
	lines, err := loadPurchaseOrderLines(ctx, db, locationID, poID)
	if err != nil {
		return err
	}
	status := PurchaseOrderStatusSubmitted
	allReceived := len(lines) > 0
	anyReceived := false
	for _, line := range lines {
		if line.ReceivedQuantity > 0 {
			anyReceived = true
		}
		if line.ReceivedQuantity < line.OrderedQuantity {
			allReceived = false
		}
	}
	switch {
	case allReceived:
		status = PurchaseOrderStatusReceived
	case anyReceived:
		status = PurchaseOrderStatusPartiallyReceived
	}
	_, err = db.ExecContext(ctx, `
UPDATE purchasing.purchase_orders
SET status = $3,
    updated_at = NOW()
WHERE location_id = $1
  AND id = $2;
`, locationID, poID, string(status))
	return err
}

func getPurchaseOrder(ctx context.Context, db sqlQueryer, locationID string, poID string) (PurchaseOrder, error) {
	row := db.QueryRowContext(ctx, `
SELECT location_id, id, po_number, vendor_id, status, ordered_at, notes, created_at, updated_at
FROM purchasing.purchase_orders
WHERE location_id = $1
  AND id = $2;
`, locationID, poID)
	po, err := scanPurchaseOrder(row)
	if err != nil {
		return PurchaseOrder{}, err
	}
	lines, err := loadPurchaseOrderLines(ctx, db, locationID, poID)
	if err != nil {
		return PurchaseOrder{}, err
	}
	po.Lines = lines
	return po, nil
}

func getPurchaseOrderLine(ctx context.Context, db sqlQueryer, locationID string, lineID string) (PurchaseOrderLine, error) {
	row := db.QueryRowContext(ctx, `
SELECT location_id, id, purchase_order_id, vendor_item_id, ordered_quantity, ROUND(ordered_unit_cost * 10000)::BIGINT, currency, received_quantity, created_at, updated_at
FROM purchasing.purchase_order_lines
WHERE location_id = $1
  AND id = $2;
`, locationID, lineID)
	return scanPurchaseOrderLine(row)
}

func loadPurchaseOrderLines(ctx context.Context, db sqlQueryer, locationID string, poID string) ([]PurchaseOrderLine, error) {
	rows, err := db.QueryContext(ctx, `
SELECT location_id, id, purchase_order_id, vendor_item_id, ordered_quantity, ROUND(ordered_unit_cost * 10000)::BIGINT, currency, received_quantity, created_at, updated_at
FROM purchasing.purchase_order_lines
WHERE location_id = $1
  AND purchase_order_id = $2
ORDER BY created_at, id;
`, locationID, poID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var lines []PurchaseOrderLine
	for rows.Next() {
		line, err := scanPurchaseOrderLine(rows)
		if err != nil {
			return nil, err
		}
		lines = append(lines, line)
	}
	return lines, rows.Err()
}

func getReceipt(ctx context.Context, db sqlQueryer, locationID string, receiptID string) (Receipt, error) {
	row := db.QueryRowContext(ctx, `
SELECT location_id, id, purchase_order_id, received_at, received_by, notes, created_at
FROM purchasing.receipts
WHERE location_id = $1
  AND id = $2;
`, locationID, receiptID)
	receipt, err := scanReceipt(row)
	if err != nil {
		return Receipt{}, err
	}
	lines, err := loadReceiptLines(ctx, db, locationID, receiptID)
	if err != nil {
		return Receipt{}, err
	}
	receipt.Lines = lines
	return receipt, nil
}

func loadReceiptLines(ctx context.Context, db sqlQueryer, locationID string, receiptID string) ([]ReceiptLine, error) {
	rows, err := db.QueryContext(ctx, `
SELECT location_id, id, receipt_id, purchase_order_line_id, ingredient_id, received_quantity, ROUND(received_unit_cost * 10000)::BIGINT, currency, inventory_quantity, inventory_unit, created_at
FROM purchasing.receipt_lines
WHERE location_id = $1
  AND receipt_id = $2
ORDER BY created_at, id;
`, locationID, receiptID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var lines []ReceiptLine
	for rows.Next() {
		line, err := scanReceiptLine(rows)
		if err != nil {
			return nil, err
		}
		lines = append(lines, line)
	}
	return lines, rows.Err()
}

type vendorScanner interface {
	Scan(...any) error
}

func scanVendor(scanner vendorScanner) (Vendor, error) {
	var vendor Vendor
	var status string
	err := scanner.Scan(&vendor.LocationID, &vendor.ID, &vendor.Name, &status, &vendor.ExternalRef, &vendor.CreatedAt, &vendor.UpdatedAt)
	vendor.Status = VendorStatus(status)
	return vendor, err
}

func scanVendorItem(scanner vendorScanner) (VendorItem, error) {
	var item VendorItem
	var status string
	err := scanner.Scan(&item.LocationID, &item.ID, &item.VendorID, &item.IngredientID, &item.VendorSKU, &item.Name, &item.PurchaseUnit, &item.PackQuantity, &item.IngredientBaseQuantity, (*int64)(&item.LastUnitCost), &item.Currency, &status, &item.CreatedAt, &item.UpdatedAt)
	item.Status = VendorItemStatus(status)
	return item, err
}

func scanPurchaseOrder(scanner vendorScanner) (PurchaseOrder, error) {
	var po PurchaseOrder
	var status string
	var orderedAt sql.NullTime
	err := scanner.Scan(&po.LocationID, &po.ID, &po.PONumber, &po.VendorID, &status, &orderedAt, &po.Notes, &po.CreatedAt, &po.UpdatedAt)
	po.Status = PurchaseOrderStatus(status)
	if orderedAt.Valid {
		value := orderedAt.Time.UTC()
		po.OrderedAt = &value
	}
	return po, err
}

func scanPurchaseOrderLine(scanner vendorScanner) (PurchaseOrderLine, error) {
	var line PurchaseOrderLine
	err := scanner.Scan(&line.LocationID, &line.ID, &line.PurchaseOrderID, &line.VendorItemID, &line.OrderedQuantity, (*int64)(&line.OrderedUnitCost), &line.Currency, &line.ReceivedQuantity, &line.CreatedAt, &line.UpdatedAt)
	return line, err
}

func scanReceipt(scanner vendorScanner) (Receipt, error) {
	var receipt Receipt
	err := scanner.Scan(&receipt.LocationID, &receipt.ID, &receipt.PurchaseOrderID, &receipt.ReceivedAt, &receipt.ReceivedBy, &receipt.Notes, &receipt.CreatedAt)
	return receipt, err
}

func scanReceiptLine(scanner vendorScanner) (ReceiptLine, error) {
	var line ReceiptLine
	var inventoryUnit string
	err := scanner.Scan(&line.LocationID, &line.ID, &line.ReceiptID, &line.PurchaseOrderLineID, &line.IngredientID, &line.ReceivedQuantity, (*int64)(&line.ReceivedUnitCost), &line.Currency, &line.InventoryQuantity, &inventoryUnit, &line.CreatedAt)
	line.InventoryUnit = ingredient.Unit(inventoryUnit)
	return line, err
}
