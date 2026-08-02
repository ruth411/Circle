package recipe

import (
	"context"
	"errors"
	"testing"

	"github.com/ruth411/circle/internal/core/ingredient"
)

type fakeRecipeRepository struct {
	getFn    func(context.Context, string, string) (Recipe, error)
	listFn   func(context.Context, string) ([]Recipe, error)
	createFn func(context.Context, Recipe) (Recipe, error)
	updateFn func(context.Context, Recipe) (Recipe, error)
}

func (f fakeRecipeRepository) Get(ctx context.Context, locationID string, recipeID string) (Recipe, error) {
	return f.getFn(ctx, locationID, recipeID)
}

func (f fakeRecipeRepository) List(ctx context.Context, locationID string) ([]Recipe, error) {
	return f.listFn(ctx, locationID)
}

func (f fakeRecipeRepository) Create(ctx context.Context, recipe Recipe) (Recipe, error) {
	return f.createFn(ctx, recipe)
}

func (f fakeRecipeRepository) Update(ctx context.Context, recipe Recipe) (Recipe, error) {
	return f.updateFn(ctx, recipe)
}

type fakeIngredientLookup struct {
	getFn func(context.Context, string, string) (ingredient.Ingredient, error)
}

func (f fakeIngredientLookup) Get(ctx context.Context, locationID string, ingredientID string) (ingredient.Ingredient, error) {
	return f.getFn(ctx, locationID, ingredientID)
}

func TestCreateRecipeValidatesIngredientAndNumbersLines(t *testing.T) {
	service := NewService(fakeRecipeRepository{
		createFn: func(_ context.Context, recipe Recipe) (Recipe, error) {
			return recipe, nil
		},
	}, fakeIngredientLookup{
		getFn: func(_ context.Context, locationID string, ingredientID string) (ingredient.Ingredient, error) {
			return ingredient.Ingredient{
				ID:             ingredientID,
				LocationID:     locationID,
				BaseUnit:       ingredient.UnitGram,
				AlternateUnits: map[ingredient.Unit]float64{ingredient.UnitEach: 100},
			}, nil
		},
	})

	created, err := service.Create(context.Background(), UpsertInput{
		ID:         " rec-1 ",
		LocationID: " loc-1 ",
		Name:       " Chicken Prep ",
		YieldCount: 2,
		Lines: []RecipeLine{
			{TargetType: LineTargetIngredient, TargetID: "chicken", Quantity: 2, Unit: ingredient.UnitEach},
		},
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if created.ID != "rec-1" {
		t.Fatalf("ID = %q, want rec-1", created.ID)
	}
	if created.Lines[0].LineNumber != 1 {
		t.Fatalf("LineNumber = %d, want 1", created.Lines[0].LineNumber)
	}
}

func TestCreateRecipeRejectsNestedCycle(t *testing.T) {
	service := NewService(fakeRecipeRepository{
		getFn: func(_ context.Context, locationID string, recipeID string) (Recipe, error) {
			return Recipe{
				ID:         recipeID,
				LocationID: locationID,
				Name:       "child",
				YieldCount: 1,
				Lines: []RecipeLine{
					{TargetType: LineTargetRecipe, TargetID: "rec-root", Quantity: 1, Unit: ingredient.UnitEach},
				},
			}, nil
		},
		createFn: func(_ context.Context, recipe Recipe) (Recipe, error) {
			return recipe, nil
		},
	}, fakeIngredientLookup{
		getFn: func(_ context.Context, locationID string, ingredientID string) (ingredient.Ingredient, error) {
			return ingredient.Ingredient{}, ingredient.ErrNotFound
		},
	})

	_, err := service.Create(context.Background(), UpsertInput{
		ID:         "rec-root",
		LocationID: "loc-1",
		Name:       "Root",
		YieldCount: 1,
		Lines: []RecipeLine{
			{TargetType: LineTargetRecipe, TargetID: "rec-child", Quantity: 1, Unit: ingredient.UnitEach},
		},
	})
	if !errors.Is(err, ErrInvalidRecipe) {
		t.Fatalf("err = %v, want ErrInvalidRecipe", err)
	}
}

func TestCreateRecipeRejectsTooDeepGraph(t *testing.T) {
	graph := map[string]Recipe{
		"rec-a": {ID: "rec-a", LocationID: "loc-1", Name: "a", YieldCount: 1, Lines: []RecipeLine{{TargetType: LineTargetRecipe, TargetID: "rec-b", Quantity: 1, Unit: ingredient.UnitEach}}},
		"rec-b": {ID: "rec-b", LocationID: "loc-1", Name: "b", YieldCount: 1, Lines: []RecipeLine{{TargetType: LineTargetRecipe, TargetID: "rec-c", Quantity: 1, Unit: ingredient.UnitEach}}},
		"rec-c": {ID: "rec-c", LocationID: "loc-1", Name: "c", YieldCount: 1, Lines: []RecipeLine{{TargetType: LineTargetIngredient, TargetID: "rice", Quantity: 1, Unit: ingredient.UnitGram}}},
	}

	service := NewService(fakeRecipeRepository{
		getFn: func(_ context.Context, locationID string, recipeID string) (Recipe, error) {
			recipe, ok := graph[recipeID]
			if !ok {
				return Recipe{}, ErrRecipeNotFound
			}
			return recipe, nil
		},
		createFn: func(_ context.Context, recipe Recipe) (Recipe, error) {
			return recipe, nil
		},
	}, fakeIngredientLookup{
		getFn: func(_ context.Context, locationID string, ingredientID string) (ingredient.Ingredient, error) {
			return ingredient.Ingredient{
				ID:         ingredientID,
				LocationID: locationID,
				BaseUnit:   ingredient.UnitGram,
			}, nil
		},
	})
	service.maxDepth = 2

	_, err := service.Create(context.Background(), UpsertInput{
		ID:         "rec-root",
		LocationID: "loc-1",
		Name:       "root",
		YieldCount: 1,
		Lines: []RecipeLine{
			{TargetType: LineTargetRecipe, TargetID: "rec-a", Quantity: 1, Unit: ingredient.UnitEach},
		},
	})
	if !errors.Is(err, ErrInvalidRecipe) {
		t.Fatalf("err = %v, want ErrInvalidRecipe", err)
	}
}

func TestUpdateRecipeReturnsRepositoryError(t *testing.T) {
	service := NewService(fakeRecipeRepository{
		updateFn: func(_ context.Context, recipe Recipe) (Recipe, error) {
			return Recipe{}, ErrRecipeNotFound
		},
	}, fakeIngredientLookup{
		getFn: func(_ context.Context, locationID string, ingredientID string) (ingredient.Ingredient, error) {
			return ingredient.Ingredient{
				ID:         ingredientID,
				LocationID: locationID,
				BaseUnit:   ingredient.UnitEach,
			}, nil
		},
	})

	_, err := service.Update(context.Background(), UpsertInput{
		ID:         "rec-1",
		LocationID: "loc-1",
		Name:       "Recipe",
		YieldCount: 1,
		Lines: []RecipeLine{
			{TargetType: LineTargetIngredient, TargetID: "chicken", Quantity: 1, Unit: ingredient.UnitEach},
		},
	})
	if !errors.Is(err, ErrRecipeNotFound) {
		t.Fatalf("err = %v, want ErrRecipeNotFound", err)
	}
}

func TestCreateRecipePropagatesIngredientLookupInfrastructureError(t *testing.T) {
	boom := errors.New("db unavailable")
	service := NewService(fakeRecipeRepository{
		createFn: func(_ context.Context, recipe Recipe) (Recipe, error) {
			return recipe, nil
		},
	}, fakeIngredientLookup{
		getFn: func(_ context.Context, locationID string, ingredientID string) (ingredient.Ingredient, error) {
			return ingredient.Ingredient{}, boom
		},
	})

	_, err := service.Create(context.Background(), UpsertInput{
		ID:         "rec-1",
		LocationID: "loc-1",
		Name:       "Recipe",
		YieldCount: 1,
		Lines: []RecipeLine{
			{TargetType: LineTargetIngredient, TargetID: "chicken", Quantity: 1, Unit: ingredient.UnitEach},
		},
	})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want db unavailable", err)
	}
	if errors.Is(err, ErrInvalidRecipe) {
		t.Fatalf("err = %v, did not want ErrInvalidRecipe", err)
	}
}
