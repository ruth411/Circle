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
	TotalMacros         ingredient.MacroValues
	PerServing          ingredient.MacroValues
	TotalCostMinor      int64
	PerServingCostMinor int64
	IngredientUsage     map[string]float64
	IngredientUnits     map[string]ingredient.Unit
	Confidence          Confidence

	totalCostMinorExact      float64
	perServingCostMinorExact float64
}

type ResolvedModifier struct {
	MacroDelta      ingredient.MacroValues
	CostDeltaMinor  int64
	IngredientUsage map[string]float64
	IngredientUnits map[string]ingredient.Unit
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
	totalCost := 0.0
	usage := map[string]float64{}
	units := map[string]ingredient.Unit{}

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
		totalCost += float64(ing.CurrentCostMinor) * signedQty
		usage[ing.ID] += signedQty
		units[ing.ID] = ing.BaseUnit

		if ing.VerificationStatus != ingredient.VerificationVerified {
			confidence.downgrade(ConfidenceMedium, fmt.Sprintf("ingredient %s is unverified", ing.ID))
		}
	}

	return ResolvedModifier{
		MacroDelta:      total,
		CostDeltaMinor:  roundMinor(totalCost),
		IngredientUsage: usage,
		IngredientUnits: units,
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
	totalCost := 0.0
	usage := map[string]float64{}
	units := map[string]ingredient.Unit{}

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
			totalCost += float64(ing.CurrentCostMinor) * baseQty
			usage[ing.ID] += baseQty
			units[ing.ID] = ing.BaseUnit

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
			totalCost += child.perServingCostMinorExact * line.Quantity
			mergeUsage(usage, child.IngredientUsage, line.Quantity)
			mergeUnits(units, child.IngredientUnits)
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

	perServingCost := totalCost / yieldCount

	return ResolvedRecipe{
		TotalMacros:              total,
		PerServing:               total.Scale(1 / yieldCount),
		TotalCostMinor:           roundMinor(totalCost),
		PerServingCostMinor:      roundMinor(perServingCost),
		IngredientUsage:          usage,
		IngredientUnits:          units,
		Confidence:               confidence,
		totalCostMinorExact:      totalCost,
		perServingCostMinorExact: perServingCost,
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

func mergeUnits(dst map[string]ingredient.Unit, src map[string]ingredient.Unit) {
	for ingredientID, unit := range src {
		dst[ingredientID] = unit
	}
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func roundMinor(value float64) int64 {
	if value < 0 {
		return int64(value - 0.5)
	}
	return int64(value + 0.5)
}
