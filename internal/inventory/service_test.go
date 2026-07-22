package inventory

import (
	"testing"
	"time"

	"github.com/ruth411/circle/internal/core/ingredient"
	"github.com/ruth411/circle/internal/ordering"
)

func TestRecordDepletionIsAppendOnlyAndIdempotent(t *testing.T) {
	now := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)
	service := NewService()

	order := ordering.Order{
		ID:       "order-1",
		Status:   ordering.OrderStatusClosed,
		ClosedAt: &now,
		Lines: []ordering.OrderLine{
			{
				LineID:          "line-1",
				IngredientUsage: map[string]float64{"chicken": 150, "rice": 100},
			},
		},
	}

	movements, err := service.RecordDepletion(order)
	if err != nil {
		t.Fatalf("RecordDepletion returned error: %v", err)
	}
	if len(movements) != 2 {
		t.Fatalf("movement count = %d, want 2", len(movements))
	}
	if movements[0].Quantity >= 0 {
		t.Fatalf("movement quantity = %v, want negative", movements[0].Quantity)
	}
	if movements[0].Unit != ingredient.UnitEach {
		t.Fatalf("movement unit = %s, want placeholder base-unit marker", movements[0].Unit)
	}

	movements, err = service.RecordDepletion(order)
	if err != nil {
		t.Fatalf("second RecordDepletion returned error: %v", err)
	}
	if len(movements) != 0 {
		t.Fatalf("second call produced %d movements, want 0", len(movements))
	}
}
