package nutrition

import (
	"math"
	"testing"

	"github.com/ruth411/circle/internal/core/ingredient"
	"github.com/ruth411/circle/internal/core/recipe"
)

func TestResolveRecipeRollsUpNestedRecipesAndConfidence(t *testing.T) {
	calc := Calculator{
		MaxDepth: 8,
		Ingredients: map[string]ingredient.Ingredient{
			"chicken": {
				ID:                     "chicken",
				BaseUnit:               ingredient.UnitGram,
				MacrosPerBaseUnit:      ingredient.MacroValues{Calories: 2, ProteinGrams: 0.3, FatGrams: 0.1},
				CurrentCostPerBaseUnit: ingredient.MustCostPerBaseUnit(2.1234),
				VerificationStatus:     ingredient.VerificationVerified,
				YieldFactors:           map[string]float64{"cooked": 0.8},
			},
			"rice": {
				ID:                     "rice",
				BaseUnit:               ingredient.UnitGram,
				MacrosPerBaseUnit:      ingredient.MacroValues{Calories: 1.2, CarbsGrams: 0.25},
				CurrentCostPerBaseUnit: ingredient.MustCostPerBaseUnit(4.5),
				VerificationStatus:     ingredient.VerificationUnverified,
			},
		},
		Recipes: map[string]recipe.Recipe{
			"base": {
				ID:         "base",
				YieldCount: 2,
				Lines: []recipe.RecipeLine{
					{TargetType: recipe.LineTargetIngredient, TargetID: "chicken", Quantity: 200, Unit: ingredient.UnitGram, PrepMethod: "cooked"},
					{TargetType: recipe.LineTargetIngredient, TargetID: "rice", Quantity: 100, Unit: ingredient.UnitGram},
				},
			},
			"combo": {
				ID:         "combo",
				YieldCount: 1,
				Lines: []recipe.RecipeLine{
					{TargetType: recipe.LineTargetRecipe, TargetID: "base", Quantity: 2, Unit: ingredient.UnitEach},
				},
			},
		},
	}

	resolved, err := calc.ResolveRecipe("combo")
	if err != nil {
		t.Fatalf("ResolveRecipe returned error: %v", err)
	}

	if got, want := resolved.TotalMacros.Calories, 440.0; !closeEnough(got, want) {
		t.Fatalf("calories = %v, want %v", got, want)
	}
	if got, want := resolved.TotalCostMinor, int64(790); got != want {
		t.Fatalf("total cost = %d, want %d", got, want)
	}
	if got, want := resolved.PerServingCostMinor, int64(790); got != want {
		t.Fatalf("per serving cost = %d, want %d", got, want)
	}
	if got, want := resolved.TotalMacros.ProteinGrams, 48.0; !closeEnough(got, want) {
		t.Fatalf("protein = %v, want %v", got, want)
	}
	if got, want := resolved.IngredientUsage["chicken"], 320.0; !closeEnough(got, want) {
		t.Fatalf("chicken usage = %v, want %v", got, want)
	}
	if resolved.Confidence.Level != ConfidenceMedium {
		t.Fatalf("confidence = %s, want %s", resolved.Confidence.Level, ConfidenceMedium)
	}
}

func TestResolveModifierUsesSignedIngredientDeltas(t *testing.T) {
	calc := Calculator{
		Ingredients: map[string]ingredient.Ingredient{
			"oil": {
				ID:                     "oil",
				BaseUnit:               ingredient.UnitMilliliter,
				MacrosPerBaseUnit:      ingredient.MacroValues{Calories: 8.8, FatGrams: 1},
				CurrentCostPerBaseUnit: ingredient.MustCostPerBaseUnit(2.125),
				VerificationStatus:     ingredient.VerificationVerified,
			},
		},
	}

	modifier := recipe.Modifier{
		ID: "light-oil",
		IngredientDeltas: []recipe.IngredientDelta{
			{IngredientID: "oil", Quantity: -5, Unit: ingredient.UnitMilliliter},
		},
	}

	resolved, err := calc.ResolveModifier(modifier)
	if err != nil {
		t.Fatalf("ResolveModifier returned error: %v", err)
	}

	if got, want := resolved.MacroDelta.Calories, -44.0; !closeEnough(got, want) {
		t.Fatalf("modifier calories = %v, want %v", got, want)
	}
	if got, want := resolved.CostDeltaMinor, int64(-11); got != want {
		t.Fatalf("modifier cost = %d, want %d", got, want)
	}
	if got, want := resolved.IngredientUsage["oil"], -5.0; !closeEnough(got, want) {
		t.Fatalf("modifier usage = %v, want %v", got, want)
	}
}

func TestResolveModifierRoundsLowCostAccumulationCorrectly(t *testing.T) {
	calc := Calculator{
		Ingredients: map[string]ingredient.Ingredient{
			"salt": {
				ID:                     "salt",
				BaseUnit:               ingredient.UnitGram,
				MacrosPerBaseUnit:      ingredient.MacroValues{},
				CurrentCostPerBaseUnit: ingredient.MustCostPerBaseUnit(0.0003),
				VerificationStatus:     ingredient.VerificationVerified,
			},
		},
	}

	modifier := recipe.Modifier{
		ID: "extra-salt",
		IngredientDeltas: []recipe.IngredientDelta{
			{IngredientID: "salt", Quantity: 5000, Unit: ingredient.UnitGram},
		},
	}

	resolved, err := calc.ResolveModifier(modifier)
	if err != nil {
		t.Fatalf("ResolveModifier returned error: %v", err)
	}
	if got, want := resolved.CostDeltaMinor, int64(2); got != want {
		t.Fatalf("modifier cost = %d, want %d", got, want)
	}
}

func closeEnough(got float64, want float64) bool {
	return math.Abs(got-want) < 0.0001
}
