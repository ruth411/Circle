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
	"github.com/ruth411/circle/internal/tenancy"
)

type fakeRecipeRepository struct {
	getFn    func(context.Context, string, string) (recipe.Recipe, error)
	listFn   func(context.Context, string) ([]recipe.Recipe, error)
	createFn func(context.Context, recipe.Recipe) (recipe.Recipe, error)
	updateFn func(context.Context, recipe.Recipe) (recipe.Recipe, error)
}

func (f fakeRecipeRepository) Get(ctx context.Context, locationID string, recipeID string) (recipe.Recipe, error) {
	return f.getFn(ctx, locationID, recipeID)
}

func (f fakeRecipeRepository) List(ctx context.Context, locationID string) ([]recipe.Recipe, error) {
	return f.listFn(ctx, locationID)
}

func (f fakeRecipeRepository) Create(ctx context.Context, value recipe.Recipe) (recipe.Recipe, error) {
	return f.createFn(ctx, value)
}

func (f fakeRecipeRepository) Update(ctx context.Context, value recipe.Recipe) (recipe.Recipe, error) {
	return f.updateFn(ctx, value)
}

type fakeRecipeIngredientLookup struct {
	getFn func(context.Context, string, string) (ingredient.Ingredient, error)
}

func (f fakeRecipeIngredientLookup) Get(ctx context.Context, locationID string, ingredientID string) (ingredient.Ingredient, error) {
	return f.getFn(ctx, locationID, ingredientID)
}

func TestRecipeCreateRouteCreatesRecipe(t *testing.T) {
	service := recipe.NewService(fakeRecipeRepository{
		createFn: func(_ context.Context, value recipe.Recipe) (recipe.Recipe, error) {
			return value, nil
		},
	}, fakeRecipeIngredientLookup{
		getFn: func(_ context.Context, locationID string, ingredientID string) (ingredient.Ingredient, error) {
			return ingredient.Ingredient{ID: ingredientID, LocationID: locationID, BaseUnit: ingredient.UnitGram}, nil
		},
	})

	server := NewServerWithDependencies(slog.New(slog.NewTextHandler(io.Discard, nil)), Dependencies{
		RecipeService:        service,
		SessionValidator:     seedSessionService(t, "loc-1"),
		OrganizationResolver: tenancy.StaticOrganizationResolver{"loc-1": "org-1"},
	})

	req := httptest.NewRequest(http.MethodPost, "/recipes", bytes.NewBufferString(`{
		"id":"rec-1",
		"name":"Chicken Prep",
		"yield_count":2,
		"lines":[{"target_type":"ingredient","target_id":"ing-1","quantity":500,"unit":"g","prep_method":"grilled"}]
	}`))
	req.Header.Set("X-Location-Id", "loc-1")
	req.Header.Set(sessionIDHeader, "session-1")
	recorder := httptest.NewRecorder()

	server.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201, body = %s", recorder.Code, recorder.Body.String())
	}
	var payload struct {
		Recipe recipeResponse `json:"recipe"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	if payload.Recipe.ID != "rec-1" {
		t.Fatalf("recipe id = %q, want rec-1", payload.Recipe.ID)
	}
	if payload.Recipe.LocationID != "loc-1" {
		t.Fatalf("location id = %q, want loc-1", payload.Recipe.LocationID)
	}
}

func TestRecipeListRouteListsRecipes(t *testing.T) {
	service := recipe.NewService(fakeRecipeRepository{
		listFn: func(_ context.Context, locationID string) ([]recipe.Recipe, error) {
			return []recipe.Recipe{{
				ID:         "rec-1",
				LocationID: locationID,
				Name:       "Chicken Prep",
				YieldCount: 2,
				Lines: []recipe.RecipeLine{{
					LineNumber: 1,
					TargetType: recipe.LineTargetIngredient,
					TargetID:   "ing-1",
					Quantity:   500,
					Unit:       ingredient.UnitGram,
					PrepMethod: "grilled",
				}},
			}}, nil
		},
	}, fakeRecipeIngredientLookup{})

	server := NewServerWithDependencies(slog.New(slog.NewTextHandler(io.Discard, nil)), Dependencies{
		RecipeService:        service,
		SessionValidator:     seedSessionService(t, "loc-1"),
		OrganizationResolver: tenancy.StaticOrganizationResolver{"loc-1": "org-1"},
	})

	req := httptest.NewRequest(http.MethodGet, "/recipes", nil)
	req.Header.Set("X-Location-Id", "loc-1")
	req.Header.Set(sessionIDHeader, "session-1")
	recorder := httptest.NewRecorder()

	server.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body = %s", recorder.Code, recorder.Body.String())
	}
	var payload struct {
		Recipes []recipeResponse `json:"recipes"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	if len(payload.Recipes) != 1 {
		t.Fatalf("recipe count = %d, want 1", len(payload.Recipes))
	}
	if payload.Recipes[0].ID != "rec-1" {
		t.Fatalf("recipe id = %q, want rec-1", payload.Recipes[0].ID)
	}
}

func TestRecipeGetRouteReturnsRecipe(t *testing.T) {
	service := recipe.NewService(fakeRecipeRepository{
		getFn: func(_ context.Context, locationID string, recipeID string) (recipe.Recipe, error) {
			return recipe.Recipe{
				ID:         recipeID,
				LocationID: locationID,
				Name:       "Chicken Prep",
				YieldCount: 2,
				Lines: []recipe.RecipeLine{{
					LineNumber: 1,
					TargetType: recipe.LineTargetIngredient,
					TargetID:   "ing-1",
					Quantity:   500,
					Unit:       ingredient.UnitGram,
					PrepMethod: "grilled",
				}},
			}, nil
		},
	}, fakeRecipeIngredientLookup{})

	server := NewServerWithDependencies(slog.New(slog.NewTextHandler(io.Discard, nil)), Dependencies{
		RecipeService:        service,
		SessionValidator:     seedSessionService(t, "loc-1"),
		OrganizationResolver: tenancy.StaticOrganizationResolver{"loc-1": "org-1"},
	})

	req := httptest.NewRequest(http.MethodGet, "/recipes/rec-1", nil)
	req.Header.Set("X-Location-Id", "loc-1")
	req.Header.Set(sessionIDHeader, "session-1")
	recorder := httptest.NewRecorder()

	server.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body = %s", recorder.Code, recorder.Body.String())
	}
	var payload struct {
		Recipe recipeResponse `json:"recipe"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	if payload.Recipe.ID != "rec-1" {
		t.Fatalf("recipe id = %q, want rec-1", payload.Recipe.ID)
	}
}

func TestRecipeUpdateRouteUpdatesRecipe(t *testing.T) {
	service := recipe.NewService(fakeRecipeRepository{
		updateFn: func(_ context.Context, value recipe.Recipe) (recipe.Recipe, error) {
			return value, nil
		},
	}, fakeRecipeIngredientLookup{
		getFn: func(_ context.Context, locationID string, ingredientID string) (ingredient.Ingredient, error) {
			return ingredient.Ingredient{ID: ingredientID, LocationID: locationID, BaseUnit: ingredient.UnitGram}, nil
		},
	})

	server := NewServerWithDependencies(slog.New(slog.NewTextHandler(io.Discard, nil)), Dependencies{
		RecipeService:        service,
		SessionValidator:     seedSessionService(t, "loc-1"),
		OrganizationResolver: tenancy.StaticOrganizationResolver{"loc-1": "org-1"},
	})

	req := httptest.NewRequest(http.MethodPut, "/recipes/rec-1", bytes.NewBufferString(`{
		"name":"Updated Chicken Prep",
		"yield_count":3,
		"lines":[{"target_type":"ingredient","target_id":"ing-1","quantity":750,"unit":"g","prep_method":"grilled"}]
	}`))
	req.Header.Set("X-Location-Id", "loc-1")
	req.Header.Set(sessionIDHeader, "session-1")
	recorder := httptest.NewRecorder()

	server.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body = %s", recorder.Code, recorder.Body.String())
	}
	var payload struct {
		Recipe recipeResponse `json:"recipe"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	if payload.Recipe.Name != "Updated Chicken Prep" {
		t.Fatalf("recipe name = %q, want Updated Chicken Prep", payload.Recipe.Name)
	}
	if payload.Recipe.ID != "rec-1" {
		t.Fatalf("recipe id = %q, want rec-1", payload.Recipe.ID)
	}
}

func TestRecipeUpdateRouteRejectsMismatchedBodyID(t *testing.T) {
	service := recipe.NewService(fakeRecipeRepository{
		updateFn: func(_ context.Context, value recipe.Recipe) (recipe.Recipe, error) {
			t.Fatal("Update should not be called on mismatched id")
			return value, nil
		},
	}, fakeRecipeIngredientLookup{
		getFn: func(_ context.Context, locationID string, ingredientID string) (ingredient.Ingredient, error) {
			return ingredient.Ingredient{ID: ingredientID, LocationID: locationID, BaseUnit: ingredient.UnitGram}, nil
		},
	})

	server := NewServerWithDependencies(slog.New(slog.NewTextHandler(io.Discard, nil)), Dependencies{
		RecipeService:        service,
		SessionValidator:     seedSessionService(t, "loc-1"),
		OrganizationResolver: tenancy.StaticOrganizationResolver{"loc-1": "org-1"},
	})

	req := httptest.NewRequest(http.MethodPut, "/recipes/rec-1", bytes.NewBufferString(`{
		"id":"rec-2",
		"name":"Updated Chicken Prep",
		"yield_count":3,
		"lines":[{"target_type":"ingredient","target_id":"ing-1","quantity":750,"unit":"g","prep_method":"grilled"}]
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
	if payload.Error.Code != "invalid_recipe" {
		t.Fatalf("error code = %q, want invalid_recipe", payload.Error.Code)
	}
}

func TestRecipeCreateRouteRejectsUnknownFields(t *testing.T) {
	service := recipe.NewService(fakeRecipeRepository{
		createFn: func(_ context.Context, value recipe.Recipe) (recipe.Recipe, error) {
			return value, nil
		},
	}, fakeRecipeIngredientLookup{
		getFn: func(_ context.Context, locationID string, ingredientID string) (ingredient.Ingredient, error) {
			return ingredient.Ingredient{ID: ingredientID, LocationID: locationID, BaseUnit: ingredient.UnitGram}, nil
		},
	})

	server := NewServerWithDependencies(slog.New(slog.NewTextHandler(io.Discard, nil)), Dependencies{
		RecipeService:        service,
		SessionValidator:     seedSessionService(t, "loc-1"),
		OrganizationResolver: tenancy.StaticOrganizationResolver{"loc-1": "org-1"},
	})

	req := httptest.NewRequest(http.MethodPost, "/recipes", bytes.NewBufferString(`{
		"id":"rec-1",
		"name":"Chicken Prep",
		"yield_count":2,
		"lines":[{"target_type":"ingredient","target_id":"ing-1","quantity":500,"unit":"g"}],
		"unexpected":true
	}`))
	req.Header.Set("X-Location-Id", "loc-1")
	req.Header.Set(sessionIDHeader, "session-1")
	recorder := httptest.NewRecorder()

	server.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestRecipeGetRouteReturnsNotFound(t *testing.T) {
	service := recipe.NewService(fakeRecipeRepository{
		getFn: func(_ context.Context, locationID string, recipeID string) (recipe.Recipe, error) {
			return recipe.Recipe{}, recipe.ErrRecipeNotFound
		},
	}, fakeRecipeIngredientLookup{})

	server := NewServerWithDependencies(slog.New(slog.NewTextHandler(io.Discard, nil)), Dependencies{
		RecipeService:        service,
		SessionValidator:     seedSessionService(t, "loc-1"),
		OrganizationResolver: tenancy.StaticOrganizationResolver{"loc-1": "org-1"},
	})

	req := httptest.NewRequest(http.MethodGet, "/recipes/missing", nil)
	req.Header.Set("X-Location-Id", "loc-1")
	req.Header.Set(sessionIDHeader, "session-1")
	recorder := httptest.NewRecorder()

	server.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404, body = %s", recorder.Code, recorder.Body.String())
	}
	var payload ErrorResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	if payload.Error.Code != "recipe_not_found" {
		t.Fatalf("error code = %q, want recipe_not_found", payload.Error.Code)
	}
}
