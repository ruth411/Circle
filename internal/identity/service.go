package identity

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"
)

var (
	ErrUserNotFound         = errors.New("user not found")
	ErrRoleNotFound         = errors.New("role not found")
	ErrSessionNotFound      = errors.New("session not found")
	ErrSessionExpired       = errors.New("session expired")
	ErrSessionRevoked       = errors.New("session revoked")
	ErrInactiveUser         = errors.New("user is inactive")
	ErrLocationMismatch     = errors.New("session location mismatch")
	ErrOrganizationMismatch = errors.New("session organization mismatch")
)

type ScopeType string

const (
	ScopeTypeLocation     ScopeType = "location"
	ScopeTypeOrganization ScopeType = "organization"
)

type User struct {
	ID             string
	OrganizationID string
	ScopeType      ScopeType
	LocationID     string
	Email          string
	DisplayName    string
	PasswordHash   string
	Active         bool
}

type Role struct {
	ID             string
	OrganizationID string
	ScopeType      ScopeType
	LocationID     string
	Name           string
}

type Session struct {
	ID             string
	UserID         string
	OrganizationID string
	ScopeType      ScopeType
	LocationID     string
	ExpiresAt      time.Time
	RevokedAt      *time.Time
	CreatedAt      time.Time
}

type contextKey string

const sessionKey contextKey = "identity.session"

type Service struct {
	mu        sync.Mutex
	now       func() time.Time
	users     map[string]User
	roles     map[string]Role
	userRoles map[string]map[string]bool
	sessions  map[string]Session
}

func NewService() *Service {
	return &Service{
		now:       func() time.Time { return time.Now().UTC() },
		users:     map[string]User{},
		roles:     map[string]Role{},
		userRoles: map[string]map[string]bool{},
		sessions:  map[string]Session{},
	}
}

func (s *Service) AddUser(user User) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	user.ID = strings.TrimSpace(user.ID)
	user.OrganizationID = strings.TrimSpace(user.OrganizationID)
	user.LocationID = strings.TrimSpace(user.LocationID)
	user.Email = strings.TrimSpace(user.Email)
	user.DisplayName = strings.TrimSpace(user.DisplayName)
	user.PasswordHash = strings.TrimSpace(user.PasswordHash)
	if user.ScopeType == "" {
		user.ScopeType = ScopeTypeLocation
	}

	if user.ID == "" {
		return fmt.Errorf("user id is required")
	}
	if user.OrganizationID == "" {
		return fmt.Errorf("user organization id is required")
	}
	if user.ScopeType == ScopeTypeLocation && user.LocationID == "" {
		return fmt.Errorf("user location id is required")
	}
	if user.ScopeType == ScopeTypeOrganization && user.LocationID != "" {
		return fmt.Errorf("organization user cannot include a location id")
	}
	if user.ScopeType != ScopeTypeLocation && user.ScopeType != ScopeTypeOrganization {
		return fmt.Errorf("user scope type is invalid")
	}
	if user.Email == "" {
		return fmt.Errorf("user email is required")
	}
	if user.DisplayName == "" {
		return fmt.Errorf("user display name is required")
	}
	if user.PasswordHash == "" {
		return fmt.Errorf("user password hash is required")
	}

	user.Active = true
	s.users[user.ID] = user
	return nil
}

func (s *Service) AddRole(role Role) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	role.ID = strings.TrimSpace(role.ID)
	role.OrganizationID = strings.TrimSpace(role.OrganizationID)
	role.LocationID = strings.TrimSpace(role.LocationID)
	role.Name = strings.TrimSpace(role.Name)
	if role.ScopeType == "" {
		role.ScopeType = ScopeTypeLocation
	}

	if role.ID == "" {
		return fmt.Errorf("role id is required")
	}
	if role.OrganizationID == "" {
		return fmt.Errorf("role organization id is required")
	}
	if role.ScopeType == ScopeTypeLocation && role.LocationID == "" {
		return fmt.Errorf("role location id is required")
	}
	if role.ScopeType == ScopeTypeOrganization && role.LocationID != "" {
		return fmt.Errorf("organization role cannot include a location id")
	}
	if role.ScopeType != ScopeTypeLocation && role.ScopeType != ScopeTypeOrganization {
		return fmt.Errorf("role scope type is invalid")
	}
	if role.Name == "" {
		return fmt.Errorf("role name is required")
	}

	s.roles[role.ID] = role
	return nil
}

func (s *Service) AssignRole(userID string, roleID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	user, ok := s.users[userID]
	if !ok {
		return ErrUserNotFound
	}

	role, ok := s.roles[roleID]
	if !ok {
		return ErrRoleNotFound
	}
	if role.OrganizationID != user.OrganizationID {
		return fmt.Errorf("%w: user %s and role %s", ErrOrganizationMismatch, userID, roleID)
	}
	if role.ScopeType != user.ScopeType {
		return fmt.Errorf("user %s and role %s scope types do not match", userID, roleID)
	}
	if role.ScopeType == ScopeTypeLocation && role.LocationID != user.LocationID {
		return fmt.Errorf("%w: user %s and role %s", ErrLocationMismatch, userID, roleID)
	}

	if s.userRoles[userID] == nil {
		s.userRoles[userID] = map[string]bool{}
	}
	s.userRoles[userID][roleID] = true
	return nil
}

func (s *Service) RolesForUser(userID string) []Role {
	s.mu.Lock()
	defer s.mu.Unlock()

	roleIDs := s.userRoles[userID]
	out := make([]Role, 0, len(roleIDs))
	for roleID := range roleIDs {
		out = append(out, s.roles[roleID])
	}
	slices.SortFunc(out, func(a Role, b Role) int {
		return strings.Compare(a.ID, b.ID)
	})
	return out
}

func (s *Service) IssueSession(sessionID string, userID string, ttl time.Duration) (Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if strings.TrimSpace(sessionID) == "" {
		return Session{}, fmt.Errorf("session id is required")
	}
	if ttl <= 0 {
		return Session{}, fmt.Errorf("session ttl must be positive")
	}

	user, ok := s.users[userID]
	if !ok {
		return Session{}, ErrUserNotFound
	}
	if !user.Active {
		return Session{}, ErrInactiveUser
	}

	now := s.now()
	session := Session{
		ID:             sessionID,
		UserID:         user.ID,
		OrganizationID: user.OrganizationID,
		ScopeType:      user.ScopeType,
		LocationID:     user.LocationID,
		ExpiresAt:      now.Add(ttl),
		CreatedAt:      now,
	}
	s.sessions[session.ID] = session
	return session, nil
}

func (s *Service) ValidateSession(_ context.Context, sessionID string) (Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, ok := s.sessions[sessionID]
	if !ok {
		return Session{}, ErrSessionNotFound
	}
	if session.RevokedAt != nil {
		return Session{}, ErrSessionRevoked
	}
	if !session.ExpiresAt.After(s.now()) {
		return Session{}, ErrSessionExpired
	}

	user, ok := s.users[session.UserID]
	if !ok {
		return Session{}, ErrUserNotFound
	}
	if !user.Active {
		return Session{}, ErrInactiveUser
	}

	return session, nil
}

func (s *Service) RevokeSession(sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, ok := s.sessions[sessionID]
	if !ok {
		return ErrSessionNotFound
	}
	now := s.now()
	session.RevokedAt = &now
	s.sessions[sessionID] = session
	return nil
}

func WithSession(ctx context.Context, session Session) context.Context {
	return context.WithValue(ctx, sessionKey, session)
}

func SessionFromContext(ctx context.Context) (Session, bool) {
	session, ok := ctx.Value(sessionKey).(Session)
	return session, ok && session.ID != ""
}

func AuthorizeLocationAccess(session Session, locationID string, organizationID string) error {
	switch session.ScopeType {
	case ScopeTypeLocation:
		if session.LocationID != locationID {
			return ErrLocationMismatch
		}
	case ScopeTypeOrganization:
		if session.OrganizationID != organizationID {
			return ErrOrganizationMismatch
		}
	default:
		return ErrLocationMismatch
	}

	return nil
}
