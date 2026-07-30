CREATE TABLE IF NOT EXISTS platform.outbox_event_deliveries (
    event_id TEXT NOT NULL REFERENCES platform.outbox_events(id) ON DELETE CASCADE,
    consumer_name TEXT NOT NULL,
    delivered_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (event_id, consumer_name)
);

CREATE INDEX IF NOT EXISTS outbox_event_deliveries_consumer_idx
    ON platform.outbox_event_deliveries (consumer_name, delivered_at, event_id);

CREATE TABLE IF NOT EXISTS diner.receipt_tokens (
    token TEXT PRIMARY KEY,
    order_id TEXT NOT NULL,
    location_id TEXT NOT NULL,
    closed_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT diner_receipt_tokens_location_order_uniq UNIQUE (location_id, order_id)
);

CREATE INDEX IF NOT EXISTS diner_receipt_tokens_expires_idx
    ON diner.receipt_tokens (expires_at);

CREATE TABLE IF NOT EXISTS diner.receipt_token_items (
    location_id TEXT NOT NULL,
    token TEXT NOT NULL REFERENCES diner.receipt_tokens(token) ON DELETE CASCADE,
    item_id TEXT NOT NULL,
    line_id TEXT NOT NULL,
    ordinal INTEGER NOT NULL,
    name TEXT NOT NULL,
    calories DOUBLE PRECISION NOT NULL,
    protein_grams DOUBLE PRECISION NOT NULL,
    carbs_grams DOUBLE PRECISION NOT NULL,
    fat_grams DOUBLE PRECISION NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (token, item_id),
    CONSTRAINT diner_receipt_token_items_token_ordinal_uniq UNIQUE (token, ordinal)
);

CREATE INDEX IF NOT EXISTS diner_receipt_token_items_location_idx
    ON diner.receipt_token_items (location_id, token, ordinal);

CREATE TABLE IF NOT EXISTS diner.claims (
    id TEXT PRIMARY KEY,
    token TEXT NOT NULL REFERENCES diner.receipt_tokens(token) ON DELETE CASCADE,
    location_id TEXT NOT NULL,
    total_calories DOUBLE PRECISION NOT NULL,
    total_protein_grams DOUBLE PRECISION NOT NULL,
    total_carbs_grams DOUBLE PRECISION NOT NULL,
    total_fat_grams DOUBLE PRECISION NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS diner_claims_token_idx
    ON diner.claims (token, updated_at);

CREATE TABLE IF NOT EXISTS diner.claim_items (
    location_id TEXT NOT NULL,
    claim_id TEXT NOT NULL REFERENCES diner.claims(id) ON DELETE CASCADE,
    token TEXT NOT NULL,
    item_id TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (claim_id, item_id),
    CONSTRAINT diner_claim_items_token_item_fk
        FOREIGN KEY (token, item_id)
        REFERENCES diner.receipt_token_items(token, item_id)
        ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS diner_claim_items_location_idx
    ON diner.claim_items (location_id, claim_id, created_at);
