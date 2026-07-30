ALTER TABLE diner.receipt_tokens
    ADD CONSTRAINT diner_receipt_tokens_location_token_uniq UNIQUE (location_id, token);

ALTER TABLE diner.receipt_token_items
    ADD CONSTRAINT diner_receipt_token_items_location_token_item_uniq UNIQUE (location_id, token, item_id);

ALTER TABLE diner.claims
    ADD CONSTRAINT diner_claims_location_id_uniq UNIQUE (location_id, id);

ALTER TABLE diner.receipt_tokens
    ADD CONSTRAINT diner_receipt_tokens_location_fk
        FOREIGN KEY (location_id)
        REFERENCES tenancy.locations (id);

ALTER TABLE diner.receipt_token_items
    ADD CONSTRAINT diner_receipt_token_items_location_fk
        FOREIGN KEY (location_id)
        REFERENCES tenancy.locations (id);

ALTER TABLE diner.receipt_token_items
    ADD CONSTRAINT diner_receipt_token_items_location_token_fk
        FOREIGN KEY (location_id, token)
        REFERENCES diner.receipt_tokens (location_id, token)
        ON DELETE CASCADE;

ALTER TABLE diner.claims
    ADD CONSTRAINT diner_claims_location_fk
        FOREIGN KEY (location_id)
        REFERENCES tenancy.locations (id);

ALTER TABLE diner.claims
    ADD CONSTRAINT diner_claims_location_token_fk
        FOREIGN KEY (location_id, token)
        REFERENCES diner.receipt_tokens (location_id, token)
        ON DELETE CASCADE;

ALTER TABLE diner.claim_items
    DROP CONSTRAINT IF EXISTS diner_claim_items_token_item_fk;

ALTER TABLE diner.claim_items
    ADD CONSTRAINT diner_claim_items_location_fk
        FOREIGN KEY (location_id)
        REFERENCES tenancy.locations (id);

ALTER TABLE diner.claim_items
    ADD CONSTRAINT diner_claim_items_location_claim_fk
        FOREIGN KEY (location_id, claim_id)
        REFERENCES diner.claims (location_id, id)
        ON DELETE CASCADE;

ALTER TABLE diner.claim_items
    ADD CONSTRAINT diner_claim_items_location_token_item_fk
        FOREIGN KEY (location_id, token, item_id)
        REFERENCES diner.receipt_token_items (location_id, token, item_id)
        ON DELETE CASCADE;
