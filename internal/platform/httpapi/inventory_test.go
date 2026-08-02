package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ruth411/circle/internal/contracts"
	"github.com/ruth411/circle/internal/core/ingredient"
	"github.com/ruth411/circle/internal/identity"
	"github.com/ruth411/circle/internal/inventory"
	"github.com/ruth411/circle/internal/tenancy"
)

type fakeInventoryRepository struct {
	recordDepletionFn  func(context.Context, contracts.ClosedOrder) ([]inventory.Movement, error)
	recordReceiptFn    func(context.Context, contracts.PurchaseReceipt) ([]inventory.Movement, error)
	listMovementsFn    func(context.Context, string) ([]inventory.Movement, error)
	onHandFn           func(context.Context, string) ([]inventory.OnHandItem, error)
	listOrgMovementsFn func(context.Context, string, string) ([]inventory.Movement, error)
	orgOnHandFn        func(context.Context, string, string) ([]inventory.OnHandItem, error)
}

func (f fakeInventoryRepository) RecordDepletion(ctx context.Context, order contracts.ClosedOrder) ([]inventory.Movement, error) {
	return f.recordDepletionFn(ctx, order)
}

func (f fakeInventoryRepository) RecordReceipt(ctx context.Context, receipt contracts.PurchaseReceipt) ([]inventory.Movement, error) {
	return f.recordReceiptFn(ctx, receipt)
}

func (f fakeInventoryRepository) ListMovements(ctx context.Context, locationID string) ([]inventory.Movement, error) {
	return f.listMovementsFn(ctx, locationID)
}

func (f fakeInventoryRepository) OnHand(ctx context.Context, locationID string) ([]inventory.OnHandItem, error) {
	return f.onHandFn(ctx, locationID)
}

func (f fakeInventoryRepository) ListOrganizationMovements(ctx context.Context, organizationID string, locationID string) ([]inventory.Movement, error) {
	return f.listOrgMovementsFn(ctx, organizationID, locationID)
}

func (f fakeInventoryRepository) OrganizationOnHand(ctx context.Context, organizationID string, locationID string) ([]inventory.OnHandItem, error) {
	return f.orgOnHandFn(ctx, organizationID, locationID)
}

func TestInventoryMovementsRouteReturnsTenantScopedResults(t *testing.T) {
	service := inventory.NewService(fakeInventoryRepository{
		listMovementsFn: func(_ context.Context, locationID string) ([]inventory.Movement, error) {
			if locationID != "loc-1" {
				t.Fatalf("locationID = %q, want loc-1", locationID)
			}
			return []inventory.Movement{{
				ID:           "mv-1",
				LocationID:   "loc-1",
				SourceType:   contracts.PurchaseReceiptSourceType,
				SourceID:     "rec-1",
				IngredientID: "ing-1",
				Quantity:     1500,
				Unit:         ingredient.UnitGram,
				OccurredAt:   time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC),
			}}, nil
		},
	})

	server := NewServerWithDependencies(slog.New(slog.NewTextHandler(io.Discard, nil)), Dependencies{
		InventoryService:     service,
		SessionValidator:     seedSessionService(t, "loc-1"),
		OrganizationResolver: tenancy.StaticOrganizationResolver{"loc-1": "org-1"},
	})

	req := httptest.NewRequest(http.MethodGet, "/inventory/movements", nil)
	req.Header.Set("X-Location-Id", "loc-1")
	req.Header.Set(sessionIDHeader, "session-1")
	recorder := httptest.NewRecorder()

	server.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}

	var payload struct {
		Movements []inventoryMovementResponse `json:"movements"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	if len(payload.Movements) != 1 {
		t.Fatalf("movement count = %d, want 1", len(payload.Movements))
	}
	if payload.Movements[0].LocationID != "loc-1" {
		t.Fatalf("location id = %q, want loc-1", payload.Movements[0].LocationID)
	}
}

func TestInventoryOnHandRouteReturnsAggregatedItems(t *testing.T) {
	service := inventory.NewService(fakeInventoryRepository{
		onHandFn: func(_ context.Context, locationID string) ([]inventory.OnHandItem, error) {
			if locationID != "loc-1" {
				t.Fatalf("locationID = %q, want loc-1", locationID)
			}
			return []inventory.OnHandItem{{
				LocationID:     "loc-1",
				IngredientID:   "ing-1",
				IngredientName: "Chicken",
				BaseUnit:       ingredient.UnitGram,
				OnHandQuantity: 1350,
			}}, nil
		},
	})

	server := NewServerWithDependencies(slog.New(slog.NewTextHandler(io.Discard, nil)), Dependencies{
		InventoryService:     service,
		SessionValidator:     seedSessionService(t, "loc-1"),
		OrganizationResolver: tenancy.StaticOrganizationResolver{"loc-1": "org-1"},
	})

	req := httptest.NewRequest(http.MethodGet, "/inventory/on-hand", nil)
	req.Header.Set("X-Location-Id", "loc-1")
	req.Header.Set(sessionIDHeader, "session-1")
	recorder := httptest.NewRecorder()

	server.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}

	var payload struct {
		Items []inventoryOnHandResponse `json:"items"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	if len(payload.Items) != 1 {
		t.Fatalf("item count = %d, want 1", len(payload.Items))
	}
	if payload.Items[0].OnHandQuantity != 1350 {
		t.Fatalf("on hand = %v, want 1350", payload.Items[0].OnHandQuantity)
	}
}

func TestOrganizationInventoryMovementsRouteRequiresOrganizationScope(t *testing.T) {
	service := inventory.NewService(fakeInventoryRepository{
		listOrgMovementsFn: func(_ context.Context, _, _ string) ([]inventory.Movement, error) {
			t.Fatal("ListOrganizationMovements should not be called")
			return nil, nil
		},
	})

	server := NewServerWithDependencies(slog.New(slog.NewTextHandler(io.Discard, nil)), Dependencies{
		InventoryService:     service,
		SessionValidator:     seedSessionService(t, "loc-1"),
		OrganizationResolver: tenancy.StaticOrganizationResolver{"loc-1": "org-1"},
	})

	req := httptest.NewRequest(http.MethodGet, "/org/inventory/movements", nil)
	req.Header.Set(sessionIDHeader, "session-1")
	recorder := httptest.NewRecorder()

	server.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", recorder.Code)
	}
}

func TestOrganizationInventoryOnHandRouteReturnsOrganizationScopedResults(t *testing.T) {
	service := inventory.NewService(fakeInventoryRepository{
		orgOnHandFn: func(_ context.Context, organizationID string, locationID string) ([]inventory.OnHandItem, error) {
			if organizationID != "org-1" {
				t.Fatalf("organizationID = %q, want org-1", organizationID)
			}
			if locationID != "loc-2" {
				t.Fatalf("locationID = %q, want loc-2", locationID)
			}
			return []inventory.OnHandItem{{
				LocationID:     "loc-2",
				IngredientID:   "ing-1",
				IngredientName: "Chicken",
				BaseUnit:       ingredient.UnitGram,
				OnHandQuantity: 2400,
			}}, nil
		},
	})

	server := NewServerWithDependencies(slog.New(slog.NewTextHandler(io.Discard, nil)), Dependencies{
		InventoryService:     service,
		SessionValidator:     seedOrganizationSessionService(t, "org-1"),
		OrganizationResolver: tenancy.StaticOrganizationResolver{"loc-2": "org-1"},
	})

	req := httptest.NewRequest(http.MethodGet, "/org/inventory/on-hand?location_id=loc-2", nil)
	req.Header.Set(sessionIDHeader, "session-1")
	recorder := httptest.NewRecorder()

	server.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}

	var payload struct {
		Items []inventoryOnHandResponse `json:"items"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	if len(payload.Items) != 1 {
		t.Fatalf("item count = %d, want 1", len(payload.Items))
	}
	if payload.Items[0].LocationID != "loc-2" {
		t.Fatalf("location id = %q, want loc-2", payload.Items[0].LocationID)
	}
}

func seedOrganizationSessionService(t *testing.T, organizationID string) *identity.Service {
	t.Helper()

	service := identity.NewService()
	if err := service.AddUser(identity.User{
		ID:             "user-1",
		OrganizationID: organizationID,
		ScopeType:      identity.ScopeTypeOrganization,
		Email:          "hq@example.com",
		DisplayName:    "HQ",
		PasswordHash:   "hash",
	}); err != nil {
		t.Fatalf("AddUser returned error: %v", err)
	}
	if _, err := service.IssueSession("session-1", "user-1", time.Hour); err != nil {
		t.Fatalf("IssueSession returned error: %v", err)
	}

	return service
}
