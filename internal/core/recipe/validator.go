package recipe

import "fmt"

func ValidateRecipeGraph(rootID string, recipes map[string]Recipe, maxDepth int) error {
	if maxDepth < 1 {
		return fmt.Errorf("max depth must be at least 1")
	}

	if _, ok := recipes[rootID]; !ok {
		return fmt.Errorf("recipe %s not found", rootID)
	}

	stack := map[string]bool{}
	return validateRecipe(rootID, recipes, maxDepth, 1, stack)
}

func validateRecipe(recipeID string, recipes map[string]Recipe, maxDepth int, depth int, stack map[string]bool) error {
	if depth > maxDepth {
		return fmt.Errorf("recipe graph exceeds max depth %d", maxDepth)
	}

	if stack[recipeID] {
		return fmt.Errorf("recipe cycle detected at %s", recipeID)
	}

	current, ok := recipes[recipeID]
	if !ok {
		return fmt.Errorf("recipe %s not found", recipeID)
	}

	stack[recipeID] = true
	defer delete(stack, recipeID)

	for _, line := range current.Lines {
		if line.Quantity <= 0 {
			return fmt.Errorf("recipe %s has non-positive quantity", recipeID)
		}

		switch line.TargetType {
		case LineTargetIngredient:
			if line.TargetID == "" {
				return fmt.Errorf("recipe %s has ingredient line with empty target", recipeID)
			}
		case LineTargetRecipe:
			if line.TargetID == "" {
				return fmt.Errorf("recipe %s has nested recipe line with empty target", recipeID)
			}
			if err := validateRecipe(line.TargetID, recipes, maxDepth, depth+1, stack); err != nil {
				return err
			}
		default:
			return fmt.Errorf("recipe %s has unknown target type %q", recipeID, line.TargetType)
		}
	}

	return nil
}
