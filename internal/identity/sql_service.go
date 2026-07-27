package identity

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type SQLSessionValidator struct {
	db  *sql.DB
	now func() time.Time
}

func NewSQLSessionValidator(db *sql.DB) *SQLSessionValidator {
	return &SQLSessionValidator{
		db:  db,
		now: func() time.Time { return time.Now().UTC() },
	}
}

func (s *SQLSessionValidator) ValidateSession(sessionID string) (Session, error) {
	const query = `
SELECT
    sessions.id,
    sessions.user_id,
    sessions.organization_id,
    sessions.scope_type,
    sessions.location_id,
    sessions.expires_at,
    sessions.revoked_at,
    sessions.created_at,
    users.is_active
FROM identity.sessions AS sessions
JOIN identity.users AS users
    ON users.id = sessions.user_id
WHERE sessions.id = $1;
`

	var session Session
	var organizationID string
	var scopeType string
	var locationID sql.NullString
	var active bool
	err := s.db.QueryRowContext(context.Background(), query, sessionID).Scan(
		&session.ID,
		&session.UserID,
		&organizationID,
		&scopeType,
		&locationID,
		&session.ExpiresAt,
		&session.RevokedAt,
		&session.CreatedAt,
		&active,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Session{}, ErrSessionNotFound
		}
		return Session{}, err
	}
	session.OrganizationID = organizationID
	session.ScopeType = ScopeType(scopeType)
	if locationID.Valid {
		session.LocationID = locationID.String
	}
	if err := validateLoadedSession(session, active, s.now()); err != nil {
		return Session{}, err
	}

	return session, nil
}

func validateLoadedSession(session Session, active bool, now time.Time) error {
	if session.RevokedAt != nil {
		return ErrSessionRevoked
	}
	if !session.ExpiresAt.After(now) {
		return ErrSessionExpired
	}
	if !active {
		return ErrInactiveUser
	}
	return nil
}
