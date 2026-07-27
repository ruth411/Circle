CREATE TABLE IF NOT EXISTS tenancy.organizations (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    archived_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS tenancy.restaurants (
    id TEXT PRIMARY KEY,
    organization_id TEXT NOT NULL REFERENCES tenancy.organizations (id),
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    archived_at TIMESTAMPTZ,
    UNIQUE (organization_id, name)
);

CREATE TABLE IF NOT EXISTS tenancy.locations (
    id TEXT PRIMARY KEY,
    restaurant_id TEXT NOT NULL REFERENCES tenancy.restaurants (id),
    name TEXT NOT NULL,
    timezone_name TEXT NOT NULL,
    currency TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    archived_at TIMESTAMPTZ,
    UNIQUE (restaurant_id, name)
);

CREATE TABLE IF NOT EXISTS identity.users (
    id TEXT PRIMARY KEY,
    location_id TEXT NOT NULL,
    email TEXT NOT NULL,
    display_name TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    archived_at TIMESTAMPTZ,
    UNIQUE (location_id, email)
);

CREATE TABLE IF NOT EXISTS identity.roles (
    id TEXT PRIMARY KEY,
    location_id TEXT NOT NULL,
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (location_id, name)
);

CREATE TABLE IF NOT EXISTS identity.user_roles (
    user_id TEXT NOT NULL REFERENCES identity.users (id) ON DELETE CASCADE,
    role_id TEXT NOT NULL REFERENCES identity.roles (id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, role_id)
);

CREATE TABLE IF NOT EXISTS identity.sessions (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES identity.users (id) ON DELETE CASCADE,
    location_id TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS identity_sessions_user_idx
    ON identity.sessions (user_id);

CREATE INDEX IF NOT EXISTS identity_sessions_location_expiry_idx
    ON identity.sessions (location_id, expires_at);
