package recipe

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/ruth411/circle/internal/core/ingredient"
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

func (r *SQLRepository) Get(ctx context.Context, locationID string, recipeID string) (Recipe, error) {
	const query = `
SELECT id, location_id, name, yield_count, created_at, updated_at
FROM recipe.recipes
WHERE location_id = $1
  AND id = $2;
`

	row := r.db.QueryRowContext(ctx, query, locationID, recipeID)
	recipe, err := scanRecipe(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Recipe{}, ErrRecipeNotFound
		}
		return Recipe{}, err
	}
	lines, err := loadLines(ctx, r.db, recipe.ID)
	if err != nil {
		return Recipe{}, err
	}
	recipe.Lines = lines
	return recipe, nil
}

func (r *SQLRepository) List(ctx context.Context, locationID string) ([]Recipe, error) {
	const query = `
SELECT id, location_id, name, yield_count, created_at, updated_at
FROM recipe.recipes
WHERE location_id = $1
ORDER BY name, id;
`

	rows, err := r.db.QueryContext(ctx, query, locationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var recipes []Recipe
	for rows.Next() {
		current, err := scanRecipe(rows)
		if err != nil {
			return nil, err
		}
		recipes = append(recipes, current)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// ponytail: N+1 line loads are fine for the small phase-3 catalog; switch to set-based loading if recipe counts grow.
	for i := range recipes {
		lines, err := loadLines(ctx, r.db, recipes[i].ID)
		if err != nil {
			return nil, err
		}
		recipes[i].Lines = lines
	}

	return recipes, nil
}

func (r *SQLRepository) Create(ctx context.Context, recipe Recipe) (Recipe, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Recipe{}, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if err = upsertRecipe(ctx, tx, recipe, false); err != nil {
		return Recipe{}, err
	}
	if err = replaceLines(ctx, tx, recipe); err != nil {
		return Recipe{}, err
	}

	created, err := getRecipe(ctx, tx, recipe.LocationID, recipe.ID)
	if err != nil {
		return Recipe{}, err
	}
	if err = tx.Commit(); err != nil {
		return Recipe{}, err
	}
	return created, nil
}

func (r *SQLRepository) Update(ctx context.Context, recipe Recipe) (Recipe, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Recipe{}, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if err = upsertRecipe(ctx, tx, recipe, true); err != nil {
		return Recipe{}, err
	}
	if err = replaceLines(ctx, tx, recipe); err != nil {
		return Recipe{}, err
	}

	updated, err := getRecipe(ctx, tx, recipe.LocationID, recipe.ID)
	if err != nil {
		return Recipe{}, err
	}
	if err = tx.Commit(); err != nil {
		return Recipe{}, err
	}
	return updated, nil
}

func upsertRecipe(ctx context.Context, db sqlQueryer, recipe Recipe, update bool) error {
	if update {
		const query = `
UPDATE recipe.recipes
SET
    name = $3,
    yield_count = $4,
    updated_at = NOW()
WHERE id = $1
  AND location_id = $2;
`
		result, err := db.ExecContext(ctx, query, recipe.ID, recipe.LocationID, recipe.Name, recipe.YieldCount)
		if err != nil {
			return mapRecipeWriteError(err)
		}
		rowsAffected, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if rowsAffected == 0 {
			return ErrRecipeNotFound
		}
		return nil
	}

	const query = `
INSERT INTO recipe.recipes (id, location_id, name, yield_count)
VALUES ($1, $2, $3, $4);
`
	_, err := db.ExecContext(ctx, query, recipe.ID, recipe.LocationID, recipe.Name, recipe.YieldCount)
	return mapRecipeWriteError(err)
}

func replaceLines(ctx context.Context, db sqlQueryer, recipe Recipe) error {
	if _, err := db.ExecContext(ctx, `DELETE FROM recipe.recipe_lines WHERE recipe_id = $1;`, recipe.ID); err != nil {
		return err
	}

	for _, line := range recipe.Lines {
		if _, err := db.ExecContext(ctx, `
INSERT INTO recipe.recipe_lines (
    recipe_id,
    line_number,
    location_id,
    target_type,
    target_id,
    quantity,
    unit,
    prep_method
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8);
`, recipe.ID, line.LineNumber, recipe.LocationID, string(line.TargetType), line.TargetID, line.Quantity, string(line.Unit), nullIfEmpty(line.PrepMethod)); err != nil {
			return err
		}
	}

	return nil
}

func getRecipe(ctx context.Context, db sqlQueryer, locationID string, recipeID string) (Recipe, error) {
	const query = `
SELECT id, location_id, name, yield_count, created_at, updated_at
FROM recipe.recipes
WHERE location_id = $1
  AND id = $2;
`
	row := db.QueryRowContext(ctx, query, locationID, recipeID)
	recipe, err := scanRecipe(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Recipe{}, ErrRecipeNotFound
		}
		return Recipe{}, err
	}
	lines, err := loadLines(ctx, db, recipe.ID)
	if err != nil {
		return Recipe{}, err
	}
	recipe.Lines = lines
	return recipe, nil
}

type recipeScanner interface {
	Scan(...any) error
}

func scanRecipe(scanner recipeScanner) (Recipe, error) {
	var recipe Recipe
	err := scanner.Scan(
		&recipe.ID,
		&recipe.LocationID,
		&recipe.Name,
		&recipe.YieldCount,
		&recipe.CreatedAt,
		&recipe.UpdatedAt,
	)
	return recipe, err
}

func loadLines(ctx context.Context, db sqlQueryer, recipeID string) ([]RecipeLine, error) {
	rows, err := db.QueryContext(ctx, `
SELECT line_number, target_type, target_id, quantity, unit, prep_method
FROM recipe.recipe_lines
WHERE recipe_id = $1
ORDER BY line_number;
`, recipeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var lines []RecipeLine
	for rows.Next() {
		var line RecipeLine
		var targetType string
		var unit string
		var prepMethod sql.NullString
		if err := rows.Scan(&line.LineNumber, &targetType, &line.TargetID, &line.Quantity, &unit, &prepMethod); err != nil {
			return nil, err
		}
		line.TargetType = LineTargetType(targetType)
		line.Unit = ingredient.Unit(unit)
		if prepMethod.Valid {
			line.PrepMethod = prepMethod.String
		}
		lines = append(lines, line)
	}
	return lines, rows.Err()
}

func mapRecipeWriteError(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrRecipeAlreadyExists
	}
	return err
}

func nullIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}
