package resolve

import (
	"context"

	"github.com/ruth411/circle/internal/core/ingredient"
	"github.com/ruth411/circle/internal/core/nutrition"
	"github.com/ruth411/circle/internal/core/recipe"
)

type recipeSource interface {
	List(context.Context, string) ([]recipe.Recipe, error)
}

type ingredientSource interface {
	List(context.Context, string, string) ([]ingredient.Ingredient, error)
}

type SnapshotResolver struct {
	Recipes     recipeSource
	Ingredients ingredientSource
	MaxDepth    int
}

type loadedSnapshotResolver struct {
	calc nutrition.Calculator
}

func (r SnapshotResolver) Prepare(ctx context.Context, locationID string) (recipe.SnapshotResolver, error) {
	calc, err := r.loadCalculator(ctx, locationID)
	if err != nil {
		return nil, err
	}
	return loadedSnapshotResolver{calc: calc}, nil
}

func (r SnapshotResolver) ResolveRecipe(ctx context.Context, locationID string, recipeID string) (recipe.ResolvedRecipeData, error) {
	prepared, err := r.Prepare(ctx, locationID)
	if err != nil {
		return recipe.ResolvedRecipeData{}, err
	}
	return prepared.ResolveRecipe(ctx, locationID, recipeID)
}

func (r loadedSnapshotResolver) ResolveRecipe(_ context.Context, _ string, recipeID string) (recipe.ResolvedRecipeData, error) {
	resolved, err := r.calc.ResolveRecipe(recipeID)
	if err != nil {
		return recipe.ResolvedRecipeData{}, err
	}

	return recipe.ResolvedRecipeData{
		Macros:          resolved.PerServing,
		CostMinor:       resolved.PerServingCostMinor,
		LowConfidence:   resolved.Confidence.Level != nutrition.ConfidenceHigh,
		IngredientUsage: cloneUsage(resolved.IngredientUsage),
		IngredientUnits: cloneUnits(resolved.IngredientUnits),
	}, nil
}

func (r SnapshotResolver) ResolveModifier(ctx context.Context, locationID string, modifier recipe.Modifier) (recipe.ResolvedModifierData, error) {
	prepared, err := r.Prepare(ctx, locationID)
	if err != nil {
		return recipe.ResolvedModifierData{}, err
	}
	return prepared.ResolveModifier(ctx, locationID, modifier)
}

func (r loadedSnapshotResolver) ResolveModifier(_ context.Context, _ string, modifier recipe.Modifier) (recipe.ResolvedModifierData, error) {
	resolved, err := r.calc.ResolveModifier(modifier)
	if err != nil {
		return recipe.ResolvedModifierData{}, err
	}

	return recipe.ResolvedModifierData{
		MacroDelta:      resolved.MacroDelta,
		CostMinor:       resolved.CostDeltaMinor,
		LowConfidence:   resolved.Confidence.Level != nutrition.ConfidenceHigh,
		IngredientUsage: cloneUsage(resolved.IngredientUsage),
		IngredientUnits: cloneUnits(resolved.IngredientUnits),
	}, nil
}

func (r SnapshotResolver) loadCalculator(ctx context.Context, locationID string) (nutrition.Calculator, error) {
	recipes, err := r.Recipes.List(ctx, locationID)
	if err != nil {
		return nutrition.Calculator{}, err
	}
	ingredients, err := r.Ingredients.List(ctx, locationID, "")
	if err != nil {
		return nutrition.Calculator{}, err
	}

	recipeMap := make(map[string]recipe.Recipe, len(recipes))
	for _, item := range recipes {
		recipeMap[item.ID] = item
	}
	ingredientMap := make(map[string]ingredient.Ingredient, len(ingredients))
	for _, item := range ingredients {
		ingredientMap[item.ID] = item
	}

	return nutrition.Calculator{
		Ingredients: ingredientMap,
		Recipes:     recipeMap,
		MaxDepth:    r.MaxDepth,
	}, nil
}

func cloneUsage(input map[string]float64) map[string]float64 {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]float64, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func cloneUnits(input map[string]ingredient.Unit) map[string]ingredient.Unit {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]ingredient.Unit, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}
