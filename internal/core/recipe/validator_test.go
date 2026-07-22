package recipe

import (
	"strings"
	"testing"

	"github.com/ruth411/circle/internal/core/ingredient"
)

func TestValidateRecipeGraphDetectsCycle(t *testing.T) {
	recipes := map[string]Recipe{
		"a": {
			ID: "a",
			Lines: []RecipeLine{{
				TargetType: LineTargetRecipe,
				TargetID:   "b",
				Quantity:   1,
				Unit:       ingredient.UnitEach,
			}},
		},
		"b": {
			ID: "b",
			Lines: []RecipeLine{{
				TargetType: LineTargetRecipe,
				TargetID:   "a",
				Quantity:   1,
				Unit:       ingredient.UnitEach,
			}},
		},
	}

	err := ValidateRecipeGraph("a", recipes, 4)
	if err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("expected cycle error, got %v", err)
	}
}

func TestValidateRecipeGraphRejectsExcessiveDepth(t *testing.T) {
	recipes := map[string]Recipe{
		"a": {ID: "a", Lines: []RecipeLine{{TargetType: LineTargetRecipe, TargetID: "b", Quantity: 1, Unit: ingredient.UnitEach}}},
		"b": {ID: "b", Lines: []RecipeLine{{TargetType: LineTargetRecipe, TargetID: "c", Quantity: 1, Unit: ingredient.UnitEach}}},
		"c": {ID: "c", Lines: []RecipeLine{{TargetType: LineTargetIngredient, TargetID: "rice", Quantity: 10, Unit: ingredient.UnitGram}}},
	}

	err := ValidateRecipeGraph("a", recipes, 2)
	if err == nil || !strings.Contains(err.Error(), "max depth") {
		t.Fatalf("expected max depth error, got %v", err)
	}
}
