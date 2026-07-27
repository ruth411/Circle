package httpapi

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ruth411/circle/internal/identity"
	"github.com/ruth411/circle/internal/tenancy"
)

func TestWithResolvedLocationStoresLocationID(t *testing.T) {
	handler := WithResolvedLocation(tenancy.HeaderResolver{}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		locationID, ok := tenancy.LocationID(r.Context())
		if !ok {
			t.Fatal("expected location id in context")
		}
		WriteJSON(w, http.StatusOK, map[string]string{"location_id": locationID})
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Location-Id", "loc-1")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	var payload map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	if payload["location_id"] != "loc-1" {
		t.Fatalf("location_id = %q, want loc-1", payload["location_id"])
	}
}

func TestRequireStaffSessionRejectsCrossLocationAccess(t *testing.T) {
	identityService := identity.NewService()
	if err := identityService.AddUser(identity.User{
		ID:             "user-1",
		OrganizationID: "org-1",
		LocationID:     "loc-1",
		Email:          "staff@example.com",
		DisplayName:    "Staff",
		PasswordHash:   "hash",
	}); err != nil {
		t.Fatalf("AddUser returned error: %v", err)
	}
	if _, err := identityService.IssueSession("session-1", "user-1", time.Hour); err != nil {
		t.Fatalf("IssueSession returned error: %v", err)
	}

	handler := WithResolvedLocation(tenancy.HeaderResolver{}, RequireStaffSession(identityService, tenancy.StaticOrganizationResolver{
		"loc-1": "org-1",
		"loc-2": "org-1",
	}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Location-Id", "loc-2")
	req.Header.Set(sessionIDHeader, "session-1")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", recorder.Code)
	}
}

func TestRequireStaffSessionAllowsMatchingLocation(t *testing.T) {
	identityService := identity.NewService()
	if err := identityService.AddUser(identity.User{
		ID:             "user-1",
		OrganizationID: "org-1",
		LocationID:     "loc-1",
		Email:          "staff@example.com",
		DisplayName:    "Staff",
		PasswordHash:   "hash",
	}); err != nil {
		t.Fatalf("AddUser returned error: %v", err)
	}
	if _, err := identityService.IssueSession("session-1", "user-1", time.Hour); err != nil {
		t.Fatalf("IssueSession returned error: %v", err)
	}

	handler := WithResolvedLocation(tenancy.HeaderResolver{}, RequireStaffSession(identityService, tenancy.StaticOrganizationResolver{
		"loc-1": "org-1",
	}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, ok := identity.SessionFromContext(r.Context())
		if !ok {
			t.Fatal("expected session in context")
		}
		WriteJSON(w, http.StatusOK, map[string]string{"session_id": session.ID})
	})))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Location-Id", "loc-1")
	req.Header.Set(sessionIDHeader, "session-1")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
}

func TestRequireStaffSessionAllowsOrganizationScopeAcrossLocations(t *testing.T) {
	identityService := identity.NewService()
	if err := identityService.AddUser(identity.User{
		ID:             "user-1",
		OrganizationID: "org-1",
		ScopeType:      identity.ScopeTypeOrganization,
		Email:          "hq@example.com",
		DisplayName:    "HQ",
		PasswordHash:   "hash",
	}); err != nil {
		t.Fatalf("AddUser returned error: %v", err)
	}
	if _, err := identityService.IssueSession("session-1", "user-1", time.Hour); err != nil {
		t.Fatalf("IssueSession returned error: %v", err)
	}

	handler := WithResolvedLocation(tenancy.HeaderResolver{}, RequireStaffSession(identityService, tenancy.StaticOrganizationResolver{
		"loc-2": "org-1",
	}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Location-Id", "loc-2")
	req.Header.Set(sessionIDHeader, "session-1")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
}

func TestRequireStaffSessionRejectsCrossOrganizationAccess(t *testing.T) {
	identityService := identity.NewService()
	if err := identityService.AddUser(identity.User{
		ID:             "user-1",
		OrganizationID: "org-1",
		ScopeType:      identity.ScopeTypeOrganization,
		Email:          "hq@example.com",
		DisplayName:    "HQ",
		PasswordHash:   "hash",
	}); err != nil {
		t.Fatalf("AddUser returned error: %v", err)
	}
	if _, err := identityService.IssueSession("session-1", "user-1", time.Hour); err != nil {
		t.Fatalf("IssueSession returned error: %v", err)
	}

	handler := WithResolvedLocation(tenancy.HeaderResolver{}, RequireStaffSession(identityService, tenancy.StaticOrganizationResolver{
		"loc-2": "org-2",
	}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Location-Id", "loc-2")
	req.Header.Set(sessionIDHeader, "session-1")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", recorder.Code)
	}
}

func TestNewServerStillBuildsWithoutAuthMiddleware(t *testing.T) {
	server := NewServer(slog.New(slog.NewTextHandler(io.Discard, nil)))
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	recorder := httptest.NewRecorder()

	server.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
}
