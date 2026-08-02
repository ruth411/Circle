package provisioning

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/ruth411/circle/internal/identity"
	platformdb "github.com/ruth411/circle/internal/platform/db"
)

type SQLRepository struct {
	db *sql.DB
}

func NewSQLRepository(db *sql.DB) *SQLRepository {
	return &SQLRepository{db: db}
}

func (r *SQLRepository) ProvisionLocation(ctx context.Context, params ProvisionLocationParams) (ProvisionedLocation, error) {
	if r.db == nil {
		return ProvisionedLocation{}, fmt.Errorf("sql database is required")
	}

	var provisioned ProvisionedLocation
	err := platformdb.WithTx(ctx, platformdb.BeginFromSQL(r.db), nil, func(ctx context.Context, tx platformdb.Tx) error {
		sqlTx, ok := tx.(*sql.Tx)
		if !ok {
			return fmt.Errorf("sql transaction is required")
		}

		if err := ensureOrganization(ctx, sqlTx, params); err != nil {
			return err
		}
		if err := ensureRestaurant(ctx, sqlTx, params); err != nil {
			return err
		}
		if err := ensureLocation(ctx, sqlTx, params); err != nil {
			return err
		}
		if err := ensureRole(ctx, sqlTx, params); err != nil {
			return err
		}
		if err := ensureUser(ctx, sqlTx, params); err != nil {
			return err
		}
		if err := assignRole(ctx, sqlTx, params); err != nil {
			return err
		}

		session, err := ensureSession(ctx, sqlTx, params)
		if err != nil {
			return err
		}

		provisioned = ProvisionedLocation{
			OrganizationID: params.OrganizationID,
			RestaurantID:   params.RestaurantID,
			LocationID:     params.LocationID,
			RoleID:         params.RoleID,
			UserID:         params.UserID,
			Session:        session,
		}
		return nil
	})
	if err != nil {
		return ProvisionedLocation{}, err
	}
	return provisioned, nil
}

func ensureOrganization(ctx context.Context, tx *sql.Tx, params ProvisionLocationParams) error {
	const insertSQL = `
INSERT INTO tenancy.organizations (id, name)
VALUES ($1, $2)
ON CONFLICT (id) DO NOTHING;
`
	if _, err := tx.ExecContext(ctx, insertSQL, params.OrganizationID, params.OrganizationName); err != nil {
		return mapProvisioningWriteError("organization", err)
	}

	const selectSQL = `
SELECT name
FROM tenancy.organizations
WHERE id = $1;
`
	var name string
	if err := tx.QueryRowContext(ctx, selectSQL, params.OrganizationID).Scan(&name); err != nil {
		return err
	}
	if name != params.OrganizationName {
		return fmt.Errorf("%w: organization %s already exists with different attributes", ErrProvisioningConflict, params.OrganizationID)
	}
	return nil
}

func ensureRestaurant(ctx context.Context, tx *sql.Tx, params ProvisionLocationParams) error {
	const insertSQL = `
INSERT INTO tenancy.restaurants (id, organization_id, name)
VALUES ($1, $2, $3)
ON CONFLICT (id) DO NOTHING;
`
	if _, err := tx.ExecContext(ctx, insertSQL, params.RestaurantID, params.OrganizationID, params.RestaurantName); err != nil {
		return mapProvisioningWriteError("restaurant", err)
	}

	const selectSQL = `
SELECT organization_id, name
FROM tenancy.restaurants
WHERE id = $1;
`
	var organizationID string
	var name string
	if err := tx.QueryRowContext(ctx, selectSQL, params.RestaurantID).Scan(&organizationID, &name); err != nil {
		return err
	}
	if organizationID != params.OrganizationID || name != params.RestaurantName {
		return fmt.Errorf("%w: restaurant %s already exists with different attributes", ErrProvisioningConflict, params.RestaurantID)
	}
	return nil
}

func ensureLocation(ctx context.Context, tx *sql.Tx, params ProvisionLocationParams) error {
	const insertSQL = `
INSERT INTO tenancy.locations (id, restaurant_id, name, timezone_name, currency)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (id) DO NOTHING;
`
	if _, err := tx.ExecContext(ctx, insertSQL, params.LocationID, params.RestaurantID, params.LocationName, params.TimezoneName, params.Currency); err != nil {
		return mapProvisioningWriteError("location", err)
	}

	const selectSQL = `
SELECT restaurant_id, name, timezone_name, currency
FROM tenancy.locations
WHERE id = $1;
`
	var restaurantID string
	var name string
	var timezoneName string
	var currency string
	if err := tx.QueryRowContext(ctx, selectSQL, params.LocationID).Scan(&restaurantID, &name, &timezoneName, &currency); err != nil {
		return err
	}
	if restaurantID != params.RestaurantID || name != params.LocationName || timezoneName != params.TimezoneName || currency != params.Currency {
		return fmt.Errorf("%w: location %s already exists with different attributes", ErrProvisioningConflict, params.LocationID)
	}
	return nil
}

func ensureRole(ctx context.Context, tx *sql.Tx, params ProvisionLocationParams) error {
	const insertSQL = `
INSERT INTO identity.roles (id, organization_id, location_id, scope_type, name)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (id) DO NOTHING;
`
	if _, err := tx.ExecContext(ctx, insertSQL, params.RoleID, params.OrganizationID, params.LocationID, string(params.ScopeType), params.RoleName); err != nil {
		return mapProvisioningWriteError("role", err)
	}

	const selectSQL = `
SELECT organization_id, location_id, scope_type, name
FROM identity.roles
WHERE id = $1;
`
	var organizationID string
	var locationID sql.NullString
	var scopeType string
	var name string
	if err := tx.QueryRowContext(ctx, selectSQL, params.RoleID).Scan(&organizationID, &locationID, &scopeType, &name); err != nil {
		return err
	}
	if organizationID != params.OrganizationID || !locationID.Valid || locationID.String != params.LocationID || scopeType != string(params.ScopeType) || name != params.RoleName {
		return fmt.Errorf("%w: role %s already exists with different attributes", ErrProvisioningConflict, params.RoleID)
	}
	return nil
}

func ensureUser(ctx context.Context, tx *sql.Tx, params ProvisionLocationParams) error {
	const insertSQL = `
INSERT INTO identity.users (id, organization_id, location_id, scope_type, email, display_name, password_hash)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (id) DO NOTHING;
`
	if _, err := tx.ExecContext(ctx, insertSQL, params.UserID, params.OrganizationID, params.LocationID, string(params.ScopeType), params.Email, params.DisplayName, params.PasswordHash); err != nil {
		return mapProvisioningWriteError("user", err)
	}

	const selectSQL = `
SELECT organization_id, location_id, scope_type, email, display_name, password_hash, is_active
FROM identity.users
WHERE id = $1;
`
	var organizationID string
	var locationID sql.NullString
	var scopeType string
	var email string
	var displayName string
	var passwordHash string
	var isActive bool
	if err := tx.QueryRowContext(ctx, selectSQL, params.UserID).Scan(&organizationID, &locationID, &scopeType, &email, &displayName, &passwordHash, &isActive); err != nil {
		return err
	}
	if organizationID != params.OrganizationID || !locationID.Valid || locationID.String != params.LocationID || scopeType != string(params.ScopeType) || email != params.Email || displayName != params.DisplayName || passwordHash != params.PasswordHash || !isActive {
		return fmt.Errorf("%w: user %s already exists with different attributes", ErrProvisioningConflict, params.UserID)
	}
	return nil
}

func assignRole(ctx context.Context, tx *sql.Tx, params ProvisionLocationParams) error {
	const insertSQL = `
INSERT INTO identity.user_roles (user_id, role_id)
VALUES ($1, $2)
ON CONFLICT (user_id, role_id) DO NOTHING;
`
	_, err := tx.ExecContext(ctx, insertSQL, params.UserID, params.RoleID)
	return err
}

func ensureSession(ctx context.Context, tx *sql.Tx, params ProvisionLocationParams) (identity.Session, error) {
	const insertSQL = `
INSERT INTO identity.sessions (id, user_id, organization_id, location_id, scope_type, expires_at)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (id) DO NOTHING;
`
	if _, err := tx.ExecContext(ctx, insertSQL, params.SessionID, params.UserID, params.OrganizationID, params.LocationID, string(params.ScopeType), params.SessionExpiresAt); err != nil {
		return identity.Session{}, mapProvisioningWriteError("session", err)
	}

	const selectSQL = `
SELECT id, user_id, organization_id, scope_type, location_id, expires_at, revoked_at, created_at
FROM identity.sessions
WHERE id = $1;
`
	var session identity.Session
	var scopeType string
	var locationID sql.NullString
	if err := tx.QueryRowContext(ctx, selectSQL, params.SessionID).Scan(
		&session.ID,
		&session.UserID,
		&session.OrganizationID,
		&scopeType,
		&locationID,
		&session.ExpiresAt,
		&session.RevokedAt,
		&session.CreatedAt,
	); err != nil {
		return identity.Session{}, err
	}
	session.ScopeType = identity.ScopeType(scopeType)
	if locationID.Valid {
		session.LocationID = locationID.String
	}
	if err := validateExistingSession(session, params, time.Now().UTC()); err != nil {
		return identity.Session{}, err
	}
	return session, nil
}

func validateExistingSession(session identity.Session, params ProvisionLocationParams, now time.Time) error {
	if session.UserID != params.UserID || session.OrganizationID != params.OrganizationID || session.ScopeType != params.ScopeType || session.LocationID != params.LocationID {
		return fmt.Errorf("%w: session %s already exists with different attributes", ErrProvisioningConflict, params.SessionID)
	}
	if session.RevokedAt != nil {
		return fmt.Errorf("%w: session %s is revoked", ErrProvisioningConflict, params.SessionID)
	}
	if !session.ExpiresAt.After(now) {
		return fmt.Errorf("%w: session %s is expired", ErrProvisioningConflict, params.SessionID)
	}
	return nil
}

func mapProvisioningWriteError(entity string, err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return fmt.Errorf("%w: %s already exists", ErrProvisioningConflict, entity)
	}
	return err
}
