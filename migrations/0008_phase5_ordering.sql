CREATE TABLE IF NOT EXISTS ordering.orders (
    id TEXT NOT NULL,
    location_id TEXT NOT NULL REFERENCES tenancy.locations (id),
    check_id TEXT NOT NULL,
    snapshot_id TEXT NOT NULL,
    snapshot_version INTEGER NOT NULL,
    business_date TEXT NOT NULL,
    status TEXT NOT NULL,
    closed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (location_id, id),
    UNIQUE (location_id, check_id),
    CHECK (snapshot_version > 0),
    CHECK (status IN ('open', 'closing', 'closed')),
    CHECK (business_date ~ '^[0-9]{4}-[0-9]{2}-[0-9]{2}$'),
    CHECK ((status = 'closed' AND closed_at IS NOT NULL) OR (status <> 'closed' AND closed_at IS NULL))
);

CREATE TABLE IF NOT EXISTS ordering.checks (
    id TEXT NOT NULL,
    location_id TEXT NOT NULL REFERENCES tenancy.locations (id),
    order_id TEXT NOT NULL,
    status TEXT NOT NULL,
    total_minor BIGINT NOT NULL DEFAULT 0,
    currency TEXT NOT NULL DEFAULT '',
    closed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (location_id, id),
    UNIQUE (location_id, order_id),
    FOREIGN KEY (location_id, order_id)
        REFERENCES ordering.orders (location_id, id)
        ON DELETE CASCADE,
    CHECK (status IN ('open', 'closing', 'closed')),
    CHECK (total_minor >= 0),
    CHECK ((status = 'closed' AND closed_at IS NOT NULL) OR (status <> 'closed' AND closed_at IS NULL))
);

CREATE TABLE IF NOT EXISTS ordering.order_lines (
    location_id TEXT NOT NULL REFERENCES tenancy.locations (id),
    order_id TEXT NOT NULL,
    line_id TEXT NOT NULL,
    menu_item_id TEXT NOT NULL,
    name TEXT NOT NULL,
    quantity INTEGER NOT NULL,
    resolved_price_minor BIGINT NOT NULL,
    currency TEXT NOT NULL,
    resolved_calories NUMERIC(10,4) NOT NULL,
    resolved_protein_grams NUMERIC(10,4) NOT NULL,
    resolved_carbs_grams NUMERIC(10,4) NOT NULL,
    resolved_fat_grams NUMERIC(10,4) NOT NULL,
    ingredient_usage_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (location_id, order_id, line_id),
    FOREIGN KEY (location_id, order_id)
        REFERENCES ordering.orders (location_id, id)
        ON DELETE CASCADE,
    CHECK (quantity > 0),
    CHECK (resolved_price_minor >= 0),
    CHECK (resolved_calories >= 0),
    CHECK (resolved_protein_grams >= 0),
    CHECK (resolved_carbs_grams >= 0),
    CHECK (resolved_fat_grams >= 0)
);

CREATE TABLE IF NOT EXISTS ordering.order_line_modifiers (
    location_id TEXT NOT NULL REFERENCES tenancy.locations (id),
    order_id TEXT NOT NULL,
    line_id TEXT NOT NULL,
    modifier_id TEXT NOT NULL,
    name TEXT NOT NULL,
    price_delta_minor BIGINT NOT NULL,
    currency TEXT NOT NULL,
    macro_delta_calories NUMERIC(10,4) NOT NULL,
    macro_delta_protein_grams NUMERIC(10,4) NOT NULL,
    macro_delta_carbs_grams NUMERIC(10,4) NOT NULL,
    macro_delta_fat_grams NUMERIC(10,4) NOT NULL,
    ingredient_usage_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (location_id, order_id, line_id, modifier_id),
    FOREIGN KEY (location_id, order_id, line_id)
        REFERENCES ordering.order_lines (location_id, order_id, line_id)
        ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS ordering.tenders (
    id TEXT NOT NULL,
    location_id TEXT NOT NULL REFERENCES tenancy.locations (id),
    check_id TEXT NOT NULL,
    amount_minor BIGINT NOT NULL,
    currency TEXT NOT NULL,
    kind TEXT NOT NULL,
    status TEXT NOT NULL,
    provider_reference TEXT NOT NULL DEFAULT '',
    processed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (location_id, id),
    FOREIGN KEY (location_id, check_id)
        REFERENCES ordering.checks (location_id, id)
        ON DELETE CASCADE,
    CHECK (amount_minor >= 0),
    CHECK (status IN ('pending', 'succeeded', 'failed'))
);

CREATE INDEX IF NOT EXISTS ordering_orders_location_status_idx
    ON ordering.orders (location_id, status, business_date);

CREATE INDEX IF NOT EXISTS ordering_order_lines_location_order_idx
    ON ordering.order_lines (location_id, order_id);

CREATE INDEX IF NOT EXISTS ordering_tenders_location_check_idx
    ON ordering.tenders (location_id, check_id, created_at);

INSERT INTO recipe.menu_snapshots (id, location_id, version)
VALUES ('snap-chipotle-charlotte-menu-v1', 'loc-chipotle-charlotte', 1)
ON CONFLICT DO NOTHING;

INSERT INTO recipe.menu_snapshots (id, location_id, version)
VALUES ('snap-chipotle-raleigh-menu-v1', 'loc-chipotle-raleigh', 1)
ON CONFLICT DO NOTHING;

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
SELECT
    seed.snapshot_id,
    seed.menu_item_id,
    seed.name,
    seed.description,
    seed.price_minor,
    seed.currency,
    seed.calories,
    seed.protein_grams,
    seed.carbs_grams,
    seed.fat_grams,
    seed.ingredient_usage_json
FROM (
    VALUES
        (
            'snap-chipotle-charlotte-menu-v1',
            'mi-chipotle-charlotte-burrito-bowl',
            'Chipotle Burrito Bowl',
            'Build-your-own bowl with a protein, rice, beans, and optional add-ons.',
            845,
            'USD',
            25.0000,
            0.0000,
            4.0000,
            0.0000,
            '{"ing-loc-chipotle-charlotte-cmg-5201":1}'::jsonb
        ),
        (
            'snap-chipotle-raleigh-menu-v1',
            'mi-chipotle-raleigh-burrito-bowl',
            'Chipotle Burrito Bowl',
            'Build-your-own bowl with a protein, rice, beans, and optional add-ons.',
            845,
            'USD',
            25.0000,
            0.0000,
            4.0000,
            0.0000,
            '{"ing-loc-chipotle-raleigh-cmg-5201":1}'::jsonb
        )
) AS seed (
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
ON CONFLICT (snapshot_id, menu_item_id) DO NOTHING;

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
SELECT
    seed.snapshot_id,
    seed.menu_item_id,
    seed.group_id,
    seed.name,
    seed.selection_min,
    seed.selection_max,
    seed.required,
    seed.exclusive,
    seed.default_modifier_ids
FROM (
    VALUES
        ('snap-chipotle-charlotte-menu-v1', 'mi-chipotle-charlotte-burrito-bowl', 'mg-chipotle-charlotte-protein', 'Protein', 1, 1, TRUE, TRUE, '["mod-chipotle-charlotte-protein-chicken"]'::jsonb),
        ('snap-chipotle-charlotte-menu-v1', 'mi-chipotle-charlotte-burrito-bowl', 'mg-chipotle-charlotte-rice', 'Rice', 1, 1, TRUE, TRUE, '["mod-chipotle-charlotte-rice-white"]'::jsonb),
        ('snap-chipotle-charlotte-menu-v1', 'mi-chipotle-charlotte-burrito-bowl', 'mg-chipotle-charlotte-beans', 'Beans', 1, 1, TRUE, TRUE, '["mod-chipotle-charlotte-beans-black"]'::jsonb),
        ('snap-chipotle-charlotte-menu-v1', 'mi-chipotle-charlotte-burrito-bowl', 'mg-chipotle-charlotte-addons', 'Add-ons', 0, 2, FALSE, FALSE, '[]'::jsonb),
        ('snap-chipotle-raleigh-menu-v1', 'mi-chipotle-raleigh-burrito-bowl', 'mg-chipotle-raleigh-protein', 'Protein', 1, 1, TRUE, TRUE, '["mod-chipotle-raleigh-protein-chicken"]'::jsonb),
        ('snap-chipotle-raleigh-menu-v1', 'mi-chipotle-raleigh-burrito-bowl', 'mg-chipotle-raleigh-rice', 'Rice', 1, 1, TRUE, TRUE, '["mod-chipotle-raleigh-rice-white"]'::jsonb),
        ('snap-chipotle-raleigh-menu-v1', 'mi-chipotle-raleigh-burrito-bowl', 'mg-chipotle-raleigh-beans', 'Beans', 1, 1, TRUE, TRUE, '["mod-chipotle-raleigh-beans-black"]'::jsonb),
        ('snap-chipotle-raleigh-menu-v1', 'mi-chipotle-raleigh-burrito-bowl', 'mg-chipotle-raleigh-addons', 'Add-ons', 0, 2, FALSE, FALSE, '[]'::jsonb)
) AS seed (
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
ON CONFLICT (snapshot_id, group_id) DO NOTHING;

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
SELECT
    seed.snapshot_id,
    seed.group_id,
    seed.modifier_id,
    seed.name,
    seed.price_delta_minor,
    seed.currency,
    seed.calories,
    seed.protein_grams,
    seed.carbs_grams,
    seed.fat_grams,
    seed.ingredient_usage_json
FROM (
    VALUES
        ('snap-chipotle-charlotte-menu-v1', 'mg-chipotle-charlotte-protein', 'mod-chipotle-charlotte-protein-chicken', 'Chicken', 0, 'USD', 180.0000, 32.0000, 0.0000, 7.0000, '{"ing-loc-chipotle-charlotte-cmg-1":1}'::jsonb),
        ('snap-chipotle-charlotte-menu-v1', 'mg-chipotle-charlotte-protein', 'mod-chipotle-charlotte-protein-steak', 'Steak', 200, 'USD', 150.0000, 21.0000, 1.0000, 6.0000, '{"ing-loc-chipotle-charlotte-cmg-2":1}'::jsonb),
        ('snap-chipotle-charlotte-menu-v1', 'mg-chipotle-charlotte-protein', 'mod-chipotle-charlotte-protein-barbacoa', 'Barbacoa', 200, 'USD', 170.0000, 24.0000, 2.0000, 7.0000, '{"ing-loc-chipotle-charlotte-cmg-4":1}'::jsonb),
        ('snap-chipotle-charlotte-menu-v1', 'mg-chipotle-charlotte-protein', 'mod-chipotle-charlotte-protein-carnitas', 'Carnitas', 75, 'USD', 210.0000, 23.0000, 0.0000, 12.0000, '{"ing-loc-chipotle-charlotte-cmg-3":1}'::jsonb),
        ('snap-chipotle-charlotte-menu-v1', 'mg-chipotle-charlotte-protein', 'mod-chipotle-charlotte-protein-sofritas', 'Sofritas', 0, 'USD', 150.0000, 8.0000, 9.0000, 10.0000, '{"ing-loc-chipotle-charlotte-cmg-5":1}'::jsonb),
        ('snap-chipotle-charlotte-menu-v1', 'mg-chipotle-charlotte-rice', 'mod-chipotle-charlotte-rice-white', 'White Rice', 0, 'USD', 210.0000, 4.0000, 40.0000, 4.0000, '{"ing-loc-chipotle-charlotte-cmg-5001":1}'::jsonb),
        ('snap-chipotle-charlotte-menu-v1', 'mg-chipotle-charlotte-rice', 'mod-chipotle-charlotte-rice-brown', 'Brown Rice', 0, 'USD', 210.0000, 4.0000, 36.0000, 6.0000, '{"ing-loc-chipotle-charlotte-cmg-5002":1}'::jsonb),
        ('snap-chipotle-charlotte-menu-v1', 'mg-chipotle-charlotte-beans', 'mod-chipotle-charlotte-beans-black', 'Black Beans', 0, 'USD', 130.0000, 8.0000, 22.0000, 1.5000, '{"ing-loc-chipotle-charlotte-cmg-5051":1}'::jsonb),
        ('snap-chipotle-charlotte-menu-v1', 'mg-chipotle-charlotte-beans', 'mod-chipotle-charlotte-beans-pinto', 'Pinto Beans', 0, 'USD', 130.0000, 8.0000, 21.0000, 1.5000, '{"ing-loc-chipotle-charlotte-cmg-5052":1}'::jsonb),
        ('snap-chipotle-charlotte-menu-v1', 'mg-chipotle-charlotte-addons', 'mod-chipotle-charlotte-addon-guac', 'Guacamole', 285, 'USD', 230.0000, 2.0000, 8.0000, 22.0000, '{"ing-loc-chipotle-charlotte-cmg-5301":1}'::jsonb),
        ('snap-chipotle-charlotte-menu-v1', 'mg-chipotle-charlotte-addons', 'mod-chipotle-charlotte-addon-fajita-veggies', 'Fajita Veggies', 0, 'USD', 20.0000, 1.0000, 5.0000, 0.0000, '{"ing-loc-chipotle-charlotte-cmg-5101":1}'::jsonb),
        ('snap-chipotle-raleigh-menu-v1', 'mg-chipotle-raleigh-protein', 'mod-chipotle-raleigh-protein-chicken', 'Chicken', 0, 'USD', 180.0000, 32.0000, 0.0000, 7.0000, '{"ing-loc-chipotle-raleigh-cmg-1":1}'::jsonb),
        ('snap-chipotle-raleigh-menu-v1', 'mg-chipotle-raleigh-protein', 'mod-chipotle-raleigh-protein-steak', 'Steak', 200, 'USD', 150.0000, 21.0000, 1.0000, 6.0000, '{"ing-loc-chipotle-raleigh-cmg-2":1}'::jsonb),
        ('snap-chipotle-raleigh-menu-v1', 'mg-chipotle-raleigh-protein', 'mod-chipotle-raleigh-protein-barbacoa', 'Barbacoa', 200, 'USD', 170.0000, 24.0000, 2.0000, 7.0000, '{"ing-loc-chipotle-raleigh-cmg-4":1}'::jsonb),
        ('snap-chipotle-raleigh-menu-v1', 'mg-chipotle-raleigh-protein', 'mod-chipotle-raleigh-protein-carnitas', 'Carnitas', 75, 'USD', 210.0000, 23.0000, 0.0000, 12.0000, '{"ing-loc-chipotle-raleigh-cmg-3":1}'::jsonb),
        ('snap-chipotle-raleigh-menu-v1', 'mg-chipotle-raleigh-protein', 'mod-chipotle-raleigh-protein-sofritas', 'Sofritas', 0, 'USD', 150.0000, 8.0000, 9.0000, 10.0000, '{"ing-loc-chipotle-raleigh-cmg-5":1}'::jsonb),
        ('snap-chipotle-raleigh-menu-v1', 'mg-chipotle-raleigh-rice', 'mod-chipotle-raleigh-rice-white', 'White Rice', 0, 'USD', 210.0000, 4.0000, 40.0000, 4.0000, '{"ing-loc-chipotle-raleigh-cmg-5001":1}'::jsonb),
        ('snap-chipotle-raleigh-menu-v1', 'mg-chipotle-raleigh-rice', 'mod-chipotle-raleigh-rice-brown', 'Brown Rice', 0, 'USD', 210.0000, 4.0000, 36.0000, 6.0000, '{"ing-loc-chipotle-raleigh-cmg-5002":1}'::jsonb),
        ('snap-chipotle-raleigh-menu-v1', 'mg-chipotle-raleigh-beans', 'mod-chipotle-raleigh-beans-black', 'Black Beans', 0, 'USD', 130.0000, 8.0000, 22.0000, 1.5000, '{"ing-loc-chipotle-raleigh-cmg-5051":1}'::jsonb),
        ('snap-chipotle-raleigh-menu-v1', 'mg-chipotle-raleigh-beans', 'mod-chipotle-raleigh-beans-pinto', 'Pinto Beans', 0, 'USD', 130.0000, 8.0000, 21.0000, 1.5000, '{"ing-loc-chipotle-raleigh-cmg-5052":1}'::jsonb),
        ('snap-chipotle-raleigh-menu-v1', 'mg-chipotle-raleigh-addons', 'mod-chipotle-raleigh-addon-guac', 'Guacamole', 285, 'USD', 230.0000, 2.0000, 8.0000, 22.0000, '{"ing-loc-chipotle-raleigh-cmg-5301":1}'::jsonb),
        ('snap-chipotle-raleigh-menu-v1', 'mg-chipotle-raleigh-addons', 'mod-chipotle-raleigh-addon-fajita-veggies', 'Fajita Veggies', 0, 'USD', 20.0000, 1.0000, 5.0000, 0.0000, '{"ing-loc-chipotle-raleigh-cmg-5101":1}'::jsonb)
) AS seed (
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
ON CONFLICT (snapshot_id, modifier_id) DO NOTHING;
