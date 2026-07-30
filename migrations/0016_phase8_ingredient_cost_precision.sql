ALTER TABLE ingredient.ingredients
    ADD COLUMN IF NOT EXISTS current_cost_per_base_unit NUMERIC(12,4);

UPDATE ingredient.ingredients
SET current_cost_per_base_unit = current_cost_minor::NUMERIC(12,4)
WHERE current_cost_per_base_unit IS NULL;

ALTER TABLE ingredient.ingredients
    ALTER COLUMN current_cost_per_base_unit SET DEFAULT 0,
    ALTER COLUMN current_cost_per_base_unit SET NOT NULL;

ALTER TABLE ingredient.ingredients
    DROP CONSTRAINT IF EXISTS ingredient_ingredients_current_cost_per_base_unit_check;

ALTER TABLE ingredient.ingredients
    ADD CONSTRAINT ingredient_ingredients_current_cost_per_base_unit_check
    CHECK (current_cost_per_base_unit >= 0);
