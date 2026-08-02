package recipe

import (
	"context"
	"errors"
	"testing"

	"github.com/ruth411/circle/internal/core/ingredient"
)

type fakeCatalogRepository struct {
	getMenuItemFn    func(context.Context, string, string) (MenuItem, error)
	listMenuItemsFn  func(context.Context, string) ([]MenuItem, error)
	createMenuItemFn func(context.Context, MenuItem) (MenuItem, error)
	updateMenuItemFn func(context.Context, MenuItem) (MenuItem, error)
	createSnapshotFn func(context.Context, MenuSnapshot) (MenuSnapshot, error)
	getSnapshotFn    func(context.Context, string, string) (MenuSnapshot, error)
	listSnapshotsFn  func(context.Context, string) ([]MenuSnapshot, error)
}

func (f fakeCatalogRepository) GetMenuItem(ctx context.Context, locationID string, menuItemID string) (MenuItem, error) {
	return f.getMenuItemFn(ctx, locationID, menuItemID)
}

func (f fakeCatalogRepository) ListMenuItems(ctx context.Context, locationID string) ([]MenuItem, error) {
	return f.listMenuItemsFn(ctx, locationID)
}

func (f fakeCatalogRepository) CreateMenuItem(ctx context.Context, item MenuItem) (MenuItem, error) {
	return f.createMenuItemFn(ctx, item)
}

func (f fakeCatalogRepository) UpdateMenuItem(ctx context.Context, item MenuItem) (MenuItem, error) {
	return f.updateMenuItemFn(ctx, item)
}

func (f fakeCatalogRepository) CreateSnapshot(ctx context.Context, snapshot MenuSnapshot) (MenuSnapshot, error) {
	return f.createSnapshotFn(ctx, snapshot)
}

func (f fakeCatalogRepository) GetSnapshot(ctx context.Context, locationID string, snapshotID string) (MenuSnapshot, error) {
	return f.getSnapshotFn(ctx, locationID, snapshotID)
}

func (f fakeCatalogRepository) ListSnapshots(ctx context.Context, locationID string) ([]MenuSnapshot, error) {
	return f.listSnapshotsFn(ctx, locationID)
}

type fakeSnapshotResolver struct {
	resolveRecipeFn   func(context.Context, string, string) (ResolvedRecipeData, error)
	resolveModifierFn func(context.Context, string, Modifier) (ResolvedModifierData, error)
}

func (f fakeSnapshotResolver) ResolveRecipe(ctx context.Context, locationID string, recipeID string) (ResolvedRecipeData, error) {
	return f.resolveRecipeFn(ctx, locationID, recipeID)
}

func (f fakeSnapshotResolver) ResolveModifier(ctx context.Context, locationID string, modifier Modifier) (ResolvedModifierData, error) {
	return f.resolveModifierFn(ctx, locationID, modifier)
}

func TestCreateMenuItemRejectsPriceOnlyModifier(t *testing.T) {
	service := NewCatalogService(
		fakeCatalogRepository{
			createMenuItemFn: func(_ context.Context, item MenuItem) (MenuItem, error) {
				return item, nil
			},
		},
		fakeRecipeRepository{
			getFn: func(_ context.Context, locationID string, recipeID string) (Recipe, error) {
				return Recipe{ID: recipeID, LocationID: locationID, Name: "recipe", YieldCount: 1, Lines: []RecipeLine{{TargetType: LineTargetIngredient, TargetID: "salsa", Quantity: 1, Unit: ingredient.UnitEach}}}, nil
			},
		},
		fakeIngredientLookup{
			getFn: func(_ context.Context, locationID string, ingredientID string) (ingredient.Ingredient, error) {
				return ingredient.Ingredient{
					ID:             ingredientID,
					LocationID:     locationID,
					BaseUnit:       ingredient.UnitGram,
					AlternateUnits: map[ingredient.Unit]float64{ingredient.UnitEach: 100},
				}, nil
			},
		},
		nil,
	)

	_, err := service.CreateMenuItem(context.Background(), MenuItem{
		ID:         "mi-1",
		LocationID: "loc-1",
		RecipeID:   "rec-1",
		Name:       "Bowl",
		PriceMinor: 845,
		Currency:   "usd",
		ModifierGroups: []ModifierGroup{
			{
				ID:           "group-1",
				Name:         "Protein",
				SelectionMin: 1,
				SelectionMax: 1,
				Required:     true,
				Exclusive:    true,
				Modifiers: []Modifier{
					{ID: "mod-1", Name: "Chicken", Currency: "USD"},
				},
			},
		},
	})
	if !errors.Is(err, ErrInvalidMenuItem) {
		t.Fatalf("err = %v, want ErrInvalidMenuItem", err)
	}
}

func TestCreateMenuItemRejectsBadSelectionRules(t *testing.T) {
	service := NewCatalogService(
		fakeCatalogRepository{
			createMenuItemFn: func(_ context.Context, item MenuItem) (MenuItem, error) {
				return item, nil
			},
		},
		fakeRecipeRepository{
			getFn: func(_ context.Context, locationID string, recipeID string) (Recipe, error) {
				return Recipe{ID: recipeID, LocationID: locationID, Name: "recipe", YieldCount: 1, Lines: []RecipeLine{{TargetType: LineTargetIngredient, TargetID: "salsa", Quantity: 1, Unit: ingredient.UnitEach}}}, nil
			},
		},
		fakeIngredientLookup{
			getFn: func(_ context.Context, locationID string, ingredientID string) (ingredient.Ingredient, error) {
				return ingredient.Ingredient{
					ID:             ingredientID,
					LocationID:     locationID,
					BaseUnit:       ingredient.UnitGram,
					AlternateUnits: map[ingredient.Unit]float64{ingredient.UnitEach: 100},
				}, nil
			},
		},
		nil,
	)

	_, err := service.CreateMenuItem(context.Background(), MenuItem{
		ID:         "mi-1",
		LocationID: "loc-1",
		RecipeID:   "rec-1",
		Name:       "Bowl",
		PriceMinor: 845,
		Currency:   "USD",
		ModifierGroups: []ModifierGroup{
			{
				ID:           "group-1",
				Name:         "Protein",
				SelectionMin: 1,
				SelectionMax: 2,
				Required:     true,
				Exclusive:    true,
				Modifiers: []Modifier{
					{
						ID:       "mod-1",
						Name:     "Chicken",
						Currency: "USD",
						IngredientDeltas: []IngredientDelta{
							{IngredientID: "chicken", Quantity: 1, Unit: ingredient.UnitEach},
						},
					},
				},
			},
		},
	})
	if !errors.Is(err, ErrInvalidMenuItem) {
		t.Fatalf("err = %v, want ErrInvalidMenuItem", err)
	}
}

func TestGenerateSnapshotBuildsDerivedModifierDataAndVersions(t *testing.T) {
	repo := &snapshotCountingRepo{}
	service := NewCatalogService(
		repo,
		nil,
		nil,
		fakeSnapshotResolver{
			resolveRecipeFn: func(_ context.Context, locationID string, recipeID string) (ResolvedRecipeData, error) {
				if locationID != "loc-1" {
					t.Fatalf("locationID = %q, want loc-1", locationID)
				}
				if recipeID != "rec-1" {
					t.Fatalf("recipeID = %q, want rec-1", recipeID)
				}
				return ResolvedRecipeData{
					Macros:          ingredient.MacroValues{Calories: 25},
					CostMinor:       321,
					LowConfidence:   true,
					IngredientUsage: map[string]float64{"salsa": 100},
					IngredientUnits: map[string]ingredient.Unit{"salsa": ingredient.UnitGram},
				}, nil
			},
			resolveModifierFn: func(_ context.Context, locationID string, modifier Modifier) (ResolvedModifierData, error) {
				if locationID != "loc-1" {
					t.Fatalf("locationID = %q, want loc-1", locationID)
				}
				switch modifier.ID {
				case "mod-chicken":
					return ResolvedModifierData{
						MacroDelta:      ingredient.MacroValues{Calories: 180, ProteinGrams: 32},
						CostMinor:       210,
						LowConfidence:   false,
						IngredientUsage: map[string]float64{"chicken": 1},
						IngredientUnits: map[string]ingredient.Unit{"chicken": ingredient.UnitGram},
					}, nil
				case "mod-guac":
					return ResolvedModifierData{
						MacroDelta:      ingredient.MacroValues{Calories: 230, FatGrams: 22},
						CostMinor:       125,
						LowConfidence:   true,
						IngredientUsage: map[string]float64{"guac": 1},
						IngredientUnits: map[string]ingredient.Unit{"guac": ingredient.UnitGram},
					}, nil
				default:
					return ResolvedModifierData{}, errors.New("unexpected modifier")
				}
			},
		},
	)

	snapshot, err := service.GenerateSnapshot(context.Background(), GenerateSnapshotInput{
		ID:         "snap-1",
		LocationID: "loc-1",
	})
	if err != nil {
		t.Fatalf("GenerateSnapshot returned error: %v", err)
	}
	if snapshot.Version != 1 {
		t.Fatalf("snapshot version = %d, want 1", snapshot.Version)
	}
	if len(snapshot.Items) != 1 {
		t.Fatalf("snapshot items = %d, want 1", len(snapshot.Items))
	}
	if got := snapshot.Items[0].ModifierGroups[0].Modifiers[0].IngredientUsage["chicken"]; got != 1 {
		t.Fatalf("modifier chicken usage = %v, want 1", got)
	}
	if got := snapshot.Items[0].ModifierGroups[0].Modifiers[0].IngredientUnits["chicken"]; got != ingredient.UnitGram {
		t.Fatalf("modifier chicken unit = %s, want g", got)
	}
	if got := snapshot.Items[0].IngredientUnits["salsa"]; got != ingredient.UnitGram {
		t.Fatalf("snapshot salsa unit = %s, want g", got)
	}
	if got := snapshot.Items[0].CostMinor; got != 321 {
		t.Fatalf("snapshot cost = %d, want 321", got)
	}
	if !snapshot.Items[0].LowConfidence {
		t.Fatal("snapshot low confidence = false, want true")
	}
	if got := snapshot.Items[0].ModifierGroups[0].Modifiers[0].CostMinor; got != 210 {
		t.Fatalf("modifier cost = %d, want 210", got)
	}

	snapshot, err = service.GenerateSnapshot(context.Background(), GenerateSnapshotInput{
		ID:         "snap-2",
		LocationID: "loc-1",
	})
	if err != nil {
		t.Fatalf("second GenerateSnapshot returned error: %v", err)
	}
	if snapshot.Version != 2 {
		t.Fatalf("snapshot version = %d, want 2", snapshot.Version)
	}
}

func TestCreateMenuItemPropagatesIngredientLookupInfrastructureError(t *testing.T) {
	boom := errors.New("db unavailable")
	service := NewCatalogService(
		fakeCatalogRepository{
			createMenuItemFn: func(_ context.Context, item MenuItem) (MenuItem, error) {
				return item, nil
			},
		},
		fakeRecipeRepository{
			getFn: func(_ context.Context, locationID string, recipeID string) (Recipe, error) {
				return Recipe{ID: recipeID, LocationID: locationID, Name: "recipe", YieldCount: 1, Lines: []RecipeLine{{TargetType: LineTargetIngredient, TargetID: "salsa", Quantity: 1, Unit: ingredient.UnitEach}}}, nil
			},
		},
		fakeIngredientLookup{
			getFn: func(_ context.Context, locationID string, ingredientID string) (ingredient.Ingredient, error) {
				return ingredient.Ingredient{}, boom
			},
		},
		nil,
	)

	_, err := service.CreateMenuItem(context.Background(), MenuItem{
		ID:         "mi-1",
		LocationID: "loc-1",
		RecipeID:   "rec-1",
		Name:       "Bowl",
		PriceMinor: 845,
		Currency:   "USD",
		ModifierGroups: []ModifierGroup{{
			ID:           "group-1",
			Name:         "Protein",
			SelectionMin: 1,
			SelectionMax: 1,
			Required:     true,
			Exclusive:    true,
			Modifiers: []Modifier{{
				ID:       "mod-1",
				Name:     "Chicken",
				Currency: "USD",
				IngredientDeltas: []IngredientDelta{{
					IngredientID: "chicken",
					Quantity:     1,
					Unit:         ingredient.UnitEach,
				}},
			}},
		}},
	})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want db unavailable", err)
	}
	if errors.Is(err, ErrInvalidMenuItem) {
		t.Fatalf("err = %v, did not want ErrInvalidMenuItem", err)
	}
}

type snapshotCountingRepo struct {
	version int
}

func (r *snapshotCountingRepo) GetMenuItem(_ context.Context, locationID string, menuItemID string) (MenuItem, error) {
	return MenuItem{}, ErrMenuItemNotFound
}

func (r *snapshotCountingRepo) ListMenuItems(_ context.Context, locationID string) ([]MenuItem, error) {
	return []MenuItem{
		{
			ID:          "mi-1",
			LocationID:  locationID,
			RecipeID:    "rec-1",
			Name:        "Burrito Bowl",
			Description: "desc",
			PriceMinor:  845,
			Currency:    "USD",
			ModifierGroups: []ModifierGroup{
				{
					ID:                 "group-1",
					Name:               "Protein",
					SelectionMin:       1,
					SelectionMax:       1,
					Required:           true,
					Exclusive:          true,
					DefaultModifierIDs: []string{"mod-chicken"},
					Modifiers: []Modifier{
						{ID: "mod-chicken", Name: "Chicken", PriceDeltaMinor: 0, Currency: "USD", IngredientDeltas: []IngredientDelta{{IngredientID: "chicken", Quantity: 1, Unit: ingredient.UnitEach}}},
						{ID: "mod-guac", Name: "Guac", PriceDeltaMinor: 285, Currency: "USD", IngredientDeltas: []IngredientDelta{{IngredientID: "guac", Quantity: 1, Unit: ingredient.UnitEach}}},
					},
				},
			},
		},
	}, nil
}

func (r *snapshotCountingRepo) CreateMenuItem(_ context.Context, item MenuItem) (MenuItem, error) {
	return item, nil
}

func (r *snapshotCountingRepo) UpdateMenuItem(_ context.Context, item MenuItem) (MenuItem, error) {
	return item, nil
}

func (r *snapshotCountingRepo) CreateSnapshot(_ context.Context, snapshot MenuSnapshot) (MenuSnapshot, error) {
	r.version++
	snapshot.Version = r.version
	return snapshot, nil
}

func (r *snapshotCountingRepo) GetSnapshot(_ context.Context, locationID string, snapshotID string) (MenuSnapshot, error) {
	return MenuSnapshot{}, ErrSnapshotNotFound
}

func (r *snapshotCountingRepo) ListSnapshots(_ context.Context, locationID string) ([]MenuSnapshot, error) {
	return nil, nil
}
