CREATE TABLE IF NOT EXISTS recipe.recipes (
    id TEXT PRIMARY KEY,
    location_id TEXT NOT NULL REFERENCES tenancy.locations (id),
    name TEXT NOT NULL,
    yield_count NUMERIC(10,2) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (location_id, name),
    CHECK (yield_count > 0)
);

CREATE TABLE IF NOT EXISTS recipe.recipe_lines (
    recipe_id TEXT NOT NULL REFERENCES recipe.recipes (id) ON DELETE CASCADE,
    line_number INTEGER NOT NULL,
    location_id TEXT NOT NULL REFERENCES tenancy.locations (id),
    target_type TEXT NOT NULL,
    target_id TEXT NOT NULL,
    quantity NUMERIC(10,4) NOT NULL,
    unit TEXT NOT NULL,
    prep_method TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (recipe_id, line_number),
    CHECK (line_number > 0),
    CHECK (quantity > 0),
    CHECK (target_type IN ('ingredient', 'recipe'))
);

CREATE INDEX IF NOT EXISTS recipe_recipes_location_idx
    ON recipe.recipes (location_id, name);

CREATE INDEX IF NOT EXISTS recipe_lines_location_target_idx
    ON recipe.recipe_lines (location_id, target_type, target_id);

INSERT INTO recipe.recipes (id, location_id, name, yield_count)
VALUES
    ('rec-chipotle-charlotte-chicken-bowl-base', 'loc-chipotle-charlotte', 'Chipotle Chicken Bowl Base', 1),
    ('rec-chipotle-raleigh-chicken-bowl-base', 'loc-chipotle-raleigh', 'Chipotle Chicken Bowl Base', 1)
ON CONFLICT (id) DO NOTHING;

INSERT INTO recipe.recipe_lines (recipe_id, line_number, location_id, target_type, target_id, quantity, unit, prep_method)
VALUES
    ('rec-chipotle-charlotte-chicken-bowl-base', 1, 'loc-chipotle-charlotte', 'ingredient', 'ing-loc-chipotle-charlotte-cmg-1', 1, 'each', NULL),
    ('rec-chipotle-charlotte-chicken-bowl-base', 2, 'loc-chipotle-charlotte', 'ingredient', 'ing-loc-chipotle-charlotte-cmg-5001', 1, 'each', NULL),
    ('rec-chipotle-charlotte-chicken-bowl-base', 3, 'loc-chipotle-charlotte', 'ingredient', 'ing-loc-chipotle-charlotte-cmg-5051', 1, 'each', NULL),
    ('rec-chipotle-charlotte-chicken-bowl-base', 4, 'loc-chipotle-charlotte', 'ingredient', 'ing-loc-chipotle-charlotte-cmg-5201', 1, 'each', NULL),
    ('rec-chipotle-raleigh-chicken-bowl-base', 1, 'loc-chipotle-raleigh', 'ingredient', 'ing-loc-chipotle-raleigh-cmg-1', 1, 'each', NULL),
    ('rec-chipotle-raleigh-chicken-bowl-base', 2, 'loc-chipotle-raleigh', 'ingredient', 'ing-loc-chipotle-raleigh-cmg-5001', 1, 'each', NULL),
    ('rec-chipotle-raleigh-chicken-bowl-base', 3, 'loc-chipotle-raleigh', 'ingredient', 'ing-loc-chipotle-raleigh-cmg-5051', 1, 'each', NULL),
    ('rec-chipotle-raleigh-chicken-bowl-base', 4, 'loc-chipotle-raleigh', 'ingredient', 'ing-loc-chipotle-raleigh-cmg-5201', 1, 'each', NULL)
ON CONFLICT (recipe_id, line_number) DO NOTHING;
