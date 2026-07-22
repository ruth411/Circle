package recipe

import (
	"time"

	"github.com/ruth411/circle/internal/core/ingredient"
)

type LineTargetType string

const (
	LineTargetIngredient LineTargetType = "ingredient"
	LineTargetRecipe     LineTargetType = "recipe"
)

type RecipeLine struct {
	TargetType LineTargetType
	TargetID   string
	Quantity   float64
	Unit       ingredient.Unit
	PrepMethod string
}

type Recipe struct {
	ID         string
	LocationID string
	Name       string
	YieldCount float64
	Lines      []RecipeLine
}

type IngredientDelta struct {
	IngredientID string
	Quantity     float64
	Unit         ingredient.Unit
	PrepMethod   string
}

type Modifier struct {
	ID               string
	Name             string
	PriceDeltaMinor  int64
	Currency         string
	IngredientDeltas []IngredientDelta
}

type ModifierGroup struct {
	ID                 string
	Name               string
	SelectionMin       int
	SelectionMax       int
	Required           bool
	Exclusive          bool
	DefaultModifierIDs []string
	Modifiers          []Modifier
}

type MenuItem struct {
	ID             string
	RecipeID       string
	Name           string
	Description    string
	PriceMinor     int64
	Currency       string
	ModifierGroups []ModifierGroup
}

type SnapshotModifier struct {
	ModifierID      string
	Name            string
	PriceDeltaMinor int64
	Currency        string
	MacroDelta      ingredient.MacroValues
	IngredientUsage map[string]float64
}

type SnapshotModifierGroup struct {
	GroupID            string
	Name               string
	SelectionMin       int
	SelectionMax       int
	Required           bool
	Exclusive          bool
	DefaultModifierIDs []string
	Modifiers          []SnapshotModifier
}

type SnapshotItem struct {
	MenuItemID      string
	Name            string
	Description     string
	PriceMinor      int64
	Currency        string
	Macros          ingredient.MacroValues
	IngredientUsage map[string]float64
	ModifierGroups  []SnapshotModifierGroup
}

type MenuSnapshot struct {
	ID         string
	LocationID string
	Version    int
	CreatedAt  time.Time
	Items      []SnapshotItem
}
