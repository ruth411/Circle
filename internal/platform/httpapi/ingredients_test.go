package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ruth411/circle/internal/core/ingredient"
	"github.com/ruth411/circle/internal/identity"
	"github.com/ruth411/circle/internal/inventory"
	"github.com/ruth411/circle/internal/tenancy"
)

func TestIngredientListRouteReturnsTenantScopedResults(t *testing.T) {
	service := ingredient.NewService(fakeIngredientRepository{
		getFn: func(locationID string, ingredientID string) (ingredient.Ingredient, error) {
			return ingredient.Ingredient{}, ingredient.ErrNotFound
		},
		listFn: func(locationID string, search string) ([]ingredient.Ingredient, error) {
			if locationID != "loc-1" {
				t.Fatalf("locationID = %q, want loc-1", locationID)
			}
			if search != "chicken" {
				t.Fatalf("search = %q, want chicken", search)
			}
			return []ingredient.Ingredient{
				{
					ID:                  "ing-1",
					LocationID:          "loc-1",
					Name:                "Chicken",
					Category:            "protein",
					BaseUnit:            ingredient.UnitEach,
					MacrosPerBaseUnit:   ingredient.MacroValues{Calories: 180},
					Currency:            "USD",
					Provenance:          ingredient.ProvenanceRestaurantOfficial,
					VerificationStatus:  ingredient.VerificationUnverified,
					ServingSizeQuantity: 4,
					ServingSizeUnit:     "oz",
				},
			}, nil
		},
	})

	identityService := seedSessionService(t, "loc-1")
	server := NewServerWithDependencies(slog.New(slog.NewTextHandler(io.Discard, nil)), Dependencies{
		IngredientService:    service,
		SessionValidator:     identityService,
		OrganizationResolver: tenancy.StaticOrganizationResolver{"loc-1": "org-1"},
	})

	req := httptest.NewRequest(http.MethodGet, "/ingredients?q=chicken", nil)
	req.Header.Set("X-Location-Id", "loc-1")
	req.Header.Set(sessionIDHeader, "session-1")
	recorder := httptest.NewRecorder()

	server.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}

	var payload struct {
		Ingredients []ingredientResponse `json:"ingredients"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	if len(payload.Ingredients) != 1 {
		t.Fatalf("ingredient count = %d, want 1", len(payload.Ingredients))
	}
	if !payload.Ingredients[0].LowConfidence {
		t.Fatal("LowConfidence = false, want true")
	}
}

func TestIngredientCreateRouteCreatesIngredient(t *testing.T) {
	service := ingredient.NewService(fakeIngredientRepository{
		getFn: func(locationID string, ingredientID string) (ingredient.Ingredient, error) {
			return ingredient.Ingredient{}, ingredient.ErrNotFound
		},
		createFn: func(value ingredient.Ingredient) (ingredient.Ingredient, error) {
			return value, nil
		},
	})

	identityService := seedSessionService(t, "loc-1")
	server := NewServerWithDependencies(slog.New(slog.NewTextHandler(io.Discard, nil)), Dependencies{
		IngredientService:    service,
		SessionValidator:     identityService,
		OrganizationResolver: tenancy.StaticOrganizationResolver{"loc-1": "org-1"},
	})

	body := bytes.NewBufferString(`{
		"id":"ing-1",
		"name":"Chicken",
		"category":"protein",
		"base_unit":"each",
		"macros_per_base_unit":{"calories":180,"protein_grams":32,"carbs_grams":0,"fat_grams":7},
		"current_cost_per_base_unit":12.3456,
		"currency":"USD",
		"on_hand_base_units":10,
		"par_level_base_units":4,
		"provenance":"restaurant_official",
		"verification_status":"verified",
		"serving_size_quantity":4,
		"serving_size_unit":"oz",
		"alternate_units":{"g":28.35},
		"yield_factors":{"cooked":0.84}
	}`)
	req := httptest.NewRequest(http.MethodPost, "/ingredients", body)
	req.Header.Set("X-Location-Id", "loc-1")
	req.Header.Set(sessionIDHeader, "session-1")
	recorder := httptest.NewRecorder()

	server.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", recorder.Code)
	}

	var payload struct {
		Ingredient ingredientResponse `json:"ingredient"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	if got, want := payload.Ingredient.CurrentCostPerBaseUnit, 12.3456; got != want {
		t.Fatalf("current cost per base unit = %v, want %v", got, want)
	}
}

func TestIngredientCreateRouteRoundsCostToFourDecimals(t *testing.T) {
	service := ingredient.NewService(fakeIngredientRepository{
		getFn: func(locationID string, ingredientID string) (ingredient.Ingredient, error) {
			return ingredient.Ingredient{}, ingredient.ErrNotFound
		},
		createFn: func(value ingredient.Ingredient) (ingredient.Ingredient, error) {
			return value, nil
		},
	})

	identityService := seedSessionService(t, "loc-1")
	server := NewServerWithDependencies(slog.New(slog.NewTextHandler(io.Discard, nil)), Dependencies{
		IngredientService:    service,
		SessionValidator:     identityService,
		OrganizationResolver: tenancy.StaticOrganizationResolver{"loc-1": "org-1"},
	})

	body := bytes.NewBufferString(`{
		"id":"ing-1",
		"name":"Chicken",
		"category":"protein",
		"base_unit":"each",
		"macros_per_base_unit":{"calories":180,"protein_grams":32,"carbs_grams":0,"fat_grams":7},
		"current_cost_per_base_unit":1.45678999,
		"currency":"USD",
		"on_hand_base_units":10,
		"par_level_base_units":4,
		"provenance":"restaurant_official",
		"verification_status":"verified",
		"serving_size_quantity":4,
		"serving_size_unit":"oz"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/ingredients", body)
	req.Header.Set("X-Location-Id", "loc-1")
	req.Header.Set(sessionIDHeader, "session-1")
	recorder := httptest.NewRecorder()

	server.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", recorder.Code)
	}

	var payload struct {
		Ingredient ingredientResponse `json:"ingredient"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	if got, want := payload.Ingredient.CurrentCostPerBaseUnit, 1.4568; got != want {
		t.Fatalf("current cost per base unit = %v, want %v", got, want)
	}
}

func TestIngredientUpdateRouteReturnsNotFound(t *testing.T) {
	service := ingredient.NewService(fakeIngredientRepository{
		getFn: func(locationID string, ingredientID string) (ingredient.Ingredient, error) {
			return ingredient.Ingredient{}, ingredient.ErrNotFound
		},
		updateFn: func(value ingredient.Ingredient) (ingredient.Ingredient, error) {
			return ingredient.Ingredient{}, ingredient.ErrNotFound
		},
	})

	identityService := seedSessionService(t, "loc-1")
	server := NewServerWithDependencies(slog.New(slog.NewTextHandler(io.Discard, nil)), Dependencies{
		IngredientService:    service,
		SessionValidator:     identityService,
		OrganizationResolver: tenancy.StaticOrganizationResolver{"loc-1": "org-1"},
	})

	body := bytes.NewBufferString(`{
		"id":"ing-missing",
		"name":"Chicken",
		"category":"protein",
		"base_unit":"each",
		"macros_per_base_unit":{"calories":180,"protein_grams":32,"carbs_grams":0,"fat_grams":7},
		"currency":"USD",
		"provenance":"restaurant_official",
		"verification_status":"verified",
		"serving_size_quantity":4,
		"serving_size_unit":"oz"
	}`)
	req := httptest.NewRequest(http.MethodPut, "/ingredients/ing-missing", body)
	req.Header.Set("X-Location-Id", "loc-1")
	req.Header.Set(sessionIDHeader, "session-1")
	recorder := httptest.NewRecorder()

	server.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", recorder.Code)
	}
}

func TestIngredientCreateRouteRejectsOversizedBody(t *testing.T) {
	service := ingredient.NewService(fakeIngredientRepository{
		getFn: func(locationID string, ingredientID string) (ingredient.Ingredient, error) {
			return ingredient.Ingredient{}, ingredient.ErrNotFound
		},
		createFn: func(value ingredient.Ingredient) (ingredient.Ingredient, error) {
			return value, nil
		},
	})

	identityService := seedSessionService(t, "loc-1")
	server := NewServerWithDependencies(slog.New(slog.NewTextHandler(io.Discard, nil)), Dependencies{
		IngredientService:    service,
		SessionValidator:     identityService,
		OrganizationResolver: tenancy.StaticOrganizationResolver{"loc-1": "org-1"},
	})

	body := `{"id":"ing-1","name":"` + strings.Repeat("x", int(maxIngredientRequestBodyBytes)) + `","category":"protein","base_unit":"each","macros_per_base_unit":{"calories":180},"currency":"USD","provenance":"restaurant_official","verification_status":"verified","serving_size_quantity":4,"serving_size_unit":"oz"}`
	req := httptest.NewRequest(http.MethodPost, "/ingredients", strings.NewReader(body))
	req.Header.Set("X-Location-Id", "loc-1")
	req.Header.Set(sessionIDHeader, "session-1")
	recorder := httptest.NewRecorder()

	server.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", recorder.Code)
	}
}

func TestIngredientResolvedRouteReturnsCombinedIngredientView(t *testing.T) {
	service := ingredient.NewService(fakeIngredientRepository{
		getFn: func(locationID string, ingredientID string) (ingredient.Ingredient, error) {
			if locationID != "loc-1" {
				t.Fatalf("locationID = %q, want loc-1", locationID)
			}
			if ingredientID != "ing-1" {
				t.Fatalf("ingredientID = %q, want ing-1", ingredientID)
			}
			receivedAt := time.Date(2026, 8, 2, 14, 0, 0, 0, time.UTC)
			return ingredient.Ingredient{
				ID:                          "ing-1",
				LocationID:                  "loc-1",
				Name:                        "Chicken",
				Category:                    "protein",
				BaseUnit:                    ingredient.UnitGram,
				MacrosPerBaseUnit:           ingredient.MacroValues{Calories: 1.8, ProteinGrams: 0.32, FatGrams: 0.07},
				CurrentCostPerBaseUnit:      ingredient.MustCostPerBaseUnit(0.1234),
				LastReceivedCostPerBaseUnit: ingredient.MustCostPerBaseUnit(0.1111),
				LastReceivedAt:              &receivedAt,
				Currency:                    "USD",
				Provenance:                  ingredient.ProvenanceRestaurantOfficial,
				VerificationStatus:          ingredient.VerificationVerified,
				ServingSizeQuantity:         100,
				ServingSizeUnit:             "g",
			}, nil
		},
		listFn: func(locationID string, search string) ([]ingredient.Ingredient, error) {
			t.Fatal("List should not be called")
			return nil, nil
		},
	})
	inventoryService := inventory.NewService(fakeInventoryRepository{
		onHandFn: func(_ context.Context, locationID string) ([]inventory.OnHandItem, error) {
			if locationID != "loc-1" {
				t.Fatalf("locationID = %q, want loc-1", locationID)
			}
			return []inventory.OnHandItem{{
				LocationID:     "loc-1",
				IngredientID:   "ing-1",
				IngredientName: "Chicken",
				BaseUnit:       ingredient.UnitGram,
				OnHandQuantity: 2400,
			}}, nil
		},
	})

	server := NewServerWithDependencies(slog.New(slog.NewTextHandler(io.Discard, nil)), Dependencies{
		IngredientService:    service,
		InventoryService:     inventoryService,
		SessionValidator:     seedSessionService(t, "loc-1"),
		OrganizationResolver: tenancy.StaticOrganizationResolver{"loc-1": "org-1"},
	})

	req := httptest.NewRequest(http.MethodGet, "/ingredients/ing-1/resolved", nil)
	req.Header.Set("X-Location-Id", "loc-1")
	req.Header.Set(sessionIDHeader, "session-1")
	recorder := httptest.NewRecorder()

	server.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body = %s", recorder.Code, recorder.Body.String())
	}

	var payload struct {
		Ingredient struct {
			ID                          string       `json:"id"`
			OnHandBaseUnits             float64      `json:"on_hand_base_units"`
			CurrentCostPerBaseUnit      float64      `json:"current_cost_per_base_unit"`
			LastReceivedCostPerBaseUnit float64      `json:"last_received_cost_per_base_unit"`
			LastReceivedAt              *time.Time   `json:"last_received_at"`
			MacrosPerBaseUnit           macroPayload `json:"macros_per_base_unit"`
		} `json:"ingredient"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	if payload.Ingredient.ID != "ing-1" {
		t.Fatalf("id = %q, want ing-1", payload.Ingredient.ID)
	}
	if payload.Ingredient.OnHandBaseUnits != 2400 {
		t.Fatalf("on hand = %v, want 2400", payload.Ingredient.OnHandBaseUnits)
	}
	if payload.Ingredient.CurrentCostPerBaseUnit != 0.1234 {
		t.Fatalf("current cost = %v, want 0.1234", payload.Ingredient.CurrentCostPerBaseUnit)
	}
	if payload.Ingredient.LastReceivedCostPerBaseUnit != 0.1111 {
		t.Fatalf("last received cost = %v, want 0.1111", payload.Ingredient.LastReceivedCostPerBaseUnit)
	}
	if payload.Ingredient.LastReceivedAt == nil {
		t.Fatal("last received at = nil, want timestamp")
	}
	if payload.Ingredient.MacrosPerBaseUnit.ProteinGrams != 0.32 {
		t.Fatalf("protein = %v, want 0.32", payload.Ingredient.MacrosPerBaseUnit.ProteinGrams)
	}
}

type fakeIngredientRepository struct {
	getFn    func(string, string) (ingredient.Ingredient, error)
	listFn   func(string, string) ([]ingredient.Ingredient, error)
	createFn func(ingredient.Ingredient) (ingredient.Ingredient, error)
	updateFn func(ingredient.Ingredient) (ingredient.Ingredient, error)
}

func (f fakeIngredientRepository) Get(_ context.Context, locationID string, ingredientID string) (ingredient.Ingredient, error) {
	return f.getFn(locationID, ingredientID)
}

func (f fakeIngredientRepository) List(_ context.Context, locationID string, search string) ([]ingredient.Ingredient, error) {
	return f.listFn(locationID, search)
}

func (f fakeIngredientRepository) Create(_ context.Context, value ingredient.Ingredient) (ingredient.Ingredient, error) {
	return f.createFn(value)
}

func (f fakeIngredientRepository) Update(_ context.Context, value ingredient.Ingredient) (ingredient.Ingredient, error) {
	return f.updateFn(value)
}

func seedSessionService(t *testing.T, locationID string) *identity.Service {
	t.Helper()

	service := identity.NewService()
	if err := service.AddUser(identity.User{
		ID:             "user-1",
		OrganizationID: "org-1",
		LocationID:     locationID,
		Email:          "staff@example.com",
		DisplayName:    "Staff",
		PasswordHash:   "hash",
	}); err != nil {
		t.Fatalf("AddUser returned error: %v", err)
	}
	if _, err := service.IssueSession("session-1", "user-1", time.Hour); err != nil {
		t.Fatalf("IssueSession returned error: %v", err)
	}

	return service
}
