CREATE TABLE IF NOT EXISTS recipe.menu_items (
    id TEXT PRIMARY KEY,
    location_id TEXT NOT NULL REFERENCES tenancy.locations (id),
    recipe_id TEXT NOT NULL REFERENCES recipe.recipes (id),
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    price_minor BIGINT NOT NULL,
    currency TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (location_id, name),
    CHECK (price_minor >= 0)
);

CREATE TABLE IF NOT EXISTS recipe.modifier_groups (
    id TEXT PRIMARY KEY,
    location_id TEXT NOT NULL REFERENCES tenancy.locations (id),
    menu_item_id TEXT NOT NULL REFERENCES recipe.menu_items (id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    selection_min INTEGER NOT NULL,
    selection_max INTEGER NOT NULL,
    required BOOLEAN NOT NULL DEFAULT FALSE,
    exclusive BOOLEAN NOT NULL DEFAULT FALSE,
    default_modifier_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (menu_item_id, name),
    CHECK (selection_min >= 0),
    CHECK (selection_max >= 0),
    CHECK (selection_max >= selection_min),
    CHECK (NOT required OR selection_min > 0),
    CHECK (NOT exclusive OR selection_max <= 1),
    CHECK (NOT exclusive OR selection_min <= 1)
);

CREATE TABLE IF NOT EXISTS recipe.modifiers (
    id TEXT PRIMARY KEY,
    location_id TEXT NOT NULL REFERENCES tenancy.locations (id),
    modifier_group_id TEXT NOT NULL REFERENCES recipe.modifier_groups (id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    price_delta_minor BIGINT NOT NULL,
    currency TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (modifier_group_id, name)
);

CREATE TABLE IF NOT EXISTS recipe.modifier_ingredient_deltas (
    modifier_id TEXT NOT NULL REFERENCES recipe.modifiers (id) ON DELETE CASCADE,
    line_number INTEGER NOT NULL,
    location_id TEXT NOT NULL REFERENCES tenancy.locations (id),
    ingredient_id TEXT NOT NULL REFERENCES ingredient.ingredients (id),
    quantity NUMERIC(10,4) NOT NULL,
    unit TEXT NOT NULL,
    prep_method TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (modifier_id, line_number),
    CHECK (line_number > 0),
    CHECK (quantity <> 0)
);

CREATE TABLE IF NOT EXISTS recipe.menu_snapshots (
    id TEXT PRIMARY KEY,
    location_id TEXT NOT NULL REFERENCES tenancy.locations (id),
    version INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (location_id, version),
    CHECK (version > 0)
);

CREATE TABLE IF NOT EXISTS recipe.menu_snapshot_items (
    snapshot_id TEXT NOT NULL REFERENCES recipe.menu_snapshots (id) ON DELETE CASCADE,
    menu_item_id TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT NOT NULL,
    price_minor BIGINT NOT NULL,
    currency TEXT NOT NULL,
    calories NUMERIC(10,4) NOT NULL,
    protein_grams NUMERIC(10,4) NOT NULL,
    carbs_grams NUMERIC(10,4) NOT NULL,
    fat_grams NUMERIC(10,4) NOT NULL,
    ingredient_usage_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    PRIMARY KEY (snapshot_id, menu_item_id)
);

CREATE TABLE IF NOT EXISTS recipe.menu_snapshot_modifier_groups (
    snapshot_id TEXT NOT NULL,
    menu_item_id TEXT NOT NULL,
    group_id TEXT NOT NULL,
    name TEXT NOT NULL,
    selection_min INTEGER NOT NULL,
    selection_max INTEGER NOT NULL,
    required BOOLEAN NOT NULL,
    exclusive BOOLEAN NOT NULL,
    default_modifier_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
    PRIMARY KEY (snapshot_id, group_id),
    FOREIGN KEY (snapshot_id, menu_item_id)
        REFERENCES recipe.menu_snapshot_items (snapshot_id, menu_item_id)
        ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS recipe.menu_snapshot_modifiers (
    snapshot_id TEXT NOT NULL,
    group_id TEXT NOT NULL,
    modifier_id TEXT NOT NULL,
    name TEXT NOT NULL,
    price_delta_minor BIGINT NOT NULL,
    currency TEXT NOT NULL,
    calories NUMERIC(10,4) NOT NULL,
    protein_grams NUMERIC(10,4) NOT NULL,
    carbs_grams NUMERIC(10,4) NOT NULL,
    fat_grams NUMERIC(10,4) NOT NULL,
    ingredient_usage_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    PRIMARY KEY (snapshot_id, modifier_id),
    FOREIGN KEY (snapshot_id, group_id)
        REFERENCES recipe.menu_snapshot_modifier_groups (snapshot_id, group_id)
        ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS recipe_menu_items_location_idx
    ON recipe.menu_items (location_id, name);

CREATE INDEX IF NOT EXISTS recipe_modifier_groups_menu_item_idx
    ON recipe.modifier_groups (menu_item_id, name);

CREATE INDEX IF NOT EXISTS recipe_modifiers_group_idx
    ON recipe.modifiers (modifier_group_id, name);

CREATE INDEX IF NOT EXISTS recipe_modifier_deltas_location_ingredient_idx
    ON recipe.modifier_ingredient_deltas (location_id, ingredient_id);

CREATE INDEX IF NOT EXISTS recipe_menu_snapshots_location_version_idx
    ON recipe.menu_snapshots (location_id, version DESC);

INSERT INTO recipe.recipes (id, location_id, name, yield_count)
VALUES
    ('rec-chipotle-charlotte-burrito-bowl-core', 'loc-chipotle-charlotte', 'Chipotle Burrito Bowl Core', 1),
    ('rec-chipotle-raleigh-burrito-bowl-core', 'loc-chipotle-raleigh', 'Chipotle Burrito Bowl Core', 1)
ON CONFLICT (id) DO NOTHING;

INSERT INTO recipe.recipe_lines (recipe_id, line_number, location_id, target_type, target_id, quantity, unit, prep_method)
VALUES
    ('rec-chipotle-charlotte-burrito-bowl-core', 1, 'loc-chipotle-charlotte', 'ingredient', 'ing-loc-chipotle-charlotte-cmg-5201', 1, 'each', NULL),
    ('rec-chipotle-raleigh-burrito-bowl-core', 1, 'loc-chipotle-raleigh', 'ingredient', 'ing-loc-chipotle-raleigh-cmg-5201', 1, 'each', NULL)
ON CONFLICT (recipe_id, line_number) DO NOTHING;

INSERT INTO recipe.menu_items (id, location_id, recipe_id, name, description, price_minor, currency)
VALUES
    (
        'mi-chipotle-charlotte-burrito-bowl',
        'loc-chipotle-charlotte',
        'rec-chipotle-charlotte-burrito-bowl-core',
        'Chipotle Burrito Bowl',
        'Build-your-own bowl with a protein, rice, beans, and optional add-ons.',
        845,
        'USD'
    ),
    (
        'mi-chipotle-raleigh-burrito-bowl',
        'loc-chipotle-raleigh',
        'rec-chipotle-raleigh-burrito-bowl-core',
        'Chipotle Burrito Bowl',
        'Build-your-own bowl with a protein, rice, beans, and optional add-ons.',
        845,
        'USD'
    )
ON CONFLICT (id) DO NOTHING;

INSERT INTO recipe.modifier_groups (id, location_id, menu_item_id, name, selection_min, selection_max, required, exclusive, default_modifier_ids)
VALUES
    ('mg-chipotle-charlotte-protein', 'loc-chipotle-charlotte', 'mi-chipotle-charlotte-burrito-bowl', 'Protein', 1, 1, TRUE, TRUE, '["mod-chipotle-charlotte-protein-chicken"]'::jsonb),
    ('mg-chipotle-charlotte-rice', 'loc-chipotle-charlotte', 'mi-chipotle-charlotte-burrito-bowl', 'Rice', 1, 1, TRUE, TRUE, '["mod-chipotle-charlotte-rice-white"]'::jsonb),
    ('mg-chipotle-charlotte-beans', 'loc-chipotle-charlotte', 'mi-chipotle-charlotte-burrito-bowl', 'Beans', 1, 1, TRUE, TRUE, '["mod-chipotle-charlotte-beans-black"]'::jsonb),
    ('mg-chipotle-charlotte-addons', 'loc-chipotle-charlotte', 'mi-chipotle-charlotte-burrito-bowl', 'Add-ons', 0, 2, FALSE, FALSE, '[]'::jsonb),
    ('mg-chipotle-raleigh-protein', 'loc-chipotle-raleigh', 'mi-chipotle-raleigh-burrito-bowl', 'Protein', 1, 1, TRUE, TRUE, '["mod-chipotle-raleigh-protein-chicken"]'::jsonb),
    ('mg-chipotle-raleigh-rice', 'loc-chipotle-raleigh', 'mi-chipotle-raleigh-burrito-bowl', 'Rice', 1, 1, TRUE, TRUE, '["mod-chipotle-raleigh-rice-white"]'::jsonb),
    ('mg-chipotle-raleigh-beans', 'loc-chipotle-raleigh', 'mi-chipotle-raleigh-burrito-bowl', 'Beans', 1, 1, TRUE, TRUE, '["mod-chipotle-raleigh-beans-black"]'::jsonb),
    ('mg-chipotle-raleigh-addons', 'loc-chipotle-raleigh', 'mi-chipotle-raleigh-burrito-bowl', 'Add-ons', 0, 2, FALSE, FALSE, '[]'::jsonb)
ON CONFLICT (id) DO NOTHING;

INSERT INTO recipe.modifiers (id, location_id, modifier_group_id, name, price_delta_minor, currency)
VALUES
    ('mod-chipotle-charlotte-protein-chicken', 'loc-chipotle-charlotte', 'mg-chipotle-charlotte-protein', 'Chicken', 0, 'USD'),
    ('mod-chipotle-charlotte-protein-steak', 'loc-chipotle-charlotte', 'mg-chipotle-charlotte-protein', 'Steak', 200, 'USD'),
    ('mod-chipotle-charlotte-protein-barbacoa', 'loc-chipotle-charlotte', 'mg-chipotle-charlotte-protein', 'Barbacoa', 200, 'USD'),
    ('mod-chipotle-charlotte-protein-carnitas', 'loc-chipotle-charlotte', 'mg-chipotle-charlotte-protein', 'Carnitas', 75, 'USD'),
    ('mod-chipotle-charlotte-protein-sofritas', 'loc-chipotle-charlotte', 'mg-chipotle-charlotte-protein', 'Sofritas', 0, 'USD'),
    ('mod-chipotle-charlotte-rice-white', 'loc-chipotle-charlotte', 'mg-chipotle-charlotte-rice', 'White Rice', 0, 'USD'),
    ('mod-chipotle-charlotte-rice-brown', 'loc-chipotle-charlotte', 'mg-chipotle-charlotte-rice', 'Brown Rice', 0, 'USD'),
    ('mod-chipotle-charlotte-beans-black', 'loc-chipotle-charlotte', 'mg-chipotle-charlotte-beans', 'Black Beans', 0, 'USD'),
    ('mod-chipotle-charlotte-beans-pinto', 'loc-chipotle-charlotte', 'mg-chipotle-charlotte-beans', 'Pinto Beans', 0, 'USD'),
    ('mod-chipotle-charlotte-addon-guac', 'loc-chipotle-charlotte', 'mg-chipotle-charlotte-addons', 'Guacamole', 285, 'USD'),
    ('mod-chipotle-charlotte-addon-fajita-veggies', 'loc-chipotle-charlotte', 'mg-chipotle-charlotte-addons', 'Fajita Veggies', 0, 'USD'),
    ('mod-chipotle-raleigh-protein-chicken', 'loc-chipotle-raleigh', 'mg-chipotle-raleigh-protein', 'Chicken', 0, 'USD'),
    ('mod-chipotle-raleigh-protein-steak', 'loc-chipotle-raleigh', 'mg-chipotle-raleigh-protein', 'Steak', 200, 'USD'),
    ('mod-chipotle-raleigh-protein-barbacoa', 'loc-chipotle-raleigh', 'mg-chipotle-raleigh-protein', 'Barbacoa', 200, 'USD'),
    ('mod-chipotle-raleigh-protein-carnitas', 'loc-chipotle-raleigh', 'mg-chipotle-raleigh-protein', 'Carnitas', 75, 'USD'),
    ('mod-chipotle-raleigh-protein-sofritas', 'loc-chipotle-raleigh', 'mg-chipotle-raleigh-protein', 'Sofritas', 0, 'USD'),
    ('mod-chipotle-raleigh-rice-white', 'loc-chipotle-raleigh', 'mg-chipotle-raleigh-rice', 'White Rice', 0, 'USD'),
    ('mod-chipotle-raleigh-rice-brown', 'loc-chipotle-raleigh', 'mg-chipotle-raleigh-rice', 'Brown Rice', 0, 'USD'),
    ('mod-chipotle-raleigh-beans-black', 'loc-chipotle-raleigh', 'mg-chipotle-raleigh-beans', 'Black Beans', 0, 'USD'),
    ('mod-chipotle-raleigh-beans-pinto', 'loc-chipotle-raleigh', 'mg-chipotle-raleigh-beans', 'Pinto Beans', 0, 'USD'),
    ('mod-chipotle-raleigh-addon-guac', 'loc-chipotle-raleigh', 'mg-chipotle-raleigh-addons', 'Guacamole', 285, 'USD'),
    ('mod-chipotle-raleigh-addon-fajita-veggies', 'loc-chipotle-raleigh', 'mg-chipotle-raleigh-addons', 'Fajita Veggies', 0, 'USD')
ON CONFLICT (id) DO NOTHING;

INSERT INTO recipe.modifier_ingredient_deltas (modifier_id, line_number, location_id, ingredient_id, quantity, unit, prep_method)
VALUES
    ('mod-chipotle-charlotte-protein-chicken', 1, 'loc-chipotle-charlotte', 'ing-loc-chipotle-charlotte-cmg-1', 1, 'each', NULL),
    ('mod-chipotle-charlotte-protein-steak', 1, 'loc-chipotle-charlotte', 'ing-loc-chipotle-charlotte-cmg-2', 1, 'each', NULL),
    ('mod-chipotle-charlotte-protein-barbacoa', 1, 'loc-chipotle-charlotte', 'ing-loc-chipotle-charlotte-cmg-4', 1, 'each', NULL),
    ('mod-chipotle-charlotte-protein-carnitas', 1, 'loc-chipotle-charlotte', 'ing-loc-chipotle-charlotte-cmg-3', 1, 'each', NULL),
    ('mod-chipotle-charlotte-protein-sofritas', 1, 'loc-chipotle-charlotte', 'ing-loc-chipotle-charlotte-cmg-5', 1, 'each', NULL),
    ('mod-chipotle-charlotte-rice-white', 1, 'loc-chipotle-charlotte', 'ing-loc-chipotle-charlotte-cmg-5001', 1, 'each', NULL),
    ('mod-chipotle-charlotte-rice-brown', 1, 'loc-chipotle-charlotte', 'ing-loc-chipotle-charlotte-cmg-5002', 1, 'each', NULL),
    ('mod-chipotle-charlotte-beans-black', 1, 'loc-chipotle-charlotte', 'ing-loc-chipotle-charlotte-cmg-5051', 1, 'each', NULL),
    ('mod-chipotle-charlotte-beans-pinto', 1, 'loc-chipotle-charlotte', 'ing-loc-chipotle-charlotte-cmg-5052', 1, 'each', NULL),
    ('mod-chipotle-charlotte-addon-guac', 1, 'loc-chipotle-charlotte', 'ing-loc-chipotle-charlotte-cmg-5301', 1, 'each', NULL),
    ('mod-chipotle-charlotte-addon-fajita-veggies', 1, 'loc-chipotle-charlotte', 'ing-loc-chipotle-charlotte-cmg-5101', 1, 'each', NULL),
    ('mod-chipotle-raleigh-protein-chicken', 1, 'loc-chipotle-raleigh', 'ing-loc-chipotle-raleigh-cmg-1', 1, 'each', NULL),
    ('mod-chipotle-raleigh-protein-steak', 1, 'loc-chipotle-raleigh', 'ing-loc-chipotle-raleigh-cmg-2', 1, 'each', NULL),
    ('mod-chipotle-raleigh-protein-barbacoa', 1, 'loc-chipotle-raleigh', 'ing-loc-chipotle-raleigh-cmg-4', 1, 'each', NULL),
    ('mod-chipotle-raleigh-protein-carnitas', 1, 'loc-chipotle-raleigh', 'ing-loc-chipotle-raleigh-cmg-3', 1, 'each', NULL),
    ('mod-chipotle-raleigh-protein-sofritas', 1, 'loc-chipotle-raleigh', 'ing-loc-chipotle-raleigh-cmg-5', 1, 'each', NULL),
    ('mod-chipotle-raleigh-rice-white', 1, 'loc-chipotle-raleigh', 'ing-loc-chipotle-raleigh-cmg-5001', 1, 'each', NULL),
    ('mod-chipotle-raleigh-rice-brown', 1, 'loc-chipotle-raleigh', 'ing-loc-chipotle-raleigh-cmg-5002', 1, 'each', NULL),
    ('mod-chipotle-raleigh-beans-black', 1, 'loc-chipotle-raleigh', 'ing-loc-chipotle-raleigh-cmg-5051', 1, 'each', NULL),
    ('mod-chipotle-raleigh-beans-pinto', 1, 'loc-chipotle-raleigh', 'ing-loc-chipotle-raleigh-cmg-5052', 1, 'each', NULL),
    ('mod-chipotle-raleigh-addon-guac', 1, 'loc-chipotle-raleigh', 'ing-loc-chipotle-raleigh-cmg-5301', 1, 'each', NULL),
    ('mod-chipotle-raleigh-addon-fajita-veggies', 1, 'loc-chipotle-raleigh', 'ing-loc-chipotle-raleigh-cmg-5101', 1, 'each', NULL)
ON CONFLICT (modifier_id, line_number) DO NOTHING;
