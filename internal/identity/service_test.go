package identity

import (
	"errors"
	"testing"
	"time"
)

func TestIssueValidateAndRevokeSession(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 7, 24, 10, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }

	if err := service.AddUser(User{
		ID:             "user-1",
		OrganizationID: "org-1",
		LocationID:     "loc-1",
		Email:          "staff@example.com",
		DisplayName:    "Staff",
		PasswordHash:   "hash",
	}); err != nil {
		t.Fatalf("AddUser returned error: %v", err)
	}

	session, err := service.IssueSession("session-1", "user-1", time.Hour)
	if err != nil {
		t.Fatalf("IssueSession returned error: %v", err)
	}
	if session.LocationID != "loc-1" {
		t.Fatalf("session location = %q, want loc-1", session.LocationID)
	}
	if session.OrganizationID != "org-1" {
		t.Fatalf("session organization = %q, want org-1", session.OrganizationID)
	}

	validated, err := service.ValidateSession("session-1")
	if err != nil {
		t.Fatalf("ValidateSession returned error: %v", err)
	}
	if validated.ID != "session-1" {
		t.Fatalf("session id = %q, want session-1", validated.ID)
	}

	if err := service.RevokeSession("session-1"); err != nil {
		t.Fatalf("RevokeSession returned error: %v", err)
	}
	if _, err := service.ValidateSession("session-1"); !errors.Is(err, ErrSessionRevoked) {
		t.Fatalf("err = %v, want ErrSessionRevoked", err)
	}
}

func TestValidateSessionRejectsExpiredSession(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 7, 24, 10, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }

	if err := service.AddUser(User{
		ID:             "user-1",
		OrganizationID: "org-1",
		LocationID:     "loc-1",
		Email:          "staff@example.com",
		DisplayName:    "Staff",
		PasswordHash:   "hash",
	}); err != nil {
		t.Fatalf("AddUser returned error: %v", err)
	}

	if _, err := service.IssueSession("session-1", "user-1", time.Minute); err != nil {
		t.Fatalf("IssueSession returned error: %v", err)
	}

	service.now = func() time.Time { return now.Add(2 * time.Minute) }
	if _, err := service.ValidateSession("session-1"); !errors.Is(err, ErrSessionExpired) {
		t.Fatalf("err = %v, want ErrSessionExpired", err)
	}
}

func TestAssignRoleRejectsCrossLocationRole(t *testing.T) {
	service := NewService()

	if err := service.AddUser(User{
		ID:             "user-1",
		OrganizationID: "org-1",
		LocationID:     "loc-1",
		Email:          "staff@example.com",
		DisplayName:    "Staff",
		PasswordHash:   "hash",
	}); err != nil {
		t.Fatalf("AddUser returned error: %v", err)
	}
	if err := service.AddRole(Role{
		ID:             "role-1",
		OrganizationID: "org-1",
		LocationID:     "loc-2",
		Name:           "manager",
	}); err != nil {
		t.Fatalf("AddRole returned error: %v", err)
	}

	if err := service.AssignRole("user-1", "role-1"); !errors.Is(err, ErrLocationMismatch) {
		t.Fatalf("err = %v, want ErrLocationMismatch", err)
	}
}

func TestAuthorizeLocationAccessAllowsOrganizationScopeAcrossLocations(t *testing.T) {
	err := AuthorizeLocationAccess(Session{
		ID:             "session-1",
		OrganizationID: "org-1",
		ScopeType:      ScopeTypeOrganization,
	}, "loc-2", "org-1")
	if err != nil {
		t.Fatalf("AuthorizeLocationAccess returned error: %v", err)
	}
}

func TestAuthorizeLocationAccessRejectsCrossOrganizationAccess(t *testing.T) {
	err := AuthorizeLocationAccess(Session{
		ID:             "session-1",
		OrganizationID: "org-1",
		ScopeType:      ScopeTypeOrganization,
	}, "loc-2", "org-2")
	if !errors.Is(err, ErrOrganizationMismatch) {
		t.Fatalf("err = %v, want ErrOrganizationMismatch", err)
	}
}
