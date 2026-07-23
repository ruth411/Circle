package contracts

import (
	"time"

	"github.com/ruth411/circle/internal/core/ingredient"
)

type ClosedOrder struct {
	OrderID  string
	ClosedAt time.Time
	Lines    []ClosedOrderLine
}

type ClosedOrderLine struct {
	LineID          string
	Name            string
	Quantity        int
	ResolvedMacros  ingredient.MacroValues
	IngredientUsage map[string]float64
}
