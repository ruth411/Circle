package resolve

import (
	"context"
	"testing"

	"github.com/ruth411/circle/internal/core/ingredient"
	"github.com/ruth411/circle/internal/core/recipe"
	"github.com/ruth411/circle/internal/ordering"
	"github.com/ruth411/circle/internal/platform/biztime"
)

type fakeRecipeRepository struct {
	listFn func(context.Context, string) ([]recipe.Recipe, error)
	getFn  func(context.Context, string, string) (recipe.Recipe, error)
}

func (f fakeRecipeRepository) List(ctx context.Context, locationID string) ([]recipe.Recipe, error) {
	return f.listFn(ctx, locationID)
}

func (f fakeRecipeRepository) Get(ctx context.Context, locationID string, recipeID string) (recipe.Recipe, error) {
	return f.getFn(ctx, locationID, recipeID)
}

func (f fakeRecipeRepository) Create(context.Context, recipe.Recipe) (recipe.Recipe, error) {
	panic("unexpected Create call")
}

func (f fakeRecipeRepository) Update(context.Context, recipe.Recipe) (recipe.Recipe, error) {
	panic("unexpected Update call")
}

type fakeIngredientRepository struct {
	listFn func(context.Context, string, string) ([]ingredient.Ingredient, error)
}

func (f fakeIngredientRepository) List(ctx context.Context, locationID string, search string) ([]ingredient.Ingredient, error) {
	return f.listFn(ctx, locationID, search)
}

func (f fakeIngredientRepository) Create(context.Context, ingredient.Ingredient) (ingredient.Ingredient, error) {
	panic("unexpected Create call")
}

func (f fakeIngredientRepository) Update(context.Context, ingredient.Ingredient) (ingredient.Ingredient, error) {
	panic("unexpected Update call")
}

type fakeCatalogRepository struct {
	listMenuItemsFn  func(context.Context, string) ([]recipe.MenuItem, error)
	createSnapshotFn func(context.Context, recipe.MenuSnapshot) (recipe.MenuSnapshot, error)
}

func (f fakeCatalogRepository) GetMenuItem(context.Context, string, string) (recipe.MenuItem, error) {
	panic("unexpected GetMenuItem call")
}

func (f fakeCatalogRepository) ListMenuItems(ctx context.Context, locationID string) ([]recipe.MenuItem, error) {
	return f.listMenuItemsFn(ctx, locationID)
}

func (f fakeCatalogRepository) CreateMenuItem(context.Context, recipe.MenuItem) (recipe.MenuItem, error) {
	panic("unexpected CreateMenuItem call")
}

func (f fakeCatalogRepository) UpdateMenuItem(context.Context, recipe.MenuItem) (recipe.MenuItem, error) {
	panic("unexpected UpdateMenuItem call")
}

func (f fakeCatalogRepository) CreateSnapshot(ctx context.Context, snapshot recipe.MenuSnapshot) (recipe.MenuSnapshot, error) {
	return f.createSnapshotFn(ctx, snapshot)
}

func (f fakeCatalogRepository) GetSnapshot(context.Context, string, string) (recipe.MenuSnapshot, error) {
	panic("unexpected GetSnapshot call")
}

func (f fakeCatalogRepository) ListSnapshots(context.Context, string) ([]recipe.MenuSnapshot, error) {
	panic("unexpected ListSnapshots call")
}

func TestSnapshotResolverResolveRecipeUsesPerServingMacros(t *testing.T) {
	resolver := SnapshotResolver{
		Recipes: fakeRecipeRepository{
			listFn: func(_ context.Context, locationID string) ([]recipe.Recipe, error) {
				return []recipe.Recipe{{
					ID:         "rec-1",
					LocationID: locationID,
					Name:       "Chicken Bowl Base",
					YieldCount: 2,
					Lines: []recipe.RecipeLine{{
						LineNumber: 1,
						TargetType: recipe.LineTargetIngredient,
						TargetID:   "chicken",
						Quantity:   100,
						Unit:       ingredient.UnitGram,
					}},
				}}, nil
			},
			getFn: func(_ context.Context, locationID string, recipeID string) (recipe.Recipe, error) {
				return recipe.Recipe{
					ID:         recipeID,
					LocationID: locationID,
					Name:       "Chicken Bowl Base",
					YieldCount: 2,
					Lines: []recipe.RecipeLine{{
						LineNumber: 1,
						TargetType: recipe.LineTargetIngredient,
						TargetID:   "chicken",
						Quantity:   100,
						Unit:       ingredient.UnitGram,
					}},
				}, nil
			},
		},
		Ingredients: fakeIngredientRepository{
			listFn: func(_ context.Context, locationID string, search string) ([]ingredient.Ingredient, error) {
				return []ingredient.Ingredient{{
					ID:                "chicken",
					LocationID:        locationID,
					Name:              "Chicken",
					Category:          "protein",
					BaseUnit:          ingredient.UnitGram,
					MacrosPerBaseUnit: ingredient.MacroValues{Calories: 2, ProteinGrams: 0.3},
					Currency:          "USD",
				}}, nil
			},
		},
	}

	resolved, err := resolver.ResolveRecipe(context.Background(), "loc-1", "rec-1")
	if err != nil {
		t.Fatalf("ResolveRecipe returned error: %v", err)
	}
	if resolved.Macros.Calories != 100 {
		t.Fatalf("calories = %v, want 100", resolved.Macros.Calories)
	}
	if resolved.IngredientUsage["chicken"] != 100 {
		t.Fatalf("ingredient usage = %v, want 100", resolved.IngredientUsage["chicken"])
	}
}

func TestSnapshotResolverResolveModifierPreservesSignedDeltas(t *testing.T) {
	resolver := SnapshotResolver{
		Recipes: fakeRecipeRepository{
			listFn: func(context.Context, string) ([]recipe.Recipe, error) { return nil, nil },
			getFn:  func(context.Context, string, string) (recipe.Recipe, error) { return recipe.Recipe{}, nil },
		},
		Ingredients: fakeIngredientRepository{
			listFn: func(_ context.Context, locationID string, search string) ([]ingredient.Ingredient, error) {
				return []ingredient.Ingredient{{
					ID:                "rice",
					LocationID:        locationID,
					Name:              "Rice",
					Category:          "base",
					BaseUnit:          ingredient.UnitGram,
					MacrosPerBaseUnit: ingredient.MacroValues{Calories: 1.3, CarbsGrams: 0.28},
					Currency:          "USD",
				}}, nil
			},
		},
	}

	resolved, err := resolver.ResolveModifier(context.Background(), "loc-1", recipe.Modifier{
		ID:       "no-rice",
		Name:     "No Rice",
		Currency: "USD",
		IngredientDeltas: []recipe.IngredientDelta{{
			IngredientID: "rice",
			Quantity:     -50,
			Unit:         ingredient.UnitGram,
		}},
	})
	if err != nil {
		t.Fatalf("ResolveModifier returned error: %v", err)
	}
	if resolved.MacroDelta.Calories != -65 {
		t.Fatalf("calories = %v, want -65", resolved.MacroDelta.Calories)
	}
	if resolved.IngredientUsage["rice"] != -50 {
		t.Fatalf("ingredient usage = %v, want -50", resolved.IngredientUsage["rice"])
	}
}

func TestGeneratedSnapshotCanCreateOrder(t *testing.T) {
	resolver := SnapshotResolver{
		Recipes: fakeRecipeRepository{
			listFn: func(_ context.Context, locationID string) ([]recipe.Recipe, error) {
				return []recipe.Recipe{{
					ID:         "rec-1",
					LocationID: locationID,
					Name:       "Chicken Bowl Base",
					YieldCount: 1,
					Lines: []recipe.RecipeLine{{
						LineNumber: 1,
						TargetType: recipe.LineTargetIngredient,
						TargetID:   "chicken",
						Quantity:   150,
						Unit:       ingredient.UnitGram,
					}},
				}}, nil
			},
			getFn: func(_ context.Context, locationID string, recipeID string) (recipe.Recipe, error) {
				return recipe.Recipe{
					ID:         recipeID,
					LocationID: locationID,
					Name:       "Chicken Bowl Base",
					YieldCount: 1,
					Lines: []recipe.RecipeLine{{
						LineNumber: 1,
						TargetType: recipe.LineTargetIngredient,
						TargetID:   "chicken",
						Quantity:   150,
						Unit:       ingredient.UnitGram,
					}},
				}, nil
			},
		},
		Ingredients: fakeIngredientRepository{
			listFn: func(_ context.Context, locationID string, search string) ([]ingredient.Ingredient, error) {
				return []ingredient.Ingredient{{
					ID:                "chicken",
					LocationID:        locationID,
					Name:              "Chicken",
					Category:          "protein",
					BaseUnit:          ingredient.UnitGram,
					MacrosPerBaseUnit: ingredient.MacroValues{Calories: 2, ProteinGrams: 0.3},
					Currency:          "USD",
				}}, nil
			},
		},
	}

	catalogService := recipe.NewCatalogService(fakeCatalogRepository{
		listMenuItemsFn: func(_ context.Context, locationID string) ([]recipe.MenuItem, error) {
			return []recipe.MenuItem{{
				ID:         "bowl",
				LocationID: locationID,
				RecipeID:   "rec-1",
				Name:       "Chicken Bowl",
				PriceMinor: 1095,
				Currency:   "USD",
			}}, nil
		},
		createSnapshotFn: func(_ context.Context, snapshot recipe.MenuSnapshot) (recipe.MenuSnapshot, error) {
			snapshot.Version = 1
			return snapshot, nil
		},
	}, fakeRecipeRepository{}, nil, resolver)

	snapshot, err := catalogService.GenerateSnapshot(context.Background(), recipe.GenerateSnapshotInput{
		ID:         "snap-1",
		LocationID: "loc-1",
	})
	if err != nil {
		t.Fatalf("GenerateSnapshot returned error: %v", err)
	}

	orderingService := ordering.NewService(ordering.MockProvider{})
	if err := orderingService.RegisterSnapshot(snapshot); err != nil {
		t.Fatalf("RegisterSnapshot returned error: %v", err)
	}

	order, err := orderingService.CreateOrder(context.Background(), ordering.CreateOrderInput{
		OrderID:      "order-1",
		LocationID:   "loc-1",
		SnapshotID:   snapshot.ID,
		BusinessDate: biztime.BusinessDate("2026-08-01"),
	})
	if err != nil {
		t.Fatalf("CreateOrder returned error: %v", err)
	}
	if order.SnapshotVersion != 1 {
		t.Fatalf("snapshot version = %d, want 1", order.SnapshotVersion)
	}
}

func TestGenerateSnapshotLoadsCatalogOncePerCall(t *testing.T) {
	recipeListCalls := 0
	ingredientListCalls := 0

	resolver := SnapshotResolver{
		Recipes: fakeRecipeRepository{
			listFn: func(_ context.Context, locationID string) ([]recipe.Recipe, error) {
				recipeListCalls++
				return []recipe.Recipe{{
					ID:         "rec-1",
					LocationID: locationID,
					Name:       "Chicken Bowl Base",
					YieldCount: 1,
					Lines: []recipe.RecipeLine{{
						LineNumber: 1,
						TargetType: recipe.LineTargetIngredient,
						TargetID:   "chicken",
						Quantity:   150,
						Unit:       ingredient.UnitGram,
					}},
				}}, nil
			},
			getFn: func(_ context.Context, locationID string, recipeID string) (recipe.Recipe, error) {
				return recipe.Recipe{}, nil
			},
		},
		Ingredients: fakeIngredientRepository{
			listFn: func(_ context.Context, locationID string, search string) ([]ingredient.Ingredient, error) {
				ingredientListCalls++
				return []ingredient.Ingredient{{
					ID:                "chicken",
					LocationID:        locationID,
					Name:              "Chicken",
					Category:          "protein",
					BaseUnit:          ingredient.UnitGram,
					MacrosPerBaseUnit: ingredient.MacroValues{Calories: 2, ProteinGrams: 0.3},
					Currency:          "USD",
				}}, nil
			},
		},
	}

	catalogService := recipe.NewCatalogService(fakeCatalogRepository{
		listMenuItemsFn: func(_ context.Context, locationID string) ([]recipe.MenuItem, error) {
			return []recipe.MenuItem{{
				ID:         "bowl",
				LocationID: locationID,
				RecipeID:   "rec-1",
				Name:       "Chicken Bowl",
				PriceMinor: 1095,
				Currency:   "USD",
				ModifierGroups: []recipe.ModifierGroup{{
					ID:                 "protein",
					Name:               "Protein",
					SelectionMin:       0,
					SelectionMax:       2,
					DefaultModifierIDs: []string{"extra-1"},
					Modifiers: []recipe.Modifier{
						{ID: "extra-1", Name: "Extra Chicken", Currency: "USD", IngredientDeltas: []recipe.IngredientDelta{{IngredientID: "chicken", Quantity: 50, Unit: ingredient.UnitGram}}},
						{ID: "extra-2", Name: "Double Chicken", Currency: "USD", IngredientDeltas: []recipe.IngredientDelta{{IngredientID: "chicken", Quantity: 100, Unit: ingredient.UnitGram}}},
					},
				}},
			}}, nil
		},
		createSnapshotFn: func(_ context.Context, snapshot recipe.MenuSnapshot) (recipe.MenuSnapshot, error) {
			snapshot.Version = 1
			return snapshot, nil
		},
	}, fakeRecipeRepository{}, nil, resolver)

	if _, err := catalogService.GenerateSnapshot(context.Background(), recipe.GenerateSnapshotInput{
		ID:         "snap-1",
		LocationID: "loc-1",
	}); err != nil {
		t.Fatalf("GenerateSnapshot returned error: %v", err)
	}

	if recipeListCalls != 1 {
		t.Fatalf("recipe list calls = %d, want 1", recipeListCalls)
	}
	if ingredientListCalls != 1 {
		t.Fatalf("ingredient list calls = %d, want 1", ingredientListCalls)
	}
}
