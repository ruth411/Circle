package contracts

import (
	"time"

	"github.com/ruth411/circle/internal/core/ingredient"
)

type ClosedOrder struct {
	OrderID    string
	LocationID string
	ClosedAt   time.Time
	Lines      []ClosedOrderLine
}

const ClosedOrderEventName = "ordering.closed_order"

type ClosedOrderLine struct {
	LineID          string
	Name            string
	Quantity        int
	ResolvedMacros  ingredient.MacroValues
	IngredientUsage map[string]float64
	IngredientUnits map[string]ingredient.Unit
}
