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

type fakeCatalogRepository struct {
	getMenuItemFn    func(context.Context, string, string) (recipe.MenuItem, error)
	listMenuItemsFn  func(context.Context, string) ([]recipe.MenuItem, error)
	createMenuItemFn func(context.Context, recipe.MenuItem) (recipe.MenuItem, error)
	updateMenuItemFn func(context.Context, recipe.MenuItem) (recipe.MenuItem, error)
	createSnapshotFn func(context.Context, recipe.MenuSnapshot) (recipe.MenuSnapshot, error)
	getSnapshotFn    func(context.Context, string, string) (recipe.MenuSnapshot, error)
	listSnapshotsFn  func(context.Context, string) ([]recipe.MenuSnapshot, error)
}

func (f fakeCatalogRepository) GetMenuItem(ctx context.Context, locationID string, menuItemID string) (recipe.MenuItem, error) {
	return f.getMenuItemFn(ctx, locationID, menuItemID)
}

func (f fakeCatalogRepository) ListMenuItems(ctx context.Context, locationID string) ([]recipe.MenuItem, error) {
	return f.listMenuItemsFn(ctx, locationID)
}

func (f fakeCatalogRepository) CreateMenuItem(ctx context.Context, item recipe.MenuItem) (recipe.MenuItem, error) {
	return f.createMenuItemFn(ctx, item)
}

func (f fakeCatalogRepository) UpdateMenuItem(ctx context.Context, item recipe.MenuItem) (recipe.MenuItem, error) {
	return f.updateMenuItemFn(ctx, item)
}

func (f fakeCatalogRepository) CreateSnapshot(ctx context.Context, snapshot recipe.MenuSnapshot) (recipe.MenuSnapshot, error) {
	return f.createSnapshotFn(ctx, snapshot)
}

func (f fakeCatalogRepository) GetSnapshot(ctx context.Context, locationID string, snapshotID string) (recipe.MenuSnapshot, error) {
	return f.getSnapshotFn(ctx, locationID, snapshotID)
}

func (f fakeCatalogRepository) ListSnapshots(ctx context.Context, locationID string) ([]recipe.MenuSnapshot, error) {
	return f.listSnapshotsFn(ctx, locationID)
}

type fakeCatalogSnapshotResolver struct {
	resolveRecipeFn   func(context.Context, string, string) (recipe.ResolvedRecipeData, error)
	resolveModifierFn func(context.Context, string, recipe.Modifier) (recipe.ResolvedModifierData, error)
}

func (f fakeCatalogSnapshotResolver) ResolveRecipe(ctx context.Context, locationID string, recipeID string) (recipe.ResolvedRecipeData, error) {
	return f.resolveRecipeFn(ctx, locationID, recipeID)
}

func (f fakeCatalogSnapshotResolver) ResolveModifier(ctx context.Context, locationID string, modifier recipe.Modifier) (recipe.ResolvedModifierData, error) {
	return f.resolveModifierFn(ctx, locationID, modifier)
}

func TestMenuItemCreateRouteCreatesMenuItem(t *testing.T) {
	service := recipe.NewCatalogService(fakeCatalogRepository{
		createMenuItemFn: func(_ context.Context, item recipe.MenuItem) (recipe.MenuItem, error) {
			return item, nil
		},
	}, fakeRecipeRepository{
		getFn: func(_ context.Context, locationID string, recipeID string) (recipe.Recipe, error) {
			return recipe.Recipe{ID: recipeID, LocationID: locationID, Name: "Chicken Bowl", YieldCount: 1}, nil
		},
	}, fakeRecipeIngredientLookup{
		getFn: func(_ context.Context, locationID string, ingredientID string) (ingredient.Ingredient, error) {
			return ingredient.Ingredient{ID: ingredientID, LocationID: locationID, BaseUnit: ingredient.UnitGram}, nil
		},
	}, nil)

	server := NewServerWithDependencies(slog.New(slog.NewTextHandler(io.Discard, nil)), Dependencies{
		CatalogService:       service,
		SessionValidator:     seedSessionService(t, "loc-1"),
		OrganizationResolver: tenancy.StaticOrganizationResolver{"loc-1": "org-1"},
	})

	req := httptest.NewRequest(http.MethodPost, "/menu-items", bytes.NewBufferString(`{
		"id":"item-1",
		"recipe_id":"rec-1",
		"name":"Chicken Bowl",
		"description":"Base bowl",
		"price_minor":995,
		"currency":"USD",
		"modifier_groups":[
			{
				"id":"grp-salsa",
				"name":"Salsa",
				"selection_min":1,
				"selection_max":1,
				"required":true,
				"exclusive":true,
				"default_modifier_ids":["mod-fresh-tomato"],
				"modifiers":[
					{
						"id":"mod-fresh-tomato",
						"name":"Fresh Tomato Salsa",
						"price_delta_minor":0,
						"currency":"USD",
						"ingredient_deltas":[
							{"ingredient_id":"ing-1","quantity":30,"unit":"g","prep_method":"fresh"}
						]
					}
				]
			}
		]
	}`))
	req.Header.Set("X-Location-Id", "loc-1")
	req.Header.Set(sessionIDHeader, "session-1")
	recorder := httptest.NewRecorder()

	server.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201, body = %s", recorder.Code, recorder.Body.String())
	}

	var payload struct {
		MenuItem menuItemResponse `json:"menu_item"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	if payload.MenuItem.ID != "item-1" {
		t.Fatalf("menu item id = %q, want item-1", payload.MenuItem.ID)
	}
	if len(payload.MenuItem.ModifierGroups) != 1 {
		t.Fatalf("modifier group count = %d, want 1", len(payload.MenuItem.ModifierGroups))
	}
}

func TestMenuItemListRouteListsMenuItems(t *testing.T) {
	service := recipe.NewCatalogService(fakeCatalogRepository{
		listMenuItemsFn: func(_ context.Context, locationID string) ([]recipe.MenuItem, error) {
			return []recipe.MenuItem{sampleMenuItem(locationID, "item-1")}, nil
		},
	}, fakeRecipeRepository{}, fakeRecipeIngredientLookup{}, nil)

	server := NewServerWithDependencies(slog.New(slog.NewTextHandler(io.Discard, nil)), Dependencies{
		CatalogService:       service,
		SessionValidator:     seedSessionService(t, "loc-1"),
		OrganizationResolver: tenancy.StaticOrganizationResolver{"loc-1": "org-1"},
	})

	req := httptest.NewRequest(http.MethodGet, "/menu-items", nil)
	req.Header.Set("X-Location-Id", "loc-1")
	req.Header.Set(sessionIDHeader, "session-1")
	recorder := httptest.NewRecorder()

	server.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body = %s", recorder.Code, recorder.Body.String())
	}

	var payload struct {
		MenuItems []menuItemResponse `json:"menu_items"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	if len(payload.MenuItems) != 1 {
		t.Fatalf("menu item count = %d, want 1", len(payload.MenuItems))
	}
	if payload.MenuItems[0].ID != "item-1" {
		t.Fatalf("menu item id = %q, want item-1", payload.MenuItems[0].ID)
	}
}

func TestMenuItemGetRouteReturnsMenuItem(t *testing.T) {
	service := recipe.NewCatalogService(fakeCatalogRepository{
		getMenuItemFn: func(_ context.Context, locationID string, menuItemID string) (recipe.MenuItem, error) {
			return sampleMenuItem(locationID, menuItemID), nil
		},
	}, fakeRecipeRepository{}, fakeRecipeIngredientLookup{}, nil)

	server := NewServerWithDependencies(slog.New(slog.NewTextHandler(io.Discard, nil)), Dependencies{
		CatalogService:       service,
		SessionValidator:     seedSessionService(t, "loc-1"),
		OrganizationResolver: tenancy.StaticOrganizationResolver{"loc-1": "org-1"},
	})

	req := httptest.NewRequest(http.MethodGet, "/menu-items/item-1", nil)
	req.Header.Set("X-Location-Id", "loc-1")
	req.Header.Set(sessionIDHeader, "session-1")
	recorder := httptest.NewRecorder()

	server.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body = %s", recorder.Code, recorder.Body.String())
	}

	var payload struct {
		MenuItem menuItemResponse `json:"menu_item"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	if payload.MenuItem.ID != "item-1" {
		t.Fatalf("menu item id = %q, want item-1", payload.MenuItem.ID)
	}
}

func TestMenuItemUpdateRouteUpdatesMenuItem(t *testing.T) {
	service := recipe.NewCatalogService(fakeCatalogRepository{
		updateMenuItemFn: func(_ context.Context, item recipe.MenuItem) (recipe.MenuItem, error) {
			return item, nil
		},
	}, fakeRecipeRepository{
		getFn: func(_ context.Context, locationID string, recipeID string) (recipe.Recipe, error) {
			return recipe.Recipe{ID: recipeID, LocationID: locationID, Name: "Chicken Bowl", YieldCount: 1}, nil
		},
	}, fakeRecipeIngredientLookup{
		getFn: func(_ context.Context, locationID string, ingredientID string) (ingredient.Ingredient, error) {
			return ingredient.Ingredient{ID: ingredientID, LocationID: locationID, BaseUnit: ingredient.UnitGram}, nil
		},
	}, nil)

	server := NewServerWithDependencies(slog.New(slog.NewTextHandler(io.Discard, nil)), Dependencies{
		CatalogService:       service,
		SessionValidator:     seedSessionService(t, "loc-1"),
		OrganizationResolver: tenancy.StaticOrganizationResolver{"loc-1": "org-1"},
	})

	req := httptest.NewRequest(http.MethodPut, "/menu-items/item-1", bytes.NewBufferString(`{
		"recipe_id":"rec-1",
		"name":"Updated Chicken Bowl",
		"description":"Updated",
		"price_minor":1095,
		"currency":"USD",
		"modifier_groups":[
			{
				"id":"grp-salsa",
				"name":"Salsa",
				"selection_min":1,
				"selection_max":1,
				"required":true,
				"exclusive":true,
				"default_modifier_ids":["mod-fresh-tomato"],
				"modifiers":[
					{
						"id":"mod-fresh-tomato",
						"name":"Fresh Tomato Salsa",
						"price_delta_minor":0,
						"currency":"USD",
						"ingredient_deltas":[
							{"ingredient_id":"ing-1","quantity":30,"unit":"g","prep_method":"fresh"}
						]
					}
				]
			}
		]
	}`))
	req.Header.Set("X-Location-Id", "loc-1")
	req.Header.Set(sessionIDHeader, "session-1")
	recorder := httptest.NewRecorder()

	server.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body = %s", recorder.Code, recorder.Body.String())
	}

	var payload struct {
		MenuItem menuItemResponse `json:"menu_item"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	if payload.MenuItem.Name != "Updated Chicken Bowl" {
		t.Fatalf("menu item name = %q, want Updated Chicken Bowl", payload.MenuItem.Name)
	}
	if payload.MenuItem.ID != "item-1" {
		t.Fatalf("menu item id = %q, want item-1", payload.MenuItem.ID)
	}
}

func TestMenuItemUpdateRouteRejectsMismatchedBodyID(t *testing.T) {
	service := recipe.NewCatalogService(fakeCatalogRepository{
		updateMenuItemFn: func(_ context.Context, item recipe.MenuItem) (recipe.MenuItem, error) {
			t.Fatal("UpdateMenuItem should not be called on mismatched id")
			return item, nil
		},
	}, fakeRecipeRepository{}, fakeRecipeIngredientLookup{}, nil)

	server := NewServerWithDependencies(slog.New(slog.NewTextHandler(io.Discard, nil)), Dependencies{
		CatalogService:       service,
		SessionValidator:     seedSessionService(t, "loc-1"),
		OrganizationResolver: tenancy.StaticOrganizationResolver{"loc-1": "org-1"},
	})

	req := httptest.NewRequest(http.MethodPut, "/menu-items/item-1", bytes.NewBufferString(`{
		"id":"item-2",
		"recipe_id":"rec-1",
		"name":"Updated Chicken Bowl",
		"price_minor":1095,
		"currency":"USD",
		"modifier_groups":[]
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
	if payload.Error.Code != "invalid_menu_item" {
		t.Fatalf("error code = %q, want invalid_menu_item", payload.Error.Code)
	}
}

func TestMenuItemCreateRouteRejectsInvalidModifierRules(t *testing.T) {
	service := recipe.NewCatalogService(fakeCatalogRepository{
		createMenuItemFn: func(_ context.Context, item recipe.MenuItem) (recipe.MenuItem, error) {
			return item, nil
		},
	}, fakeRecipeRepository{
		getFn: func(_ context.Context, locationID string, recipeID string) (recipe.Recipe, error) {
			return recipe.Recipe{ID: recipeID, LocationID: locationID, Name: "Chicken Bowl", YieldCount: 1}, nil
		},
	}, fakeRecipeIngredientLookup{
		getFn: func(_ context.Context, locationID string, ingredientID string) (ingredient.Ingredient, error) {
			return ingredient.Ingredient{ID: ingredientID, LocationID: locationID, BaseUnit: ingredient.UnitGram}, nil
		},
	}, nil)

	server := NewServerWithDependencies(slog.New(slog.NewTextHandler(io.Discard, nil)), Dependencies{
		CatalogService:       service,
		SessionValidator:     seedSessionService(t, "loc-1"),
		OrganizationResolver: tenancy.StaticOrganizationResolver{"loc-1": "org-1"},
	})

	req := httptest.NewRequest(http.MethodPost, "/menu-items", bytes.NewBufferString(`{
		"id":"item-1",
		"recipe_id":"rec-1",
		"name":"Chicken Bowl",
		"price_minor":995,
		"currency":"USD",
		"modifier_groups":[
			{
				"id":"grp-salsa",
				"name":"Salsa",
				"selection_min":1,
				"selection_max":2,
				"required":true,
				"exclusive":true,
				"default_modifier_ids":["mod-fresh-tomato"],
				"modifiers":[
					{
						"id":"mod-fresh-tomato",
						"name":"Fresh Tomato Salsa",
						"price_delta_minor":0,
						"currency":"USD",
						"ingredient_deltas":[
							{"ingredient_id":"ing-1","quantity":30,"unit":"g"}
						]
					}
				]
			}
		]
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
	if payload.Error.Code != "invalid_menu_item" {
		t.Fatalf("error code = %q, want invalid_menu_item", payload.Error.Code)
	}
}

func TestMenuItemCreateRouteRejectsUnknownFields(t *testing.T) {
	service := recipe.NewCatalogService(fakeCatalogRepository{}, fakeRecipeRepository{}, fakeRecipeIngredientLookup{}, nil)

	server := NewServerWithDependencies(slog.New(slog.NewTextHandler(io.Discard, nil)), Dependencies{
		CatalogService:       service,
		SessionValidator:     seedSessionService(t, "loc-1"),
		OrganizationResolver: tenancy.StaticOrganizationResolver{"loc-1": "org-1"},
	})

	req := httptest.NewRequest(http.MethodPost, "/menu-items", bytes.NewBufferString(`{
		"id":"item-1",
		"recipe_id":"rec-1",
		"name":"Chicken Bowl",
		"price_minor":995,
		"currency":"USD",
		"modifier_groups":[],
		"unexpected":true
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
	if payload.Error.Code != "invalid_json" {
		t.Fatalf("error code = %q, want invalid_json", payload.Error.Code)
	}
}

func TestMenuItemCreateRouteReturnsConflictOnDuplicate(t *testing.T) {
	service := recipe.NewCatalogService(fakeCatalogRepository{
		createMenuItemFn: func(_ context.Context, item recipe.MenuItem) (recipe.MenuItem, error) {
			return recipe.MenuItem{}, recipe.ErrMenuItemAlreadyExists
		},
	}, fakeRecipeRepository{
		getFn: func(_ context.Context, locationID string, recipeID string) (recipe.Recipe, error) {
			return recipe.Recipe{ID: recipeID, LocationID: locationID, Name: "Chicken Bowl", YieldCount: 1}, nil
		},
	}, fakeRecipeIngredientLookup{
		getFn: func(_ context.Context, locationID string, ingredientID string) (ingredient.Ingredient, error) {
			return ingredient.Ingredient{ID: ingredientID, LocationID: locationID, BaseUnit: ingredient.UnitGram}, nil
		},
	}, nil)

	server := NewServerWithDependencies(slog.New(slog.NewTextHandler(io.Discard, nil)), Dependencies{
		CatalogService:       service,
		SessionValidator:     seedSessionService(t, "loc-1"),
		OrganizationResolver: tenancy.StaticOrganizationResolver{"loc-1": "org-1"},
	})

	req := httptest.NewRequest(http.MethodPost, "/menu-items", bytes.NewBufferString(`{
		"id":"item-1",
		"recipe_id":"rec-1",
		"name":"Chicken Bowl",
		"price_minor":995,
		"currency":"USD",
		"modifier_groups":[]
	}`))
	req.Header.Set("X-Location-Id", "loc-1")
	req.Header.Set(sessionIDHeader, "session-1")
	recorder := httptest.NewRecorder()

	server.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409, body = %s", recorder.Code, recorder.Body.String())
	}
	var payload ErrorResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	if payload.Error.Code != "menu_item_already_exists" {
		t.Fatalf("error code = %q, want menu_item_already_exists", payload.Error.Code)
	}
}

func TestMenuItemGetRouteReturnsNotFound(t *testing.T) {
	service := recipe.NewCatalogService(fakeCatalogRepository{
		getMenuItemFn: func(_ context.Context, locationID string, menuItemID string) (recipe.MenuItem, error) {
			return recipe.MenuItem{}, recipe.ErrMenuItemNotFound
		},
	}, fakeRecipeRepository{}, fakeRecipeIngredientLookup{}, nil)

	server := NewServerWithDependencies(slog.New(slog.NewTextHandler(io.Discard, nil)), Dependencies{
		CatalogService:       service,
		SessionValidator:     seedSessionService(t, "loc-1"),
		OrganizationResolver: tenancy.StaticOrganizationResolver{"loc-1": "org-1"},
	})

	req := httptest.NewRequest(http.MethodGet, "/menu-items/item-missing", nil)
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
	if payload.Error.Code != "menu_item_not_found" {
		t.Fatalf("error code = %q, want menu_item_not_found", payload.Error.Code)
	}
}

func TestMenuSnapshotCreateRouteCreatesSnapshot(t *testing.T) {
	service := recipe.NewCatalogService(fakeCatalogRepository{
		listMenuItemsFn: func(_ context.Context, locationID string) ([]recipe.MenuItem, error) {
			return []recipe.MenuItem{sampleMenuItem(locationID, "item-1")}, nil
		},
		createSnapshotFn: func(_ context.Context, snapshot recipe.MenuSnapshot) (recipe.MenuSnapshot, error) {
			snapshot.Version = 1
			return snapshot, nil
		},
	}, fakeRecipeRepository{}, fakeRecipeIngredientLookup{}, fakeCatalogSnapshotResolver{
		resolveRecipeFn: func(_ context.Context, locationID string, recipeID string) (recipe.ResolvedRecipeData, error) {
			return recipe.ResolvedRecipeData{
				Macros:          ingredient.MacroValues{Calories: 500, ProteinGrams: 40},
				IngredientUsage: map[string]float64{"chicken": 150},
				IngredientUnits: map[string]ingredient.Unit{"chicken": ingredient.UnitGram},
			}, nil
		},
		resolveModifierFn: func(_ context.Context, locationID string, modifier recipe.Modifier) (recipe.ResolvedModifierData, error) {
			return recipe.ResolvedModifierData{
				MacroDelta:      ingredient.MacroValues{Calories: 120, ProteinGrams: 12},
				IngredientUsage: map[string]float64{"chicken": 50},
				IngredientUnits: map[string]ingredient.Unit{"chicken": ingredient.UnitGram},
			}, nil
		},
	})

	server := NewServerWithDependencies(slog.New(slog.NewTextHandler(io.Discard, nil)), Dependencies{
		CatalogService:       service,
		SessionValidator:     seedSessionService(t, "loc-1"),
		OrganizationResolver: tenancy.StaticOrganizationResolver{"loc-1": "org-1"},
	})

	req := httptest.NewRequest(http.MethodPost, "/menu-snapshots", bytes.NewBufferString(`{"id":"snap-1"}`))
	req.Header.Set("X-Location-Id", "loc-1")
	req.Header.Set(sessionIDHeader, "session-1")
	recorder := httptest.NewRecorder()

	server.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201, body = %s", recorder.Code, recorder.Body.String())
	}
	var payload struct {
		Snapshot menuSnapshotResponse `json:"snapshot"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	if payload.Snapshot.ID != "snap-1" {
		t.Fatalf("snapshot id = %q, want snap-1", payload.Snapshot.ID)
	}
	if payload.Snapshot.Items[0].Macros.Calories != 500 {
		t.Fatalf("snapshot calories = %v, want 500", payload.Snapshot.Items[0].Macros.Calories)
	}
}

func TestMenuSnapshotCreateRouteReturnsConflictOnDuplicate(t *testing.T) {
	service := recipe.NewCatalogService(fakeCatalogRepository{
		listMenuItemsFn: func(_ context.Context, locationID string) ([]recipe.MenuItem, error) {
			return []recipe.MenuItem{sampleMenuItem(locationID, "item-1")}, nil
		},
		createSnapshotFn: func(_ context.Context, snapshot recipe.MenuSnapshot) (recipe.MenuSnapshot, error) {
			return recipe.MenuSnapshot{}, recipe.ErrSnapshotAlreadyExists
		},
	}, fakeRecipeRepository{}, fakeRecipeIngredientLookup{}, fakeCatalogSnapshotResolver{
		resolveRecipeFn: func(_ context.Context, locationID string, recipeID string) (recipe.ResolvedRecipeData, error) {
			return recipe.ResolvedRecipeData{Macros: ingredient.MacroValues{Calories: 500}}, nil
		},
		resolveModifierFn: func(_ context.Context, locationID string, modifier recipe.Modifier) (recipe.ResolvedModifierData, error) {
			return recipe.ResolvedModifierData{}, nil
		},
	})

	server := NewServerWithDependencies(slog.New(slog.NewTextHandler(io.Discard, nil)), Dependencies{
		CatalogService:       service,
		SessionValidator:     seedSessionService(t, "loc-1"),
		OrganizationResolver: tenancy.StaticOrganizationResolver{"loc-1": "org-1"},
	})

	req := httptest.NewRequest(http.MethodPost, "/menu-snapshots", bytes.NewBufferString(`{"id":"snap-1"}`))
	req.Header.Set("X-Location-Id", "loc-1")
	req.Header.Set(sessionIDHeader, "session-1")
	recorder := httptest.NewRecorder()

	server.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409, body = %s", recorder.Code, recorder.Body.String())
	}
	var payload ErrorResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	if payload.Error.Code != "snapshot_already_exists" {
		t.Fatalf("error code = %q, want snapshot_already_exists", payload.Error.Code)
	}
}

func TestMenuSnapshotListRouteListsSnapshots(t *testing.T) {
	service := recipe.NewCatalogService(fakeCatalogRepository{
		listSnapshotsFn: func(_ context.Context, locationID string) ([]recipe.MenuSnapshot, error) {
			return []recipe.MenuSnapshot{{
				ID:         "snap-1",
				LocationID: locationID,
				Version:    1,
			}}, nil
		},
	}, fakeRecipeRepository{}, fakeRecipeIngredientLookup{}, nil)

	server := NewServerWithDependencies(slog.New(slog.NewTextHandler(io.Discard, nil)), Dependencies{
		CatalogService:       service,
		SessionValidator:     seedSessionService(t, "loc-1"),
		OrganizationResolver: tenancy.StaticOrganizationResolver{"loc-1": "org-1"},
	})

	req := httptest.NewRequest(http.MethodGet, "/menu-snapshots", nil)
	req.Header.Set("X-Location-Id", "loc-1")
	req.Header.Set(sessionIDHeader, "session-1")
	recorder := httptest.NewRecorder()

	server.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body = %s", recorder.Code, recorder.Body.String())
	}
	var payload struct {
		Snapshots []menuSnapshotSummaryResponse `json:"snapshots"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	if len(payload.Snapshots) != 1 {
		t.Fatalf("snapshot count = %d, want 1", len(payload.Snapshots))
	}
}

func TestMenuSnapshotGetRouteReturnsSnapshot(t *testing.T) {
	service := recipe.NewCatalogService(fakeCatalogRepository{
		getSnapshotFn: func(_ context.Context, locationID string, snapshotID string) (recipe.MenuSnapshot, error) {
			return sampleSnapshot(locationID, snapshotID), nil
		},
	}, fakeRecipeRepository{}, fakeRecipeIngredientLookup{}, nil)

	server := NewServerWithDependencies(slog.New(slog.NewTextHandler(io.Discard, nil)), Dependencies{
		CatalogService:       service,
		SessionValidator:     seedSessionService(t, "loc-1"),
		OrganizationResolver: tenancy.StaticOrganizationResolver{"loc-1": "org-1"},
	})

	req := httptest.NewRequest(http.MethodGet, "/menu-snapshots/snap-1", nil)
	req.Header.Set("X-Location-Id", "loc-1")
	req.Header.Set(sessionIDHeader, "session-1")
	recorder := httptest.NewRecorder()

	server.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body = %s", recorder.Code, recorder.Body.String())
	}
	var payload struct {
		Snapshot menuSnapshotResponse `json:"snapshot"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	if payload.Snapshot.ID != "snap-1" {
		t.Fatalf("snapshot id = %q, want snap-1", payload.Snapshot.ID)
	}
}

func TestMenuSnapshotGetRouteReturnsNotFound(t *testing.T) {
	service := recipe.NewCatalogService(fakeCatalogRepository{
		getSnapshotFn: func(_ context.Context, locationID string, snapshotID string) (recipe.MenuSnapshot, error) {
			return recipe.MenuSnapshot{}, recipe.ErrSnapshotNotFound
		},
	}, fakeRecipeRepository{}, fakeRecipeIngredientLookup{}, nil)

	server := NewServerWithDependencies(slog.New(slog.NewTextHandler(io.Discard, nil)), Dependencies{
		CatalogService:       service,
		SessionValidator:     seedSessionService(t, "loc-1"),
		OrganizationResolver: tenancy.StaticOrganizationResolver{"loc-1": "org-1"},
	})

	req := httptest.NewRequest(http.MethodGet, "/menu-snapshots/missing", nil)
	req.Header.Set("X-Location-Id", "loc-1")
	req.Header.Set(sessionIDHeader, "session-1")
	recorder := httptest.NewRecorder()

	server.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404, body = %s", recorder.Code, recorder.Body.String())
	}
}

func sampleMenuItem(locationID string, menuItemID string) recipe.MenuItem {
	return recipe.MenuItem{
		ID:          menuItemID,
		LocationID:  locationID,
		RecipeID:    "rec-1",
		Name:        "Chicken Bowl",
		Description: "Base bowl",
		PriceMinor:  995,
		Currency:    "USD",
		ModifierGroups: []recipe.ModifierGroup{{
			ID:                 "grp-salsa",
			Name:               "Salsa",
			SelectionMin:       1,
			SelectionMax:       1,
			Required:           true,
			Exclusive:          true,
			DefaultModifierIDs: []string{"mod-fresh-tomato"},
			Modifiers: []recipe.Modifier{{
				ID:              "mod-fresh-tomato",
				Name:            "Fresh Tomato Salsa",
				PriceDeltaMinor: 0,
				Currency:        "USD",
				IngredientDeltas: []recipe.IngredientDelta{{
					IngredientID: "ing-1",
					Quantity:     30,
					Unit:         ingredient.UnitGram,
					PrepMethod:   "fresh",
				}},
			}},
		}},
	}
}

func sampleSnapshot(locationID string, snapshotID string) recipe.MenuSnapshot {
	return recipe.MenuSnapshot{
		ID:         snapshotID,
		LocationID: locationID,
		Version:    1,
		Items: []recipe.SnapshotItem{{
			MenuItemID:      "item-1",
			Name:            "Chicken Bowl",
			Description:     "Base bowl",
			PriceMinor:      995,
			Currency:        "USD",
			Macros:          ingredient.MacroValues{Calories: 500, ProteinGrams: 40},
			IngredientUsage: map[string]float64{"chicken": 150},
			IngredientUnits: map[string]ingredient.Unit{"chicken": ingredient.UnitGram},
			ModifierGroups: []recipe.SnapshotModifierGroup{{
				GroupID:            "grp-salsa",
				Name:               "Salsa",
				SelectionMin:       1,
				SelectionMax:       1,
				Required:           true,
				Exclusive:          true,
				DefaultModifierIDs: []string{"mod-fresh-tomato"},
				Modifiers: []recipe.SnapshotModifier{{
					ModifierID:      "mod-fresh-tomato",
					Name:            "Fresh Tomato Salsa",
					PriceDeltaMinor: 0,
					Currency:        "USD",
					MacroDelta:      ingredient.MacroValues{Calories: 20},
					IngredientUsage: map[string]float64{"tomato": 30},
					IngredientUnits: map[string]ingredient.Unit{"tomato": ingredient.UnitGram},
				}},
			}},
		}},
	}
}
