ALTER TABLE identity.users
    ADD COLUMN IF NOT EXISTS organization_id TEXT,
    ADD COLUMN IF NOT EXISTS scope_type TEXT;

ALTER TABLE identity.users
    ALTER COLUMN location_id DROP NOT NULL;

UPDATE identity.users AS users
SET organization_id = restaurants.organization_id,
    scope_type = COALESCE(users.scope_type, 'location')
FROM tenancy.locations AS locations
JOIN tenancy.restaurants AS restaurants
    ON restaurants.id = locations.restaurant_id
WHERE users.location_id = locations.id
  AND users.organization_id IS NULL;

ALTER TABLE identity.users
    ALTER COLUMN organization_id SET NOT NULL,
    ALTER COLUMN scope_type SET NOT NULL;

ALTER TABLE identity.users
    DROP CONSTRAINT IF EXISTS identity_users_scope_check;

ALTER TABLE identity.users
    ADD CONSTRAINT identity_users_scope_check
    CHECK (
        (scope_type = 'organization' AND location_id IS NULL)
        OR (scope_type = 'location' AND location_id IS NOT NULL)
    );

ALTER TABLE identity.roles
    ADD COLUMN IF NOT EXISTS organization_id TEXT,
    ADD COLUMN IF NOT EXISTS scope_type TEXT;

ALTER TABLE identity.roles
    ALTER COLUMN location_id DROP NOT NULL;

UPDATE identity.roles AS roles
SET organization_id = restaurants.organization_id,
    scope_type = COALESCE(roles.scope_type, 'location')
FROM tenancy.locations AS locations
JOIN tenancy.restaurants AS restaurants
    ON restaurants.id = locations.restaurant_id
WHERE roles.location_id = locations.id
  AND roles.organization_id IS NULL;

ALTER TABLE identity.roles
    ALTER COLUMN organization_id SET NOT NULL,
    ALTER COLUMN scope_type SET NOT NULL;

ALTER TABLE identity.roles
    DROP CONSTRAINT IF EXISTS identity_roles_scope_check;

ALTER TABLE identity.roles
    ADD CONSTRAINT identity_roles_scope_check
    CHECK (
        (scope_type = 'organization' AND location_id IS NULL)
        OR (scope_type = 'location' AND location_id IS NOT NULL)
    );

ALTER TABLE identity.sessions
    ADD COLUMN IF NOT EXISTS organization_id TEXT,
    ADD COLUMN IF NOT EXISTS scope_type TEXT;

ALTER TABLE identity.sessions
    ALTER COLUMN location_id DROP NOT NULL;

UPDATE identity.sessions AS sessions
SET organization_id = restaurants.organization_id,
    scope_type = COALESCE(sessions.scope_type, 'location')
FROM tenancy.locations AS locations
JOIN tenancy.restaurants AS restaurants
    ON restaurants.id = locations.restaurant_id
WHERE sessions.location_id = locations.id
  AND sessions.organization_id IS NULL;

ALTER TABLE identity.sessions
    ALTER COLUMN organization_id SET NOT NULL,
    ALTER COLUMN scope_type SET NOT NULL;

ALTER TABLE identity.sessions
    DROP CONSTRAINT IF EXISTS identity_sessions_scope_check;

ALTER TABLE identity.sessions
    ADD CONSTRAINT identity_sessions_scope_check
    CHECK (
        (scope_type = 'organization' AND location_id IS NULL)
        OR (scope_type = 'location' AND location_id IS NOT NULL)
    );

CREATE TABLE IF NOT EXISTS ingredient.ingredients (
    id TEXT PRIMARY KEY,
    location_id TEXT NOT NULL REFERENCES tenancy.locations (id),
    source_item_id TEXT NOT NULL,
    name TEXT NOT NULL,
    category TEXT NOT NULL,
    base_unit TEXT NOT NULL,
    calories_per_base_unit NUMERIC(10,2) NOT NULL,
    protein_grams_per_base_unit NUMERIC(10,2) NOT NULL,
    carbs_grams_per_base_unit NUMERIC(10,2) NOT NULL,
    fat_grams_per_base_unit NUMERIC(10,2) NOT NULL,
    current_cost_minor BIGINT NOT NULL DEFAULT 0,
    currency TEXT NOT NULL,
    on_hand_base_units NUMERIC(10,2) NOT NULL DEFAULT 0,
    par_level_base_units NUMERIC(10,2) NOT NULL DEFAULT 0,
    provenance TEXT NOT NULL,
    verification_status TEXT NOT NULL,
    serving_size_quantity NUMERIC(10,2) NOT NULL,
    serving_size_unit TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (location_id, source_item_id),
    UNIQUE (location_id, name),
    CHECK (calories_per_base_unit >= 0),
    CHECK (protein_grams_per_base_unit >= 0),
    CHECK (carbs_grams_per_base_unit >= 0),
    CHECK (fat_grams_per_base_unit >= 0),
    CHECK (current_cost_minor >= 0),
    CHECK (on_hand_base_units >= 0),
    CHECK (par_level_base_units >= 0),
    CHECK (serving_size_quantity > 0)
);

CREATE TABLE IF NOT EXISTS ingredient.ingredient_units (
    id TEXT PRIMARY KEY,
    ingredient_id TEXT NOT NULL REFERENCES ingredient.ingredients (id) ON DELETE CASCADE,
    unit_name TEXT NOT NULL,
    to_base_unit_factor NUMERIC(10,4) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (ingredient_id, unit_name),
    CHECK (to_base_unit_factor > 0)
);

CREATE TABLE IF NOT EXISTS ingredient.ingredient_yield_factors (
    id TEXT PRIMARY KEY,
    ingredient_id TEXT NOT NULL REFERENCES ingredient.ingredients (id) ON DELETE CASCADE,
    prep_method TEXT NOT NULL,
    yield_factor NUMERIC(10,4) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (ingredient_id, prep_method),
    CHECK (yield_factor > 0)
);

INSERT INTO tenancy.organizations (id, name)
VALUES ('org-chipotle', 'Chipotle Corporate')
ON CONFLICT (id) DO NOTHING;

INSERT INTO tenancy.restaurants (id, organization_id, name)
VALUES ('rest-chipotle', 'org-chipotle', 'Chipotle')
ON CONFLICT (id) DO NOTHING;

INSERT INTO tenancy.locations (id, restaurant_id, name, timezone_name, currency)
VALUES
    ('loc-chipotle-charlotte', 'rest-chipotle', 'Chipotle Charlotte', 'America/New_York', 'USD'),
    ('loc-chipotle-raleigh', 'rest-chipotle', 'Chipotle Raleigh', 'America/New_York', 'USD')
ON CONFLICT (id) DO NOTHING;

INSERT INTO identity.roles (id, organization_id, location_id, scope_type, name)
VALUES
    ('role-chipotle-hq-ops', 'org-chipotle', NULL, 'organization', 'hq_ops'),
    ('role-chipotle-charlotte-manager', 'org-chipotle', 'loc-chipotle-charlotte', 'location', 'store_manager'),
    ('role-chipotle-raleigh-manager', 'org-chipotle', 'loc-chipotle-raleigh', 'location', 'store_manager')
ON CONFLICT (id) DO NOTHING;

INSERT INTO identity.users (id, organization_id, location_id, scope_type, email, display_name, password_hash)
VALUES
    ('user-chipotle-hq-ops', 'org-chipotle', NULL, 'organization', 'hq.ops@chipotle.circle.local', 'Chipotle HQ Operations', 'seed-password-hash-change-me'),
    ('user-chipotle-charlotte-manager', 'org-chipotle', 'loc-chipotle-charlotte', 'location', 'charlotte.manager@chipotle.circle.local', 'Chipotle Charlotte Manager', 'seed-password-hash-change-me'),
    ('user-chipotle-raleigh-manager', 'org-chipotle', 'loc-chipotle-raleigh', 'location', 'raleigh.manager@chipotle.circle.local', 'Chipotle Raleigh Manager', 'seed-password-hash-change-me')
ON CONFLICT (id) DO NOTHING;

INSERT INTO identity.user_roles (user_id, role_id)
VALUES
    ('user-chipotle-hq-ops', 'role-chipotle-hq-ops'),
    ('user-chipotle-charlotte-manager', 'role-chipotle-charlotte-manager'),
    ('user-chipotle-raleigh-manager', 'role-chipotle-raleigh-manager')
ON CONFLICT (user_id, role_id) DO NOTHING;

INSERT INTO identity.sessions (id, user_id, organization_id, location_id, scope_type, expires_at)
VALUES
    ('session-chipotle-hq-dev', 'user-chipotle-hq-ops', 'org-chipotle', NULL, 'organization', NOW() + INTERVAL '365 days'),
    ('session-chipotle-charlotte-dev', 'user-chipotle-charlotte-manager', 'org-chipotle', 'loc-chipotle-charlotte', 'location', NOW() + INTERVAL '365 days'),
    ('session-chipotle-raleigh-dev', 'user-chipotle-raleigh-manager', 'org-chipotle', 'loc-chipotle-raleigh', 'location', NOW() + INTERVAL '365 days')
ON CONFLICT (id) DO NOTHING;

WITH chipotle_seed(source_item_id, name, category, calories, protein, carbs, fat, serving_size_quantity, serving_size_unit) AS (
    VALUES
        ('CMG-15', 'Chipotle Honey Chicken', 'protein', 210.00, 21.00, 13.00, 8.00, 4.00, 'oz'),
        ('CMG-1', 'Chicken', 'protein', 180.00, 32.00, 0.00, 7.00, 4.00, 'oz'),
        ('CMG-2', 'Steak', 'protein', 150.00, 21.00, 1.00, 6.00, 4.00, 'oz'),
        ('CMG-3', 'Carnitas', 'protein', 210.00, 23.00, 0.00, 12.00, 4.00, 'oz'),
        ('CMG-4', 'Barbacoa', 'protein', 170.00, 24.00, 2.00, 7.00, 4.00, 'oz'),
        ('CMG-5', 'Sofritas', 'protein', 150.00, 8.00, 9.00, 10.00, 4.00, 'oz'),
        ('CMG-5001', 'White Rice', 'grain', 210.00, 4.00, 40.00, 4.00, 4.00, 'oz'),
        ('CMG-5002', 'Brown Rice', 'grain', 210.00, 4.00, 36.00, 6.00, 4.00, 'oz'),
        ('CMG-5051', 'Black Beans', 'beans', 130.00, 8.00, 22.00, 1.50, 4.00, 'oz'),
        ('CMG-5052', 'Pinto Beans', 'beans', 130.00, 8.00, 21.00, 1.50, 4.00, 'oz'),
        ('CMG-5101', 'Fajita Veggies', 'vegetable', 20.00, 1.00, 5.00, 0.00, 2.50, 'oz'),
        ('CMG-5201', 'Fresh Tomato Salsa', 'salsa', 25.00, 0.00, 4.00, 0.00, 3.50, 'oz'),
        ('CMG-5202', 'Roasted Chili-Corn Salsa', 'salsa', 80.00, 3.00, 16.00, 1.50, 3.50, 'oz'),
        ('CMG-5203', 'Tomatillo-Green Chili Salsa', 'salsa', 15.00, 0.00, 4.00, 0.00, 2.00, 'oz'),
        ('CMG-5204', 'Tomatillo-Red Chili Salsa', 'salsa', 30.00, 0.00, 4.00, 0.00, 2.00, 'oz'),
        ('CMG-5251', 'Sour Cream', 'dairy', 110.00, 2.00, 2.00, 9.00, 2.00, 'oz'),
        ('CMG-5252', 'Monterey Jack Cheese', 'dairy', 110.00, 6.00, 1.00, 8.00, 1.00, 'oz'),
        ('CMG-5301', 'Guacamole', 'topping', 230.00, 2.00, 8.00, 22.00, 4.00, 'oz'),
        ('CMG-5351', 'Romaine Lettuce', 'vegetable', 5.00, 0.00, 1.00, 0.00, 1.00, 'oz'),
        ('CMG-5353', 'Chipotle-Honey Vinaigrette', 'dressing', 220.00, 1.00, 18.00, 16.00, 2.00, 'oz'),
        ('CMG-5355', 'Adobo Ranch', 'dressing', 110.00, 2.00, 4.00, 9.00, 2.00, 'oz'),
        ('CMG-5410', 'Red Chimichurri', 'sauce', 190.00, 1.00, 8.00, 17.00, 2.00, 'fl oz'),
        ('CMG-5412', 'Cilantro-Lime Sauce', 'sauce', 80.00, 2.00, 3.00, 6.00, 2.00, 'oz')
),
chipotle_locations(location_id) AS (
    VALUES
        ('loc-chipotle-charlotte'),
        ('loc-chipotle-raleigh')
)
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
SELECT
    'ing-' || locations.location_id || '-' || lower(replace(seed.source_item_id, 'CMG-', 'cmg-')),
    locations.location_id,
    seed.source_item_id,
    seed.name,
    seed.category,
    'each',
    seed.calories,
    seed.protein,
    seed.carbs,
    seed.fat,
    0,
    'USD',
    0,
    0,
    'restaurant_official',
    'verified',
    seed.serving_size_quantity,
    seed.serving_size_unit
FROM chipotle_seed AS seed
CROSS JOIN chipotle_locations AS locations
ON CONFLICT (location_id, source_item_id) DO NOTHING;
