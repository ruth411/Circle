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
	"time"

	"github.com/ruth411/circle/internal/core/ingredient"
	"github.com/ruth411/circle/internal/identity"
	"github.com/ruth411/circle/internal/tenancy"
)

func TestIngredientListRouteReturnsTenantScopedResults(t *testing.T) {
	service := ingredient.NewService(fakeIngredientRepository{
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
		"current_cost_minor":1299,
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
}

func TestIngredientUpdateRouteReturnsNotFound(t *testing.T) {
	service := ingredient.NewService(fakeIngredientRepository{
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

type fakeIngredientRepository struct {
	listFn   func(string, string) ([]ingredient.Ingredient, error)
	createFn func(ingredient.Ingredient) (ingredient.Ingredient, error)
	updateFn func(ingredient.Ingredient) (ingredient.Ingredient, error)
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
