package diner

import (
	"testing"
	"time"

	"github.com/ruth411/circle/internal/contracts"
	"github.com/ruth411/circle/internal/core/ingredient"
)

func TestClaimCanBeRevisedWithoutChangingOrder(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 7, 22, 9, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }

	order := contracts.ClosedOrder{
		OrderID:  "order-1",
		ClosedAt: now,
		Lines: []contracts.ClosedOrderLine{
			{LineID: "line-1", Name: "Bowl", Quantity: 1, ResolvedMacros: ingredient.MacroValues{Calories: 600, ProteinGrams: 40}},
			{LineID: "line-2", Name: "Cookie", Quantity: 1, ResolvedMacros: ingredient.MacroValues{Calories: 200, CarbsGrams: 30}},
		},
	}

	token, err := service.IssueToken(order)
	if err != nil {
		t.Fatalf("IssueToken returned error: %v", err)
	}

	claim, err := service.SubmitClaim("claim-1", token.Token, []string{"line-1"})
	if err != nil {
		t.Fatalf("SubmitClaim returned error: %v", err)
	}
	if claim.Totals.Calories != 600 {
		t.Fatalf("claim calories = %v, want 600", claim.Totals.Calories)
	}

	claim, err = service.SubmitClaim("claim-1", token.Token, []string{"line-1", "line-2"})
	if err != nil {
		t.Fatalf("SubmitClaim revision returned error: %v", err)
	}
	if claim.Totals.Calories != 800 {
		t.Fatalf("revised claim calories = %v, want 800", claim.Totals.Calories)
	}
}

func TestTokenExpiresAfterOneDay(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 7, 22, 9, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }

	token, err := service.IssueToken(contracts.ClosedOrder{
		OrderID:  "order-1",
		ClosedAt: now,
		Lines:    []contracts.ClosedOrderLine{{LineID: "line-1", Name: "Bowl", Quantity: 1}},
	})
	if err != nil {
		t.Fatalf("IssueToken returned error: %v", err)
	}

	service.now = func() time.Time { return now.Add(25 * time.Hour) }
	if _, err := service.ResolveToken(token.Token); err == nil {
		t.Fatal("expected token to expire")
	}
}

func TestClaimCanSplitMultiQuantityLineAcrossDiners(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 7, 22, 9, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }

	token, err := service.IssueToken(contracts.ClosedOrder{
		OrderID:  "order-1",
		ClosedAt: now,
		Lines: []contracts.ClosedOrderLine{
			{
				LineID:         "line-1",
				Name:           "Taco",
				Quantity:       2,
				ResolvedMacros: ingredient.MacroValues{Calories: 600, ProteinGrams: 40},
			},
		},
	})
	if err != nil {
		t.Fatalf("IssueToken returned error: %v", err)
	}

	if len(token.Items) != 2 {
		t.Fatalf("token item count = %d, want 2", len(token.Items))
	}
	if token.Items[0].ItemID == token.Items[1].ItemID {
		t.Fatal("expected split items to have distinct IDs")
	}
	if token.Items[0].Macros.Calories != 300 || token.Items[1].Macros.Calories != 300 {
		t.Fatalf("split calories = %v and %v, want 300 each", token.Items[0].Macros.Calories, token.Items[1].Macros.Calories)
	}

	claim, err := service.SubmitClaim("claim-1", token.Token, []string{token.Items[0].ItemID})
	if err != nil {
		t.Fatalf("SubmitClaim returned error: %v", err)
	}
	if claim.Totals.Calories != 300 {
		t.Fatalf("claim calories = %v, want 300", claim.Totals.Calories)
	}
}
