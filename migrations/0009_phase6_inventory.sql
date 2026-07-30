CREATE TABLE IF NOT EXISTS inventory.inventory_movements (
    id TEXT NOT NULL,
    location_id TEXT NOT NULL REFERENCES tenancy.locations (id),
    ingredient_id TEXT NOT NULL,
    movement_type TEXT NOT NULL,
    source_type TEXT NOT NULL,
    source_id TEXT NOT NULL,
    quantity_base_units NUMERIC(12,4) NOT NULL,
    unit TEXT NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (location_id, id),
    UNIQUE (location_id, source_type, source_id, ingredient_id),
    FOREIGN KEY (location_id, ingredient_id)
        REFERENCES ingredient.ingredients (location_id, id)
        ON DELETE RESTRICT,
    CHECK (quantity_base_units <> 0),
    CHECK (unit <> ''),
    CHECK (source_type <> ''),
    CHECK (source_id <> '')
);

CREATE INDEX IF NOT EXISTS inventory_movements_location_occurred_idx
    ON inventory.inventory_movements (location_id, occurred_at, created_at, id);

CREATE TABLE IF NOT EXISTS inventory.inventory_counts (
    id TEXT NOT NULL,
    location_id TEXT NOT NULL REFERENCES tenancy.locations (id),
    counted_at TIMESTAMPTZ NOT NULL,
    notes TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (location_id, id)
);

CREATE TABLE IF NOT EXISTS inventory.inventory_count_lines (
    location_id TEXT NOT NULL,
    count_id TEXT NOT NULL,
    ingredient_id TEXT NOT NULL,
    counted_quantity_base_units NUMERIC(12,4) NOT NULL,
    unit TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (location_id, count_id, ingredient_id),
    FOREIGN KEY (location_id, count_id)
        REFERENCES inventory.inventory_counts (location_id, id)
        ON DELETE CASCADE,
    FOREIGN KEY (location_id, ingredient_id)
        REFERENCES ingredient.ingredients (location_id, id)
        ON DELETE RESTRICT,
    CHECK (unit <> '')
);
