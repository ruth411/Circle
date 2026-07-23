package inventory

import (
	"fmt"
	"sync"
	"time"

	"github.com/ruth411/circle/internal/contracts"
	"github.com/ruth411/circle/internal/core/ingredient"
)

type Movement struct {
	ID           string
	OrderID      string
	IngredientID string
	Quantity     float64
	Unit         ingredient.Unit
	OccurredAt   time.Time
}

type Service struct {
	mu        sync.Mutex
	baseUnits map[string]ingredient.Unit
	recorded  map[string]bool
	movements []Movement
}

func NewService(baseUnits map[string]ingredient.Unit) *Service {
	clonedBaseUnits := make(map[string]ingredient.Unit, len(baseUnits))
	for ingredientID, unit := range baseUnits {
		clonedBaseUnits[ingredientID] = unit
	}

	return &Service{
		baseUnits: clonedBaseUnits,
		recorded:  map[string]bool{},
	}
}

func (s *Service) RecordDepletion(order contracts.ClosedOrder) ([]Movement, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if order.ClosedAt.IsZero() {
		return nil, fmt.Errorf("order %s must have a closed timestamp", order.OrderID)
	}

	if s.recorded[order.OrderID] {
		return nil, nil
	}

	var movements []Movement
	seq := 1
	occurredAt := order.ClosedAt.UTC()

	for _, line := range order.Lines {
		for ingredientID, qty := range line.IngredientUsage {
			unit, ok := s.baseUnits[ingredientID]
			if !ok {
				return nil, fmt.Errorf("missing base unit for ingredient %s", ingredientID)
			}

			movement := Movement{
				ID:           fmt.Sprintf("%s-%d", order.OrderID, seq),
				OrderID:      order.OrderID,
				IngredientID: ingredientID,
				Quantity:     -qty,
				Unit:         unit,
				OccurredAt:   occurredAt,
			}
			seq++
			movements = append(movements, movement)
			s.movements = append(s.movements, movement)
		}
	}

	s.recorded[order.OrderID] = true
	return movements, nil
}

func (s *Service) Movements() []Movement {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]Movement, len(s.movements))
	copy(out, s.movements)
	return out
}
