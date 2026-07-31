ALTER TABLE ingredient.ingredients
    ADD COLUMN IF NOT EXISTS last_received_cost_per_base_unit NUMERIC(12,4),
    ADD COLUMN IF NOT EXISTS last_received_at TIMESTAMPTZ;

CREATE TABLE IF NOT EXISTS purchasing.vendors (
    location_id TEXT NOT NULL REFERENCES tenancy.locations (id),
    id TEXT NOT NULL,
    name TEXT NOT NULL,
    status TEXT NOT NULL,
    external_ref TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (location_id, id),
    UNIQUE (location_id, name),
    CHECK (status IN ('active', 'inactive'))
);

CREATE TABLE IF NOT EXISTS purchasing.vendor_items (
    location_id TEXT NOT NULL REFERENCES tenancy.locations (id),
    id TEXT NOT NULL,
    vendor_id TEXT NOT NULL,
    ingredient_id TEXT NOT NULL,
    vendor_sku TEXT NOT NULL DEFAULT '',
    name TEXT NOT NULL,
    purchase_unit TEXT NOT NULL,
    pack_quantity NUMERIC(12,4) NOT NULL,
    ingredient_base_quantity NUMERIC(12,4) NOT NULL,
    last_unit_cost NUMERIC(12,4) NOT NULL DEFAULT 0,
    currency TEXT NOT NULL,
    status TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (location_id, id),
    FOREIGN KEY (location_id, vendor_id)
        REFERENCES purchasing.vendors (location_id, id)
        ON DELETE RESTRICT,
    FOREIGN KEY (location_id, ingredient_id)
        REFERENCES ingredient.ingredients (location_id, id)
        ON DELETE RESTRICT,
    UNIQUE (location_id, vendor_id, name),
    CHECK (purchase_unit <> ''),
    CHECK (pack_quantity > 0),
    CHECK (ingredient_base_quantity > 0),
    CHECK (last_unit_cost >= 0),
    CHECK (currency <> ''),
    CHECK (status IN ('active', 'inactive'))
);

CREATE TABLE IF NOT EXISTS purchasing.purchase_orders (
    location_id TEXT NOT NULL REFERENCES tenancy.locations (id),
    id TEXT NOT NULL,
    po_number TEXT NOT NULL,
    vendor_id TEXT NOT NULL,
    status TEXT NOT NULL,
    ordered_at TIMESTAMPTZ,
    notes TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (location_id, id),
    UNIQUE (location_id, po_number),
    FOREIGN KEY (location_id, vendor_id)
        REFERENCES purchasing.vendors (location_id, id)
        ON DELETE RESTRICT,
    CHECK (status IN ('draft', 'submitted', 'partially_received', 'received', 'cancelled'))
);

CREATE TABLE IF NOT EXISTS purchasing.purchase_order_lines (
    location_id TEXT NOT NULL REFERENCES tenancy.locations (id),
    id TEXT NOT NULL,
    purchase_order_id TEXT NOT NULL,
    vendor_item_id TEXT NOT NULL,
    ordered_quantity NUMERIC(12,4) NOT NULL,
    ordered_unit_cost NUMERIC(12,4) NOT NULL,
    currency TEXT NOT NULL,
    received_quantity NUMERIC(12,4) NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (location_id, id),
    FOREIGN KEY (location_id, purchase_order_id)
        REFERENCES purchasing.purchase_orders (location_id, id)
        ON DELETE CASCADE,
    FOREIGN KEY (location_id, vendor_item_id)
        REFERENCES purchasing.vendor_items (location_id, id)
        ON DELETE RESTRICT,
    CHECK (ordered_quantity > 0),
    CHECK (ordered_unit_cost >= 0),
    CHECK (currency <> ''),
    CHECK (received_quantity >= 0)
);

CREATE TABLE IF NOT EXISTS purchasing.receipts (
    location_id TEXT NOT NULL REFERENCES tenancy.locations (id),
    id TEXT NOT NULL,
    purchase_order_id TEXT NOT NULL,
    received_at TIMESTAMPTZ NOT NULL,
    received_by TEXT NOT NULL DEFAULT '',
    notes TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (location_id, id),
    FOREIGN KEY (location_id, purchase_order_id)
        REFERENCES purchasing.purchase_orders (location_id, id)
        ON DELETE RESTRICT
);

CREATE TABLE IF NOT EXISTS purchasing.receipt_lines (
    location_id TEXT NOT NULL REFERENCES tenancy.locations (id),
    id TEXT NOT NULL,
    receipt_id TEXT NOT NULL,
    purchase_order_line_id TEXT NOT NULL,
    ingredient_id TEXT NOT NULL,
    received_quantity NUMERIC(12,4) NOT NULL,
    received_unit_cost NUMERIC(12,4) NOT NULL,
    currency TEXT NOT NULL,
    inventory_quantity NUMERIC(12,4) NOT NULL,
    inventory_unit TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (location_id, id),
    FOREIGN KEY (location_id, receipt_id)
        REFERENCES purchasing.receipts (location_id, id)
        ON DELETE CASCADE,
    FOREIGN KEY (location_id, purchase_order_line_id)
        REFERENCES purchasing.purchase_order_lines (location_id, id)
        ON DELETE RESTRICT,
    FOREIGN KEY (location_id, ingredient_id)
        REFERENCES ingredient.ingredients (location_id, id)
        ON DELETE RESTRICT,
    CHECK (received_quantity > 0),
    CHECK (received_unit_cost >= 0),
    CHECK (currency <> ''),
    CHECK (inventory_quantity > 0),
    CHECK (inventory_unit <> '')
);

CREATE INDEX IF NOT EXISTS purchasing_vendor_items_vendor_idx
    ON purchasing.vendor_items (location_id, vendor_id, name, id);

CREATE INDEX IF NOT EXISTS purchasing_purchase_orders_vendor_idx
    ON purchasing.purchase_orders (location_id, vendor_id, status, created_at, id);

CREATE INDEX IF NOT EXISTS purchasing_receipts_po_idx
    ON purchasing.receipts (location_id, purchase_order_id, received_at, id);
