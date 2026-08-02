package provisioning

import (
	"testing"
	"time"

	"github.com/ruth411/circle/internal/identity"
)

func TestValidateExistingSessionRejectsExpiredSession(t *testing.T) {
	now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	session := identity.Session{
		ID:             "session-1",
		UserID:         "user-1",
		OrganizationID: "org-1",
		ScopeType:      identity.ScopeTypeLocation,
		LocationID:     "loc-1",
		ExpiresAt:      now.Add(-time.Minute),
	}

	err := validateExistingSession(session, ProvisionLocationParams{
		SessionID:        "session-1",
		UserID:           "user-1",
		OrganizationID:   "org-1",
		ScopeType:        identity.ScopeTypeLocation,
		LocationID:       "loc-1",
		SessionExpiresAt: now.Add(24 * time.Hour),
	}, now)
	if err == nil {
		t.Fatal("expected expired session to be rejected")
	}
}

func TestValidateExistingSessionAcceptsMatchingActiveSession(t *testing.T) {
	now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	session := identity.Session{
		ID:             "session-1",
		UserID:         "user-1",
		OrganizationID: "org-1",
		ScopeType:      identity.ScopeTypeLocation,
		LocationID:     "loc-1",
		ExpiresAt:      now.Add(24 * time.Hour),
	}

	if err := validateExistingSession(session, ProvisionLocationParams{
		SessionID:        "session-1",
		UserID:           "user-1",
		OrganizationID:   "org-1",
		ScopeType:        identity.ScopeTypeLocation,
		LocationID:       "loc-1",
		SessionExpiresAt: now.Add(24 * time.Hour),
	}, now); err != nil {
		t.Fatalf("expected active matching session to be accepted, got %v", err)
	}
}
