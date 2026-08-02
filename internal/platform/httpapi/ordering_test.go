package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ruth411/circle/internal/core/ingredient"
	"github.com/ruth411/circle/internal/core/recipe"
	"github.com/ruth411/circle/internal/ordering"
	"github.com/ruth411/circle/internal/platform/biztime"
	"github.com/ruth411/circle/internal/tenancy"
)

func TestOrderCreateRouteCreatesOrder(t *testing.T) {
	service := seedOrderingService()
	identityService := seedSessionService(t, "loc-1")
	server := NewServerWithDependencies(slog.New(slog.NewTextHandler(io.Discard, nil)), Dependencies{
		OrderingService:      service,
		SessionValidator:     identityService,
		OrganizationResolver: tenancy.StaticOrganizationResolver{"loc-1": "org-1"},
	})

	req := httptest.NewRequest(http.MethodPost, "/orders", bytes.NewBufferString(`{
		"id":"order-1",
		"snapshot_id":"snap-1",
		"business_date":"2026-07-28"
	}`))
	req.Header.Set("X-Location-Id", "loc-1")
	req.Header.Set(sessionIDHeader, "session-1")
	recorder := httptest.NewRecorder()

	server.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201, body = %s", recorder.Code, recorder.Body.String())
	}

	var payload struct {
		Order orderResponse `json:"order"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	if payload.Order.ID != "order-1" {
		t.Fatalf("order id = %q, want order-1", payload.Order.ID)
	}
	if payload.Order.Status != ordering.OrderStatusOpen {
		t.Fatalf("status = %s, want %s", payload.Order.Status, ordering.OrderStatusOpen)
	}
	if payload.Order.LocationID != "loc-1" {
		t.Fatalf("location id = %q, want loc-1", payload.Order.LocationID)
	}
}

func TestOrderAddLineRouteAddsResolvedLine(t *testing.T) {
	service := seedOrderingService()
	order, err := service.CreateOrder(bizContext(), ordering.CreateOrderInput{
		OrderID:      "order-1",
		LocationID:   "loc-1",
		SnapshotID:   "snap-1",
		BusinessDate: biztime.BusinessDate("2026-07-28"),
	})
	if err != nil {
		t.Fatalf("CreateOrder returned error: %v", err)
	}

	identityService := seedSessionService(t, "loc-1")
	server := NewServerWithDependencies(slog.New(slog.NewTextHandler(io.Discard, nil)), Dependencies{
		OrderingService:      service,
		SessionValidator:     identityService,
		OrganizationResolver: tenancy.StaticOrganizationResolver{"loc-1": "org-1"},
	})

	req := httptest.NewRequest(http.MethodPost, "/orders/"+order.ID+"/lines", bytes.NewBufferString(`{
		"menu_item_id":"bowl",
		"modifier_ids":["extra"],
		"quantity":2
	}`))
	req.Header.Set("X-Location-Id", "loc-1")
	req.Header.Set(sessionIDHeader, "session-1")
	recorder := httptest.NewRecorder()

	server.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201, body = %s", recorder.Code, recorder.Body.String())
	}

	var payload struct {
		Line orderLineResponse `json:"line"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	if payload.Line.ResolvedPriceMinor != 2900 {
		t.Fatalf("line price = %d, want 2900", payload.Line.ResolvedPriceMinor)
	}
	if payload.Line.ResolvedMacros.Calories != 1240 {
		t.Fatalf("line calories = %v, want 1240", payload.Line.ResolvedMacros.Calories)
	}
}

func TestOrderGetRouteReturnsAggregateMacros(t *testing.T) {
	service := seedOrderingService()
	order, err := service.CreateOrder(bizContext(), ordering.CreateOrderInput{
		OrderID:      "order-1",
		LocationID:   "loc-1",
		SnapshotID:   "snap-1",
		BusinessDate: biztime.BusinessDate("2026-07-28"),
	})
	if err != nil {
		t.Fatalf("CreateOrder returned error: %v", err)
	}
	if _, err := service.AddLine(bizContext(), ordering.AddLineInput{
		LocationID:  "loc-1",
		OrderID:     order.ID,
		MenuItemID:  "bowl",
		ModifierIDs: []string{"extra"},
		Quantity:    2,
	}); err != nil {
		t.Fatalf("AddLine returned error: %v", err)
	}

	server := NewServerWithDependencies(slog.New(slog.NewTextHandler(io.Discard, nil)), Dependencies{
		OrderingService:      service,
		SessionValidator:     seedSessionService(t, "loc-1"),
		OrganizationResolver: tenancy.StaticOrganizationResolver{"loc-1": "org-1"},
	})

	req := httptest.NewRequest(http.MethodGet, "/orders/"+order.ID, nil)
	req.Header.Set("X-Location-Id", "loc-1")
	req.Header.Set(sessionIDHeader, "session-1")
	recorder := httptest.NewRecorder()

	server.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body = %s", recorder.Code, recorder.Body.String())
	}

	var payload struct {
		Order struct {
			TotalMacros macroPayload `json:"total_macros"`
		} `json:"order"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	if payload.Order.TotalMacros.Calories != 1240 {
		t.Fatalf("total calories = %v, want 1240", payload.Order.TotalMacros.Calories)
	}
	if payload.Order.TotalMacros.ProteinGrams != 104 {
		t.Fatalf("total protein = %v, want 104", payload.Order.TotalMacros.ProteinGrams)
	}
}

func TestOrderCloseRouteRejectsUnderpaidTender(t *testing.T) {
	service := seedOrderingService()
	order, err := service.CreateOrder(bizContext(), ordering.CreateOrderInput{
		OrderID:      "order-1",
		LocationID:   "loc-1",
		SnapshotID:   "snap-1",
		BusinessDate: biztime.BusinessDate("2026-07-28"),
	})
	if err != nil {
		t.Fatalf("CreateOrder returned error: %v", err)
	}
	if _, err := service.AddLine(bizContext(), ordering.AddLineInput{
		LocationID:  "loc-1",
		OrderID:     order.ID,
		MenuItemID:  "bowl",
		ModifierIDs: []string{"extra"},
		Quantity:    1,
	}); err != nil {
		t.Fatalf("AddLine returned error: %v", err)
	}

	identityService := seedSessionService(t, "loc-1")
	server := NewServerWithDependencies(slog.New(slog.NewTextHandler(io.Discard, nil)), Dependencies{
		OrderingService:      service,
		SessionValidator:     identityService,
		OrganizationResolver: tenancy.StaticOrganizationResolver{"loc-1": "org-1"},
	})

	req := httptest.NewRequest(http.MethodPost, "/orders/"+order.ID+"/close", bytes.NewBufferString(`{
		"tender":{
			"id":"tender-1",
			"amount_minor":100,
			"currency":"USD",
			"kind":"mock"
		}
	}`))
	req.Header.Set("X-Location-Id", "loc-1")
	req.Header.Set(sessionIDHeader, "session-1")
	recorder := httptest.NewRecorder()

	server.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400, body = %s", recorder.Code, recorder.Body.String())
	}

	var payload ErrorResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	if payload.Error.Code != "underpaid_tender" {
		t.Fatalf("error code = %q, want underpaid_tender, body = %s", payload.Error.Code, recorder.Body.String())
	}
}

func seedOrderingService() *ordering.Service {
	service := ordering.NewService(ordering.MockProvider{})
	if err := service.RegisterSnapshot(recipe.MenuSnapshot{
		ID:         "snap-1",
		LocationID: "loc-1",
		Version:    1,
		Items: []recipe.SnapshotItem{
			{
				MenuItemID:      "bowl",
				Name:            "Bowl",
				PriceMinor:      1200,
				Currency:        "USD",
				Macros:          ingredient.MacroValues{Calories: 500, ProteinGrams: 40},
				IngredientUsage: map[string]float64{"chicken": 150},
				ModifierGroups: []recipe.SnapshotModifierGroup{
					{
						GroupID:      "protein",
						SelectionMin: 1,
						SelectionMax: 1,
						Required:     true,
						Exclusive:    true,
						Modifiers: []recipe.SnapshotModifier{
							{
								ModifierID:      "extra",
								Name:            "Extra",
								PriceDeltaMinor: 250,
								Currency:        "USD",
								MacroDelta:      ingredient.MacroValues{Calories: 120, ProteinGrams: 12},
								IngredientUsage: map[string]float64{"chicken": 50},
							},
						},
					},
				},
			},
		},
	}); err != nil {
		panic(err)
	}
	return service
}

func bizContext() context.Context {
	return context.Background()
}
