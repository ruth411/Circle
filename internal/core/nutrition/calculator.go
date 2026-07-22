package nutrition

import (
	"fmt"

	"github.com/ruth411/circle/internal/core/ingredient"
	"github.com/ruth411/circle/internal/core/recipe"
)

type ConfidenceLevel string

const (
	ConfidenceHigh   ConfidenceLevel = "high"
	ConfidenceMedium ConfidenceLevel = "medium"
	ConfidenceLow    ConfidenceLevel = "low"
)

type Confidence struct {
	Level   ConfidenceLevel
	Reasons []string
}

type ResolvedRecipe struct {
	TotalMacros     ingredient.MacroValues
	PerServing      ingredient.MacroValues
	IngredientUsage map[string]float64
	Confidence      Confidence
}

type ResolvedModifier struct {
	MacroDelta      ingredient.MacroValues
	IngredientUsage map[string]float64
	Confidence      Confidence
}

type Calculator struct {
	Ingredients map[string]ingredient.Ingredient
	Recipes     map[string]recipe.Recipe
	MaxDepth    int
}

func (c Calculator) ResolveRecipe(recipeID string) (ResolvedRecipe, error) {
	if c.MaxDepth == 0 {
		c.MaxDepth = 8
	}

	return c.resolveRecipe(recipeID, 1, map[string]bool{})
}

func (c Calculator) ResolveModifier(modifier recipe.Modifier) (ResolvedModifier, error) {
	confidence := Confidence{Level: ConfidenceHigh}
	total := ingredient.MacroValues{}
	usage := map[string]float64{}

	for _, delta := range modifier.IngredientDeltas {
		ing, ok := c.Ingredients[delta.IngredientID]
		if !ok {
			return ResolvedModifier{}, fmt.Errorf("ingredient %s not found", delta.IngredientID)
		}

		baseQty, err := ing.ToBaseUnit(abs(delta.Quantity), delta.Unit)
		if err != nil {
			return ResolvedModifier{}, err
		}

		signedQty := baseQty
		if delta.Quantity < 0 {
			signedQty = -signedQty
		}

		if delta.PrepMethod != "" {
			factor, ok := ing.YieldFactor(delta.PrepMethod)
			if !ok {
				confidence.downgrade(ConfidenceMedium, fmt.Sprintf("missing yield factor for %s (%s)", ing.ID, delta.PrepMethod))
			} else {
				signedQty *= factor
			}
		}

		total = total.Add(ing.MacrosPerBaseUnit.Scale(signedQty))
		usage[ing.ID] += signedQty

		if ing.VerificationStatus != ingredient.VerificationVerified {
			confidence.downgrade(ConfidenceMedium, fmt.Sprintf("ingredient %s is unverified", ing.ID))
		}
	}

	return ResolvedModifier{
		MacroDelta:      total,
		IngredientUsage: usage,
		Confidence:      confidence,
	}, nil
}

func (c Calculator) resolveRecipe(recipeID string, depth int, stack map[string]bool) (ResolvedRecipe, error) {
	if depth > c.MaxDepth {
		return ResolvedRecipe{}, fmt.Errorf("recipe graph exceeds max depth %d", c.MaxDepth)
	}

	if stack[recipeID] {
		return ResolvedRecipe{}, fmt.Errorf("recipe cycle detected at %s", recipeID)
	}

	current, ok := c.Recipes[recipeID]
	if !ok {
		return ResolvedRecipe{}, fmt.Errorf("recipe %s not found", recipeID)
	}

	stack[recipeID] = true
	defer delete(stack, recipeID)

	confidence := Confidence{Level: ConfidenceHigh}
	total := ingredient.MacroValues{}
	usage := map[string]float64{}

	for _, line := range current.Lines {
		switch line.TargetType {
		case recipe.LineTargetIngredient:
			ing, ok := c.Ingredients[line.TargetID]
			if !ok {
				return ResolvedRecipe{}, fmt.Errorf("ingredient %s not found", line.TargetID)
			}

			baseQty, err := ing.ToBaseUnit(line.Quantity, line.Unit)
			if err != nil {
				return ResolvedRecipe{}, err
			}

			if line.PrepMethod != "" {
				factor, ok := ing.YieldFactor(line.PrepMethod)
				if !ok {
					confidence.downgrade(ConfidenceMedium, fmt.Sprintf("missing yield factor for %s (%s)", ing.ID, line.PrepMethod))
				} else {
					baseQty *= factor
				}
			}

			total = total.Add(ing.MacrosPerBaseUnit.Scale(baseQty))
			usage[ing.ID] += baseQty

			if ing.VerificationStatus != ingredient.VerificationVerified {
				confidence.downgrade(ConfidenceMedium, fmt.Sprintf("ingredient %s is unverified", ing.ID))
			}
		case recipe.LineTargetRecipe:
			if line.Unit != ingredient.UnitEach {
				return ResolvedRecipe{}, fmt.Errorf("nested recipe %s must use unit %s", line.TargetID, ingredient.UnitEach)
			}

			child, err := c.resolveRecipe(line.TargetID, depth+1, stack)
			if err != nil {
				return ResolvedRecipe{}, err
			}

			total = total.Add(child.PerServing.Scale(line.Quantity))
			mergeUsage(usage, child.IngredientUsage, line.Quantity)
			confidence.merge(child.Confidence)
		default:
			return ResolvedRecipe{}, fmt.Errorf("unknown line target type %q", line.TargetType)
		}
	}

	yieldCount := current.YieldCount
	if yieldCount <= 0 {
		yieldCount = 1
		confidence.downgrade(ConfidenceLow, fmt.Sprintf("recipe %s has non-positive yield count", current.ID))
	}

	return ResolvedRecipe{
		TotalMacros:     total,
		PerServing:      total.Scale(1 / yieldCount),
		IngredientUsage: usage,
		Confidence:      confidence,
	}, nil
}

func (c *Confidence) downgrade(level ConfidenceLevel, reason string) {
	if severity(level) > severity(c.Level) {
		c.Level = level
	}
	if reason != "" {
		c.Reasons = append(c.Reasons, reason)
	}
}

func (c *Confidence) merge(other Confidence) {
	if severity(other.Level) > severity(c.Level) {
		c.Level = other.Level
	}
	c.Reasons = append(c.Reasons, other.Reasons...)
}

func severity(level ConfidenceLevel) int {
	switch level {
	case ConfidenceLow:
		return 2
	case ConfidenceMedium:
		return 1
	default:
		return 0
	}
}

func mergeUsage(dst map[string]float64, src map[string]float64, multiplier float64) {
	for ingredientID, qty := range src {
		dst[ingredientID] += qty * multiplier
	}
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
