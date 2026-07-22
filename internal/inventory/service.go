package inventory

import (
	"fmt"
	"sync"
	"time"

	"github.com/ruth411/circle/internal/core/ingredient"
	"github.com/ruth411/circle/internal/ordering"
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
	recorded  map[string]bool
	movements []Movement
}

func NewService() *Service {
	return &Service{
		recorded: map[string]bool{},
	}
}

func (s *Service) RecordDepletion(order ordering.Order) ([]Movement, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if order.Status != ordering.OrderStatusClosed {
		return nil, fmt.Errorf("order %s must be closed before depletion", order.ID)
	}

	if s.recorded[order.ID] {
		return nil, nil
	}

	var movements []Movement
	seq := 1
	occurredAt := time.Now().UTC()
	if order.ClosedAt != nil {
		occurredAt = order.ClosedAt.UTC()
	}

	for _, line := range order.Lines {
		for ingredientID, qty := range line.IngredientUsage {
			movement := Movement{
				ID:           fmt.Sprintf("%s-%d", order.ID, seq),
				OrderID:      order.ID,
				IngredientID: ingredientID,
				Quantity:     -qty,
				Unit:         ingredient.UnitEach, // ponytail: usage is already normalized to base units; attach per-ingredient base units once inventory reads the ingredient catalog.
				OccurredAt:   occurredAt,
			}
			seq++
			movements = append(movements, movement)
			s.movements = append(s.movements, movement)
		}
	}

	s.recorded[order.ID] = true
	return movements, nil
}

func (s *Service) Movements() []Movement {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]Movement, len(s.movements))
	copy(out, s.movements)
	return out
}
