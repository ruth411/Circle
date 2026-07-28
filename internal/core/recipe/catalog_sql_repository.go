package recipe

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/ruth411/circle/internal/core/ingredient"
)

func (r *SQLRepository) GetMenuItem(ctx context.Context, locationID string, menuItemID string) (MenuItem, error) {
	return getMenuItem(ctx, r.db, locationID, menuItemID)
}

func (r *SQLRepository) ListMenuItems(ctx context.Context, locationID string) ([]MenuItem, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, location_id, recipe_id, name, description, price_minor, currency, created_at, updated_at
FROM recipe.menu_items
WHERE location_id = $1
ORDER BY name, id;
`, locationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []MenuItem
	for rows.Next() {
		item, err := scanMenuItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// ponytail: N+1 nested loads are fine for the small phase-4 catalog; switch to set-based loading when menu size justifies it.
	for i := range items {
		groups, err := loadModifierGroups(ctx, r.db, items[i].LocationID, items[i].ID)
		if err != nil {
			return nil, err
		}
		items[i].ModifierGroups = groups
	}

	return items, nil
}

func (r *SQLRepository) CreateMenuItem(ctx context.Context, item MenuItem) (MenuItem, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return MenuItem{}, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if err = upsertMenuItem(ctx, tx, item, false); err != nil {
		return MenuItem{}, err
	}
	if err = replaceModifierGroups(ctx, tx, item); err != nil {
		return MenuItem{}, err
	}

	created, err := getMenuItem(ctx, tx, item.LocationID, item.ID)
	if err != nil {
		return MenuItem{}, err
	}
	if err = tx.Commit(); err != nil {
		return MenuItem{}, err
	}
	return created, nil
}

func (r *SQLRepository) UpdateMenuItem(ctx context.Context, item MenuItem) (MenuItem, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return MenuItem{}, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if err = upsertMenuItem(ctx, tx, item, true); err != nil {
		return MenuItem{}, err
	}
	if err = replaceModifierGroups(ctx, tx, item); err != nil {
		return MenuItem{}, err
	}

	updated, err := getMenuItem(ctx, tx, item.LocationID, item.ID)
	if err != nil {
		return MenuItem{}, err
	}
	if err = tx.Commit(); err != nil {
		return MenuItem{}, err
	}
	return updated, nil
}

func (r *SQLRepository) CreateSnapshot(ctx context.Context, snapshot MenuSnapshot) (MenuSnapshot, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return MenuSnapshot{}, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if err = lockSnapshotSeries(ctx, tx, snapshot.LocationID); err != nil {
		return MenuSnapshot{}, err
	}

	version, err := nextSnapshotVersion(ctx, tx, snapshot.LocationID)
	if err != nil {
		return MenuSnapshot{}, err
	}
	snapshot.Version = version

	if _, err = tx.ExecContext(ctx, `
INSERT INTO recipe.menu_snapshots (id, location_id, version)
VALUES ($1, $2, $3);
`, snapshot.ID, snapshot.LocationID, snapshot.Version); err != nil {
		return MenuSnapshot{}, err
	}

	if err = insertSnapshotItems(ctx, tx, snapshot); err != nil {
		return MenuSnapshot{}, err
	}

	created, err := getSnapshot(ctx, tx, snapshot.LocationID, snapshot.ID)
	if err != nil {
		return MenuSnapshot{}, err
	}
	if err = tx.Commit(); err != nil {
		return MenuSnapshot{}, err
	}
	return created, nil
}

func (r *SQLRepository) GetSnapshot(ctx context.Context, locationID string, snapshotID string) (MenuSnapshot, error) {
	return getSnapshot(ctx, r.db, locationID, snapshotID)
}

func (r *SQLRepository) ListSnapshots(ctx context.Context, locationID string) ([]MenuSnapshot, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, location_id, version, created_at
FROM recipe.menu_snapshots
WHERE location_id = $1
ORDER BY version DESC, id;
`, locationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var snapshots []MenuSnapshot
	for rows.Next() {
		var snapshot MenuSnapshot
		if err := rows.Scan(&snapshot.ID, &snapshot.LocationID, &snapshot.Version, &snapshot.CreatedAt); err != nil {
			return nil, err
		}
		snapshots = append(snapshots, snapshot)
	}
	return snapshots, rows.Err()
}

type menuItemScanner interface {
	Scan(...any) error
}

func scanMenuItem(scanner menuItemScanner) (MenuItem, error) {
	var item MenuItem
	err := scanner.Scan(
		&item.ID,
		&item.LocationID,
		&item.RecipeID,
		&item.Name,
		&item.Description,
		&item.PriceMinor,
		&item.Currency,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	return item, err
}

func getMenuItem(ctx context.Context, db sqlQueryer, locationID string, menuItemID string) (MenuItem, error) {
	row := db.QueryRowContext(ctx, `
SELECT id, location_id, recipe_id, name, description, price_minor, currency, created_at, updated_at
FROM recipe.menu_items
WHERE location_id = $1
  AND id = $2;
`, locationID, menuItemID)

	item, err := scanMenuItem(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return MenuItem{}, ErrMenuItemNotFound
		}
		return MenuItem{}, err
	}
	groups, err := loadModifierGroups(ctx, db, item.LocationID, item.ID)
	if err != nil {
		return MenuItem{}, err
	}
	item.ModifierGroups = groups
	return item, nil
}

func upsertMenuItem(ctx context.Context, db sqlQueryer, item MenuItem, update bool) error {
	if update {
		result, err := db.ExecContext(ctx, `
UPDATE recipe.menu_items
SET
    recipe_id = $3,
    name = $4,
    description = $5,
    price_minor = $6,
    currency = $7,
    updated_at = NOW()
WHERE id = $1
  AND location_id = $2;
`, item.ID, item.LocationID, item.RecipeID, item.Name, item.Description, item.PriceMinor, item.Currency)
		if err != nil {
			return err
		}
		rowsAffected, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if rowsAffected == 0 {
			return ErrMenuItemNotFound
		}
		return nil
	}

	_, err := db.ExecContext(ctx, `
INSERT INTO recipe.menu_items (id, location_id, recipe_id, name, description, price_minor, currency)
VALUES ($1, $2, $3, $4, $5, $6, $7);
`, item.ID, item.LocationID, item.RecipeID, item.Name, item.Description, item.PriceMinor, item.Currency)
	return err
}

func replaceModifierGroups(ctx context.Context, db sqlQueryer, item MenuItem) error {
	if _, err := db.ExecContext(ctx, `DELETE FROM recipe.modifier_groups WHERE location_id = $1 AND menu_item_id = $2;`, item.LocationID, item.ID); err != nil {
		return err
	}

	for _, group := range item.ModifierGroups {
		defaultsJSON, err := json.Marshal(group.DefaultModifierIDs)
		if err != nil {
			return err
		}
		if _, err := db.ExecContext(ctx, `
INSERT INTO recipe.modifier_groups (
    id,
    location_id,
    menu_item_id,
    name,
    selection_min,
    selection_max,
    required,
    exclusive,
    default_modifier_ids
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9);
`, group.ID, item.LocationID, item.ID, group.Name, group.SelectionMin, group.SelectionMax, group.Required, group.Exclusive, defaultsJSON); err != nil {
			return err
		}

		for _, modifier := range group.Modifiers {
			if _, err := db.ExecContext(ctx, `
INSERT INTO recipe.modifiers (
    id,
    location_id,
    modifier_group_id,
    name,
    price_delta_minor,
    currency
)
VALUES ($1, $2, $3, $4, $5, $6);
`, modifier.ID, item.LocationID, group.ID, modifier.Name, modifier.PriceDeltaMinor, modifier.Currency); err != nil {
				return err
			}

			for i, delta := range modifier.IngredientDeltas {
				if _, err := db.ExecContext(ctx, `
INSERT INTO recipe.modifier_ingredient_deltas (
    modifier_id,
    line_number,
    location_id,
    ingredient_id,
    quantity,
    unit,
    prep_method
)
VALUES ($1, $2, $3, $4, $5, $6, $7);
`, modifier.ID, i+1, item.LocationID, delta.IngredientID, delta.Quantity, string(delta.Unit), nullIfEmpty(delta.PrepMethod)); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func loadModifierGroups(ctx context.Context, db sqlQueryer, locationID string, menuItemID string) ([]ModifierGroup, error) {
	rows, err := db.QueryContext(ctx, `
SELECT id, name, selection_min, selection_max, required, exclusive, default_modifier_ids
FROM recipe.modifier_groups
WHERE location_id = $1
  AND menu_item_id = $2
ORDER BY name, id;
`, locationID, menuItemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []ModifierGroup
	for rows.Next() {
		var group ModifierGroup
		var defaultsRaw []byte
		if err := rows.Scan(&group.ID, &group.Name, &group.SelectionMin, &group.SelectionMax, &group.Required, &group.Exclusive, &defaultsRaw); err != nil {
			return nil, err
		}
		if err := decodeStringSlice(defaultsRaw, &group.DefaultModifierIDs); err != nil {
			return nil, err
		}
		modifiers, err := loadModifiers(ctx, db, locationID, group.ID)
		if err != nil {
			return nil, err
		}
		group.Modifiers = modifiers
		groups = append(groups, group)
	}

	return groups, rows.Err()
}

func loadModifiers(ctx context.Context, db sqlQueryer, locationID string, groupID string) ([]Modifier, error) {
	rows, err := db.QueryContext(ctx, `
SELECT id, name, price_delta_minor, currency
FROM recipe.modifiers
WHERE location_id = $1
  AND modifier_group_id = $2
ORDER BY name, id;
`, locationID, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var modifiers []Modifier
	for rows.Next() {
		var modifier Modifier
		if err := rows.Scan(&modifier.ID, &modifier.Name, &modifier.PriceDeltaMinor, &modifier.Currency); err != nil {
			return nil, err
		}
		deltas, err := loadModifierIngredientDeltas(ctx, db, locationID, modifier.ID)
		if err != nil {
			return nil, err
		}
		modifier.IngredientDeltas = deltas
		modifiers = append(modifiers, modifier)
	}

	return modifiers, rows.Err()
}

func loadModifierIngredientDeltas(ctx context.Context, db sqlQueryer, locationID string, modifierID string) ([]IngredientDelta, error) {
	rows, err := db.QueryContext(ctx, `
SELECT ingredient_id, quantity, unit, prep_method
FROM recipe.modifier_ingredient_deltas
WHERE location_id = $1
  AND modifier_id = $2
ORDER BY line_number;
`, locationID, modifierID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var deltas []IngredientDelta
	for rows.Next() {
		var delta IngredientDelta
		var unit string
		var prepMethod sql.NullString
		if err := rows.Scan(&delta.IngredientID, &delta.Quantity, &unit, &prepMethod); err != nil {
			return nil, err
		}
		delta.Unit = ingredient.Unit(unit)
		if prepMethod.Valid {
			delta.PrepMethod = prepMethod.String
		}
		deltas = append(deltas, delta)
	}
	return deltas, rows.Err()
}

func nextSnapshotVersion(ctx context.Context, db sqlQueryer, locationID string) (int, error) {
	var version int
	err := db.QueryRowContext(ctx, `
SELECT COALESCE(MAX(version), 0) + 1
FROM recipe.menu_snapshots
WHERE location_id = $1;
`, locationID).Scan(&version)
	return version, err
}

func lockSnapshotSeries(ctx context.Context, db sqlQueryer, locationID string) error {
	// ponytail: a per-location advisory lock is the smallest reliable fix for version allocation races.
	rows, err := db.QueryContext(ctx, `
SELECT pg_advisory_xact_lock(hashtext('recipe.menu_snapshots'), hashtext($1));
`, locationID)
	if err != nil {
		return err
	}
	defer rows.Close()
	return rows.Err()
}

func insertSnapshotItems(ctx context.Context, db sqlQueryer, snapshot MenuSnapshot) error {
	for _, item := range snapshot.Items {
		usageJSON, err := json.Marshal(item.IngredientUsage)
		if err != nil {
			return err
		}
		if _, err := db.ExecContext(ctx, `
INSERT INTO recipe.menu_snapshot_items (
    snapshot_id,
    menu_item_id,
    name,
    description,
    price_minor,
    currency,
    calories,
    protein_grams,
    carbs_grams,
    fat_grams,
    ingredient_usage_json
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11);
`, snapshot.ID, item.MenuItemID, item.Name, item.Description, item.PriceMinor, item.Currency, item.Macros.Calories, item.Macros.ProteinGrams, item.Macros.CarbsGrams, item.Macros.FatGrams, usageJSON); err != nil {
			return err
		}

		for _, group := range item.ModifierGroups {
			defaultsJSON, err := json.Marshal(group.DefaultModifierIDs)
			if err != nil {
				return err
			}
			if _, err := db.ExecContext(ctx, `
INSERT INTO recipe.menu_snapshot_modifier_groups (
    snapshot_id,
    menu_item_id,
    group_id,
    name,
    selection_min,
    selection_max,
    required,
    exclusive,
    default_modifier_ids
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9);
`, snapshot.ID, item.MenuItemID, group.GroupID, group.Name, group.SelectionMin, group.SelectionMax, group.Required, group.Exclusive, defaultsJSON); err != nil {
				return err
			}

			for _, modifier := range group.Modifiers {
				usageJSON, err := json.Marshal(modifier.IngredientUsage)
				if err != nil {
					return err
				}
				if _, err := db.ExecContext(ctx, `
INSERT INTO recipe.menu_snapshot_modifiers (
    snapshot_id,
    group_id,
    modifier_id,
    name,
    price_delta_minor,
    currency,
    calories,
    protein_grams,
    carbs_grams,
    fat_grams,
    ingredient_usage_json
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11);
`, snapshot.ID, group.GroupID, modifier.ModifierID, modifier.Name, modifier.PriceDeltaMinor, modifier.Currency, modifier.MacroDelta.Calories, modifier.MacroDelta.ProteinGrams, modifier.MacroDelta.CarbsGrams, modifier.MacroDelta.FatGrams, usageJSON); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func getSnapshot(ctx context.Context, db sqlQueryer, locationID string, snapshotID string) (MenuSnapshot, error) {
	var snapshot MenuSnapshot
	err := db.QueryRowContext(ctx, `
SELECT id, location_id, version, created_at
FROM recipe.menu_snapshots
WHERE location_id = $1
  AND id = $2;
`, locationID, snapshotID).Scan(&snapshot.ID, &snapshot.LocationID, &snapshot.Version, &snapshot.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return MenuSnapshot{}, ErrSnapshotNotFound
		}
		return MenuSnapshot{}, err
	}

	items, err := loadSnapshotItems(ctx, db, snapshot.ID)
	if err != nil {
		return MenuSnapshot{}, err
	}
	snapshot.Items = items
	return snapshot, nil
}

func loadSnapshotItems(ctx context.Context, db sqlQueryer, snapshotID string) ([]SnapshotItem, error) {
	rows, err := db.QueryContext(ctx, `
SELECT menu_item_id, name, description, price_minor, currency, calories, protein_grams, carbs_grams, fat_grams, ingredient_usage_json
FROM recipe.menu_snapshot_items
WHERE snapshot_id = $1
ORDER BY name, menu_item_id;
`, snapshotID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []SnapshotItem
	for rows.Next() {
		var item SnapshotItem
		var usageRaw []byte
		if err := rows.Scan(&item.MenuItemID, &item.Name, &item.Description, &item.PriceMinor, &item.Currency, &item.Macros.Calories, &item.Macros.ProteinGrams, &item.Macros.CarbsGrams, &item.Macros.FatGrams, &usageRaw); err != nil {
			return nil, err
		}
		if err := decodeIngredientUsage(usageRaw, &item.IngredientUsage); err != nil {
			return nil, err
		}
		groups, err := loadSnapshotModifierGroups(ctx, db, snapshotID, item.MenuItemID)
		if err != nil {
			return nil, err
		}
		item.ModifierGroups = groups
		items = append(items, item)
	}

	return items, rows.Err()
}

func loadSnapshotModifierGroups(ctx context.Context, db sqlQueryer, snapshotID string, menuItemID string) ([]SnapshotModifierGroup, error) {
	rows, err := db.QueryContext(ctx, `
SELECT group_id, name, selection_min, selection_max, required, exclusive, default_modifier_ids
FROM recipe.menu_snapshot_modifier_groups
WHERE snapshot_id = $1
  AND menu_item_id = $2
ORDER BY name, group_id;
`, snapshotID, menuItemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []SnapshotModifierGroup
	for rows.Next() {
		var group SnapshotModifierGroup
		var defaultsRaw []byte
		if err := rows.Scan(&group.GroupID, &group.Name, &group.SelectionMin, &group.SelectionMax, &group.Required, &group.Exclusive, &defaultsRaw); err != nil {
			return nil, err
		}
		if err := decodeStringSlice(defaultsRaw, &group.DefaultModifierIDs); err != nil {
			return nil, err
		}
		modifiers, err := loadSnapshotModifiers(ctx, db, snapshotID, group.GroupID)
		if err != nil {
			return nil, err
		}
		group.Modifiers = modifiers
		groups = append(groups, group)
	}

	return groups, rows.Err()
}

func loadSnapshotModifiers(ctx context.Context, db sqlQueryer, snapshotID string, groupID string) ([]SnapshotModifier, error) {
	rows, err := db.QueryContext(ctx, `
SELECT modifier_id, name, price_delta_minor, currency, calories, protein_grams, carbs_grams, fat_grams, ingredient_usage_json
FROM recipe.menu_snapshot_modifiers
WHERE snapshot_id = $1
  AND group_id = $2
ORDER BY name, modifier_id;
`, snapshotID, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var modifiers []SnapshotModifier
	for rows.Next() {
		var modifier SnapshotModifier
		var usageRaw []byte
		if err := rows.Scan(&modifier.ModifierID, &modifier.Name, &modifier.PriceDeltaMinor, &modifier.Currency, &modifier.MacroDelta.Calories, &modifier.MacroDelta.ProteinGrams, &modifier.MacroDelta.CarbsGrams, &modifier.MacroDelta.FatGrams, &usageRaw); err != nil {
			return nil, err
		}
		if err := decodeIngredientUsage(usageRaw, &modifier.IngredientUsage); err != nil {
			return nil, err
		}
		modifiers = append(modifiers, modifier)
	}

	return modifiers, rows.Err()
}

func decodeStringSlice(raw []byte, out *[]string) error {
	if len(raw) == 0 {
		*out = nil
		return nil
	}
	return json.Unmarshal(raw, out)
}

func decodeIngredientUsage(raw []byte, out *map[string]float64) error {
	if len(raw) == 0 {
		*out = nil
		return nil
	}
	return json.Unmarshal(raw, out)
}
