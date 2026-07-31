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
			ID:           movementID(movementSourceClosedOrd, order.OrderID, i+1),
			LocationID:   order.LocationID,
			SourceType:   movementSourceClosedOrd,
			SourceID:     order.OrderID,
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

func (r *memoryRepository) RecordReceipt(_ context.Context, receipt contracts.PurchaseReceipt) ([]Movement, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := receipt.LocationID + "|" + receipt.SourceType + "|" + receipt.SourceID
	if r.recorded[key] {
		return nil, nil
	}

	usage, units, err := aggregateReceiptLines(receipt)
	if err != nil {
		return nil, err
	}
	ingredientIDs := sortedIngredientIDs(usage)

	var movements []Movement
	for i, ingredientID := range ingredientIDs {
		unit, ok := units[ingredientID]
		if !ok {
			return nil, fmt.Errorf("%w: missing unit snapshot for ingredient %s", ErrInvalidReceiptData, ingredientID)
		}
		movement := Movement{
			ID:           movementID(receipt.SourceType, receipt.SourceID, i+1),
			LocationID:   receipt.LocationID,
			SourceType:   receipt.SourceType,
			SourceID:     receipt.SourceID,
			IngredientID: ingredientID,
			Quantity:     usage[ingredientID],
			Unit:         unit,
			OccurredAt:   receipt.OccurredAt.UTC(),
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
				return nil, nil, fmt.Errorf("%w: missing unit snapshot for ingredient %s", ErrInvalidClosedOrderData, ingredientID)
			}
			usage[ingredientID] += qty
			units[ingredientID] = unit
		}
	}
	return usage, units, nil
}

func aggregateReceiptLines(receipt contracts.PurchaseReceipt) (map[string]float64, map[string]ingredient.Unit, error) {
	usage := map[string]float64{}
	units := map[string]ingredient.Unit{}
	for _, line := range receipt.Lines {
		if unit, ok := units[line.IngredientID]; ok && unit != line.Unit {
			return nil, nil, fmt.Errorf("%w: ingredient %s has conflicting units %s and %s", ErrInvalidReceiptData, line.IngredientID, unit, line.Unit)
		}
		usage[line.IngredientID] += line.QuantityBaseUnits
		units[line.IngredientID] = line.Unit
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

func movementID(sourceType string, sourceID string, index int) string {
	return fmt.Sprintf("%s-%s-%d", sourceType, sourceID, index)
}
