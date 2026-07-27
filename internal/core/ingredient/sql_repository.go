package ingredient

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
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

func (r *SQLRepository) List(ctx context.Context, locationID string, search string) ([]Ingredient, error) {
	const query = `
SELECT
    id,
    location_id,
    source_item_id,
    name,
    category,
    base_unit,
    calories_per_base_unit,
    protein_grams_per_base_unit,
    carbs_grams_per_base_unit,
    fat_grams_per_base_unit,
    current_cost_minor,
    currency,
    on_hand_base_units,
    par_level_base_units,
    provenance,
    verification_status,
    serving_size_quantity,
    serving_size_unit,
    created_at,
    updated_at
FROM ingredient.ingredients
WHERE location_id = $1
  AND ($2 = '' OR name ILIKE '%' || $2 || '%' OR category ILIKE '%' || $2 || '%' OR source_item_id ILIKE '%' || $2 || '%')
ORDER BY name, id;
`

	rows, err := r.db.QueryContext(ctx, query, locationID, search)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ingredients []Ingredient
	for rows.Next() {
		ingredient, err := scanIngredient(rows)
		if err != nil {
			return nil, err
		}
		ingredients = append(ingredients, ingredient)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// ponytail: N+1 unit/yield loads are fine for the small phase-2 catalog; switch to set-based joins when list latency matters.
	for i := range ingredients {
		if err := loadUnits(ctx, r.db, &ingredients[i]); err != nil {
			return nil, err
		}
		if err := loadYieldFactors(ctx, r.db, &ingredients[i]); err != nil {
			return nil, err
		}
	}

	return ingredients, nil
}

func (r *SQLRepository) Create(ctx context.Context, ingredient Ingredient) (Ingredient, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Ingredient{}, err
	}

	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if err = upsertIngredient(ctx, tx, ingredient, false); err != nil {
		return Ingredient{}, err
	}
	if err = replaceUnits(ctx, tx, ingredient.ID, ingredient.AlternateUnits); err != nil {
		return Ingredient{}, err
	}
	if err = replaceYieldFactors(ctx, tx, ingredient.ID, ingredient.YieldFactors); err != nil {
		return Ingredient{}, err
	}

	created, err := getByID(ctx, tx, ingredient.LocationID, ingredient.ID)
	if err != nil {
		return Ingredient{}, err
	}
	if err = tx.Commit(); err != nil {
		return Ingredient{}, err
	}

	return created, nil
}

func (r *SQLRepository) Update(ctx context.Context, ingredient Ingredient) (Ingredient, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Ingredient{}, err
	}

	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if err = upsertIngredient(ctx, tx, ingredient, true); err != nil {
		return Ingredient{}, err
	}
	if err = replaceUnits(ctx, tx, ingredient.ID, ingredient.AlternateUnits); err != nil {
		return Ingredient{}, err
	}
	if err = replaceYieldFactors(ctx, tx, ingredient.ID, ingredient.YieldFactors); err != nil {
		return Ingredient{}, err
	}

	updated, err := getByID(ctx, tx, ingredient.LocationID, ingredient.ID)
	if err != nil {
		return Ingredient{}, err
	}
	if err = tx.Commit(); err != nil {
		return Ingredient{}, err
	}

	return updated, nil
}

func upsertIngredient(ctx context.Context, db sqlQueryer, ingredient Ingredient, update bool) error {
	if update {
		const query = `
UPDATE ingredient.ingredients
SET
    source_item_id = $3,
    name = $4,
    category = $5,
    base_unit = $6,
    calories_per_base_unit = $7,
    protein_grams_per_base_unit = $8,
    carbs_grams_per_base_unit = $9,
    fat_grams_per_base_unit = $10,
    current_cost_minor = $11,
    currency = $12,
    on_hand_base_units = $13,
    par_level_base_units = $14,
    provenance = $15,
    verification_status = $16,
    serving_size_quantity = $17,
    serving_size_unit = $18,
    updated_at = NOW()
WHERE id = $1
  AND location_id = $2;
`
		result, err := db.ExecContext(ctx, query,
			ingredient.ID,
			ingredient.LocationID,
			ingredient.SourceItemID,
			ingredient.Name,
			ingredient.Category,
			string(ingredient.BaseUnit),
			ingredient.MacrosPerBaseUnit.Calories,
			ingredient.MacrosPerBaseUnit.ProteinGrams,
			ingredient.MacrosPerBaseUnit.CarbsGrams,
			ingredient.MacrosPerBaseUnit.FatGrams,
			ingredient.CurrentCostMinor,
			ingredient.Currency,
			ingredient.OnHandBaseUnits,
			ingredient.ParLevelBaseUnits,
			string(ingredient.Provenance),
			string(ingredient.VerificationStatus),
			ingredient.ServingSizeQuantity,
			ingredient.ServingSizeUnit,
		)
		if err != nil {
			return err
		}

		rowsAffected, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if rowsAffected == 0 {
			return ErrNotFound
		}
		return nil
	}

	const query = `
INSERT INTO ingredient.ingredients (
    id,
    location_id,
    source_item_id,
    name,
    category,
    base_unit,
    calories_per_base_unit,
    protein_grams_per_base_unit,
    carbs_grams_per_base_unit,
    fat_grams_per_base_unit,
    current_cost_minor,
    currency,
    on_hand_base_units,
    par_level_base_units,
    provenance,
    verification_status,
    serving_size_quantity,
    serving_size_unit
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18);
`
	_, err := db.ExecContext(ctx, query,
		ingredient.ID,
		ingredient.LocationID,
		ingredient.SourceItemID,
		ingredient.Name,
		ingredient.Category,
		string(ingredient.BaseUnit),
		ingredient.MacrosPerBaseUnit.Calories,
		ingredient.MacrosPerBaseUnit.ProteinGrams,
		ingredient.MacrosPerBaseUnit.CarbsGrams,
		ingredient.MacrosPerBaseUnit.FatGrams,
		ingredient.CurrentCostMinor,
		ingredient.Currency,
		ingredient.OnHandBaseUnits,
		ingredient.ParLevelBaseUnits,
		string(ingredient.Provenance),
		string(ingredient.VerificationStatus),
		ingredient.ServingSizeQuantity,
		ingredient.ServingSizeUnit,
	)
	return err
}

func replaceUnits(ctx context.Context, db sqlQueryer, ingredientID string, units map[Unit]float64) error {
	if _, err := db.ExecContext(ctx, `DELETE FROM ingredient.ingredient_units WHERE ingredient_id = $1;`, ingredientID); err != nil {
		return err
	}
	if len(units) == 0 {
		return nil
	}

	keys := make([]string, 0, len(units))
	for unit := range units {
		keys = append(keys, string(unit))
	}
	slices.Sort(keys)

	for _, key := range keys {
		if _, err := db.ExecContext(ctx, `
INSERT INTO ingredient.ingredient_units (id, ingredient_id, unit_name, to_base_unit_factor)
VALUES ($1, $2, $3, $4);
`, fmt.Sprintf("unit-%s-%s", ingredientID, key), ingredientID, key, units[Unit(key)]); err != nil {
			return err
		}
	}

	return nil
}

func replaceYieldFactors(ctx context.Context, db sqlQueryer, ingredientID string, yieldFactors map[string]float64) error {
	if _, err := db.ExecContext(ctx, `DELETE FROM ingredient.ingredient_yield_factors WHERE ingredient_id = $1;`, ingredientID); err != nil {
		return err
	}
	if len(yieldFactors) == 0 {
		return nil
	}

	keys := make([]string, 0, len(yieldFactors))
	for method := range yieldFactors {
		keys = append(keys, method)
	}
	slices.Sort(keys)

	for _, key := range keys {
		if _, err := db.ExecContext(ctx, `
INSERT INTO ingredient.ingredient_yield_factors (id, ingredient_id, prep_method, yield_factor)
VALUES ($1, $2, $3, $4);
`, fmt.Sprintf("yield-%s-%s", ingredientID, key), ingredientID, key, yieldFactors[key]); err != nil {
			return err
		}
	}

	return nil
}

func getByID(ctx context.Context, db sqlQueryer, locationID string, ingredientID string) (Ingredient, error) {
	const query = `
SELECT
    id,
    location_id,
    source_item_id,
    name,
    category,
    base_unit,
    calories_per_base_unit,
    protein_grams_per_base_unit,
    carbs_grams_per_base_unit,
    fat_grams_per_base_unit,
    current_cost_minor,
    currency,
    on_hand_base_units,
    par_level_base_units,
    provenance,
    verification_status,
    serving_size_quantity,
    serving_size_unit,
    created_at,
    updated_at
FROM ingredient.ingredients
WHERE location_id = $1
  AND id = $2;
`

	row := db.QueryRowContext(ctx, query, locationID, ingredientID)
	ingredient, err := scanIngredient(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return Ingredient{}, ErrNotFound
		}
		return Ingredient{}, err
	}
	if err := loadUnits(ctx, db, &ingredient); err != nil {
		return Ingredient{}, err
	}
	if err := loadYieldFactors(ctx, db, &ingredient); err != nil {
		return Ingredient{}, err
	}
	return ingredient, nil
}

type ingredientScanner interface {
	Scan(...any) error
}

func scanIngredient(scanner ingredientScanner) (Ingredient, error) {
	var ingredient Ingredient
	var baseUnit string
	var provenance string
	var verificationStatus string

	err := scanner.Scan(
		&ingredient.ID,
		&ingredient.LocationID,
		&ingredient.SourceItemID,
		&ingredient.Name,
		&ingredient.Category,
		&baseUnit,
		&ingredient.MacrosPerBaseUnit.Calories,
		&ingredient.MacrosPerBaseUnit.ProteinGrams,
		&ingredient.MacrosPerBaseUnit.CarbsGrams,
		&ingredient.MacrosPerBaseUnit.FatGrams,
		&ingredient.CurrentCostMinor,
		&ingredient.Currency,
		&ingredient.OnHandBaseUnits,
		&ingredient.ParLevelBaseUnits,
		&provenance,
		&verificationStatus,
		&ingredient.ServingSizeQuantity,
		&ingredient.ServingSizeUnit,
		&ingredient.CreatedAt,
		&ingredient.UpdatedAt,
	)
	if err != nil {
		return Ingredient{}, err
	}

	ingredient.BaseUnit = Unit(baseUnit)
	ingredient.Provenance = Provenance(provenance)
	ingredient.VerificationStatus = VerificationStatus(verificationStatus)
	return ingredient, nil
}

func loadUnits(ctx context.Context, db sqlQueryer, ingredient *Ingredient) error {
	rows, err := db.QueryContext(ctx, `
SELECT unit_name, to_base_unit_factor
FROM ingredient.ingredient_units
WHERE ingredient_id = $1
ORDER BY unit_name;
`, ingredient.ID)
	if err != nil {
		return err
	}
	defer rows.Close()

	var units map[Unit]float64
	for rows.Next() {
		var unit string
		var factor float64
		if err := rows.Scan(&unit, &factor); err != nil {
			return err
		}
		if units == nil {
			units = map[Unit]float64{}
		}
		units[Unit(unit)] = factor
	}
	if err := rows.Err(); err != nil {
		return err
	}

	ingredient.AlternateUnits = units
	return nil
}

func loadYieldFactors(ctx context.Context, db sqlQueryer, ingredient *Ingredient) error {
	rows, err := db.QueryContext(ctx, `
SELECT prep_method, yield_factor
FROM ingredient.ingredient_yield_factors
WHERE ingredient_id = $1
ORDER BY prep_method;
`, ingredient.ID)
	if err != nil {
		return err
	}
	defer rows.Close()

	var yieldFactors map[string]float64
	for rows.Next() {
		var method string
		var factor float64
		if err := rows.Scan(&method, &factor); err != nil {
			return err
		}
		if yieldFactors == nil {
			yieldFactors = map[string]float64{}
		}
		yieldFactors[method] = factor
	}
	if err := rows.Err(); err != nil {
		return err
	}

	ingredient.YieldFactors = yieldFactors
	return nil
}
