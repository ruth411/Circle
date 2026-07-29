package inventory

import (
	"context"
	"fmt"
	"slices"
	"sync"

	"github.com/ruth411/circle/internal/contracts"
	"github.com/ruth411/circle/internal/core/ingredient"
)

type memoryRepository struct {
	mu        sync.Mutex
	recorded  map[string]bool
	movements []Movement
}

func newMemoryRepository(baseUnits map[string]ingredient.Unit) *memoryRepository {
	return &memoryRepository{
		recorded: map[string]bool{},
	}
}

func (r *memoryRepository) RecordDepletion(_ context.Context, order contracts.ClosedOrder) ([]Movement, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := order.LocationID + "|" + order.OrderID
	if r.recorded[key] {
		return nil, nil
	}

	usage, units, err := aggregateUsageAndUnits(order)
	if err != nil {
		return nil, err
	}
	ingredientIDs := sortedIngredientIDs(usage)

	var movements []Movement
	for i, ingredientID := range ingredientIDs {
		unit, ok := units[ingredientID]
		if !ok {
			return nil, fmt.Errorf("missing unit snapshot for ingredient %s", ingredientID)
		}
		movement := Movement{
			ID:           fmt.Sprintf("%s-%d", order.OrderID, i+1),
			LocationID:   order.LocationID,
			OrderID:      order.OrderID,
			IngredientID: ingredientID,
			Quantity:     -usage[ingredientID],
			Unit:         unit,
			OccurredAt:   order.ClosedAt.UTC(),
		}
		movements = append(movements, movement)
		r.movements = append(r.movements, movement)
	}

	r.recorded[key] = true
	return movements, nil
}

func (r *memoryRepository) ListMovements(_ context.Context, locationID string) ([]Movement, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var out []Movement
	for _, movement := range r.movements {
		if movement.LocationID == locationID {
			out = append(out, movement)
		}
	}
	return out, nil
}

func aggregateUsageAndUnits(order contracts.ClosedOrder) (map[string]float64, map[string]ingredient.Unit, error) {
	usage := map[string]float64{}
	units := map[string]ingredient.Unit{}
	for _, line := range order.Lines {
		for ingredientID, qty := range line.IngredientUsage {
			unit, ok := line.IngredientUnits[ingredientID]
			if !ok {
				return nil, nil, fmt.Errorf("missing unit snapshot for ingredient %s", ingredientID)
			}
			usage[ingredientID] += qty
			units[ingredientID] = unit
		}
	}
	return usage, units, nil
}

func sortedIngredientIDs(usage map[string]float64) []string {
	ingredientIDs := make([]string, 0, len(usage))
	for ingredientID, qty := range usage {
		if qty == 0 {
			continue
		}
		ingredientIDs = append(ingredientIDs, ingredientID)
	}
	slices.Sort(ingredientIDs)
	return ingredientIDs
}
