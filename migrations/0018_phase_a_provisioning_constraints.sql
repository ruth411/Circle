DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'identity_users_organization_fk'
    ) THEN
        ALTER TABLE identity.users
            ADD CONSTRAINT identity_users_organization_fk
            FOREIGN KEY (organization_id) REFERENCES tenancy.organizations (id);
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'identity_users_location_fk'
    ) THEN
        ALTER TABLE identity.users
            ADD CONSTRAINT identity_users_location_fk
            FOREIGN KEY (location_id) REFERENCES tenancy.locations (id);
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'identity_roles_organization_fk'
    ) THEN
        ALTER TABLE identity.roles
            ADD CONSTRAINT identity_roles_organization_fk
            FOREIGN KEY (organization_id) REFERENCES tenancy.organizations (id);
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'identity_roles_location_fk'
    ) THEN
        ALTER TABLE identity.roles
            ADD CONSTRAINT identity_roles_location_fk
            FOREIGN KEY (location_id) REFERENCES tenancy.locations (id);
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'identity_sessions_organization_fk'
    ) THEN
        ALTER TABLE identity.sessions
            ADD CONSTRAINT identity_sessions_organization_fk
            FOREIGN KEY (organization_id) REFERENCES tenancy.organizations (id);
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'identity_sessions_location_fk'
    ) THEN
        ALTER TABLE identity.sessions
            ADD CONSTRAINT identity_sessions_location_fk
            FOREIGN KEY (location_id) REFERENCES tenancy.locations (id);
    END IF;
END $$;
