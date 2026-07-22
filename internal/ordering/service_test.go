package ordering

import (
	"context"
	"testing"
	"time"

	"github.com/ruth411/circle/internal/core/ingredient"
	"github.com/ruth411/circle/internal/core/recipe"
)

func TestOrderLifecycleFreezesPaidOrder(t *testing.T) {
	service := NewService(MockProvider{})
	service.RegisterSnapshot(recipe.MenuSnapshot{
		ID:         "snap-1",
		LocationID: "loc-1",
		Version:    1,
		Items: []recipe.SnapshotItem{
			{
				MenuItemID:      "bowl",
				Name:            "Bowl",
				PriceMinor:      1200,
				Currency:        "USD",
				Macros:          ingredient.MacroValues{Calories: 500, ProteinGrams: 40},
				IngredientUsage: map[string]float64{"chicken": 150},
				ModifierGroups: []recipe.SnapshotModifierGroup{
					{
						GroupID:      "protein",
						SelectionMin: 1,
						SelectionMax: 1,
						Required:     true,
						Exclusive:    true,
						Modifiers: []recipe.SnapshotModifier{
							{
								ModifierID:      "extra",
								Name:            "Extra",
								PriceDeltaMinor: 250,
								Currency:        "USD",
								MacroDelta:      ingredient.MacroValues{Calories: 120, ProteinGrams: 12},
								IngredientUsage: map[string]float64{"chicken": 50},
							},
						},
					},
				},
			},
		},
	})

	order, err := service.CreateOrder(CreateOrderInput{
		OrderID:      "order-1",
		LocationID:   "loc-1",
		SnapshotID:   "snap-1",
		BusinessDate: time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("CreateOrder returned error: %v", err)
	}

	line, err := service.AddLine(AddLineInput{
		OrderID:     order.ID,
		MenuItemID:  "bowl",
		ModifierIDs: []string{"extra"},
		Quantity:    2,
	})
	if err != nil {
		t.Fatalf("AddLine returned error: %v", err)
	}

	if line.ResolvedPriceMinor != 2900 {
		t.Fatalf("line price = %d, want 2900", line.ResolvedPriceMinor)
	}
	if line.ResolvedMacros.Calories != 1240 {
		t.Fatalf("line calories = %v, want 1240", line.ResolvedMacros.Calories)
	}
	if line.IngredientUsage["chicken"] != 400 {
		t.Fatalf("line usage = %v, want 400", line.IngredientUsage["chicken"])
	}

	closed, err := service.CloseCheck(context.Background(), CloseCheckInput{
		OrderID: order.ID,
		Tender: Tender{
			ID:          "tender-1",
			CheckID:     order.CheckID,
			AmountMinor: 2900,
			Currency:    "USD",
			Kind:        "mock",
		},
	})
	if err != nil {
		t.Fatalf("CloseCheck returned error: %v", err)
	}

	if closed.Status != OrderStatusClosed {
		t.Fatalf("status = %s, want %s", closed.Status, OrderStatusClosed)
	}

	if _, err := service.AddLine(AddLineInput{OrderID: order.ID, MenuItemID: "bowl", ModifierIDs: []string{"extra"}, Quantity: 1}); err == nil {
		t.Fatal("expected AddLine to reject closed order")
	}
}
