package diner

import (
	"context"
	"errors"
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
		OrderID:    "order-1",
		LocationID: "loc-1",
		ClosedAt:   now,
		Lines: []contracts.ClosedOrderLine{
			{LineID: "line-1", Name: "Bowl", Quantity: 1, ResolvedMacros: ingredient.MacroValues{Calories: 600, ProteinGrams: 40}},
			{LineID: "line-2", Name: "Cookie", Quantity: 1, ResolvedMacros: ingredient.MacroValues{Calories: 200, CarbsGrams: 30}},
		},
	}

	token, err := service.IssueToken(context.Background(), order)
	if err != nil {
		t.Fatalf("IssueToken returned error: %v", err)
	}

	claim, err := service.SubmitClaim(context.Background(), "", token.Token, []string{token.Items[0].ItemID})
	if err != nil {
		t.Fatalf("SubmitClaim returned error: %v", err)
	}
	if claim.ID == "" {
		t.Fatal("claim id empty, want generated id")
	}
	if claim.Totals.Calories != 600 {
		t.Fatalf("claim calories = %v, want 600", claim.Totals.Calories)
	}

	claim, err = service.ReviseClaim(context.Background(), claim.ID, token.Token, []string{token.Items[0].ItemID, token.Items[1].ItemID})
	if err != nil {
		t.Fatalf("ReviseClaim returned error: %v", err)
	}
	if claim.Totals.Calories != 800 {
		t.Fatalf("revised claim calories = %v, want 800", claim.Totals.Calories)
	}
}

func TestIssueTokenIsIdempotentForSameClosedOrder(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 7, 29, 9, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }

	order := contracts.ClosedOrder{
		OrderID:    "order-1",
		LocationID: "loc-1",
		ClosedAt:   now,
		Lines: []contracts.ClosedOrderLine{
			{LineID: "line-1", Name: "Bowl", Quantity: 1, ResolvedMacros: ingredient.MacroValues{Calories: 600}},
		},
	}

	first, err := service.IssueToken(context.Background(), order)
	if err != nil {
		t.Fatalf("first IssueToken returned error: %v", err)
	}
	second, err := service.IssueToken(context.Background(), order)
	if err != nil {
		t.Fatalf("second IssueToken returned error: %v", err)
	}

	if first.Token != second.Token {
		t.Fatalf("token = %q then %q, want stable token for same order", first.Token, second.Token)
	}
}

func TestIssueTokenExpiresFromClosedAt(t *testing.T) {
	service := NewService()
	closedAt := time.Date(2026, 7, 29, 9, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return closedAt.Add(6 * time.Hour) }

	token, err := service.IssueToken(context.Background(), contracts.ClosedOrder{
		OrderID:    "order-1",
		LocationID: "loc-1",
		ClosedAt:   closedAt,
		Lines:      []contracts.ClosedOrderLine{{LineID: "line-1", Name: "Bowl", Quantity: 1}},
	})
	if err != nil {
		t.Fatalf("IssueToken returned error: %v", err)
	}

	want := closedAt.Add(TokenTTL)
	if !token.ExpiresAt.Equal(want) {
		t.Fatalf("expires_at = %s, want %s", token.ExpiresAt.Format(time.RFC3339), want.Format(time.RFC3339))
	}
}

func TestTokenExpiresAfterOneDay(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 7, 22, 9, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }

	token, err := service.IssueToken(context.Background(), contracts.ClosedOrder{
		OrderID:    "order-1",
		LocationID: "loc-1",
		ClosedAt:   now,
		Lines:      []contracts.ClosedOrderLine{{LineID: "line-1", Name: "Bowl", Quantity: 1}},
	})
	if err != nil {
		t.Fatalf("IssueToken returned error: %v", err)
	}

	service.now = func() time.Time { return now.Add(25 * time.Hour) }
	if _, err := service.ResolveToken(context.Background(), token.Token); err == nil {
		t.Fatal("expected token to expire")
	}
}

func TestTokenExpiresExactlyAtExpiryBoundary(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 7, 22, 9, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }

	token, err := service.IssueToken(context.Background(), contracts.ClosedOrder{
		OrderID:    "order-1",
		LocationID: "loc-1",
		ClosedAt:   now,
		Lines:      []contracts.ClosedOrderLine{{LineID: "line-1", Name: "Bowl", Quantity: 1}},
	})
	if err != nil {
		t.Fatalf("IssueToken returned error: %v", err)
	}

	service.now = func() time.Time { return token.ExpiresAt }
	if _, err := service.ResolveToken(context.Background(), token.Token); !errors.Is(err, ErrTokenExpired) {
		t.Fatalf("err = %v, want ErrTokenExpired", err)
	}
}

func TestClaimCanSplitMultiQuantityLineAcrossDiners(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 7, 22, 9, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }

	token, err := service.IssueToken(context.Background(), contracts.ClosedOrder{
		OrderID:    "order-1",
		LocationID: "loc-1",
		ClosedAt:   now,
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

	claim, err := service.SubmitClaim(context.Background(), "claim-1", token.Token, []string{token.Items[0].ItemID})
	if err != nil {
		t.Fatalf("SubmitClaim returned error: %v", err)
	}
	if claim.Totals.Calories != 300 {
		t.Fatalf("claim calories = %v, want 300", claim.Totals.Calories)
	}
}

func TestSubmitClaimRejectsClaimIDReuse(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 7, 22, 9, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }

	token, err := service.IssueToken(context.Background(), contracts.ClosedOrder{
		OrderID:    "order-1",
		LocationID: "loc-1",
		ClosedAt:   now,
		Lines:      []contracts.ClosedOrderLine{{LineID: "line-1", Name: "Bowl", Quantity: 1}},
	})
	if err != nil {
		t.Fatalf("IssueToken returned error: %v", err)
	}

	if _, err := service.SubmitClaim(context.Background(), "claim-1", token.Token, []string{token.Items[0].ItemID}); err != nil {
		t.Fatalf("first SubmitClaim returned error: %v", err)
	}
	if _, err := service.SubmitClaim(context.Background(), "claim-1", token.Token, []string{token.Items[0].ItemID}); !errors.Is(err, ErrClaimAlreadyExists) {
		t.Fatalf("err = %v, want ErrClaimAlreadyExists", err)
	}
}

func TestExpiredTokenRejectsClaimRevision(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 7, 22, 9, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }

	token, err := service.IssueToken(context.Background(), contracts.ClosedOrder{
		OrderID:    "order-1",
		LocationID: "loc-1",
		ClosedAt:   now,
		Lines:      []contracts.ClosedOrderLine{{LineID: "line-1", Name: "Bowl", Quantity: 1}},
	})
	if err != nil {
		t.Fatalf("IssueToken returned error: %v", err)
	}
	claim, err := service.SubmitClaim(context.Background(), "", token.Token, []string{token.Items[0].ItemID})
	if err != nil {
		t.Fatalf("SubmitClaim returned error: %v", err)
	}

	service.now = func() time.Time { return now.Add(25 * time.Hour) }
	if _, err := service.ReviseClaim(context.Background(), claim.ID, token.Token, []string{token.Items[0].ItemID}); err == nil {
		t.Fatal("expected claim revision to fail after expiry")
	}
}

func TestTokenExpiryBoundaryRejectsClaimCreateAndRevision(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 7, 22, 9, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }

	token, err := service.IssueToken(context.Background(), contracts.ClosedOrder{
		OrderID:    "order-1",
		LocationID: "loc-1",
		ClosedAt:   now,
		Lines: []contracts.ClosedOrderLine{
			{LineID: "line-1", Name: "Bowl", Quantity: 1},
			{LineID: "line-2", Name: "Cookie", Quantity: 1},
		},
	})
	if err != nil {
		t.Fatalf("IssueToken returned error: %v", err)
	}

	claim, err := service.SubmitClaim(context.Background(), "claim-1", token.Token, []string{token.Items[0].ItemID})
	if err != nil {
		t.Fatalf("initial SubmitClaim returned error: %v", err)
	}

	service.now = func() time.Time { return token.ExpiresAt }
	if _, err := service.SubmitClaim(context.Background(), "claim-2", token.Token, []string{token.Items[1].ItemID}); !errors.Is(err, ErrTokenExpired) {
		t.Fatalf("submit err = %v, want ErrTokenExpired", err)
	}
	if _, err := service.ReviseClaim(context.Background(), claim.ID, token.Token, []string{token.Items[0].ItemID, token.Items[1].ItemID}); !errors.Is(err, ErrTokenExpired) {
		t.Fatalf("revise err = %v, want ErrTokenExpired", err)
	}
}

func TestIssueTokenRejectsMalformedClosedOrderLine(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 7, 29, 9, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }

	_, err := service.IssueToken(context.Background(), contracts.ClosedOrder{
		OrderID:    "order-1",
		LocationID: "loc-1",
		ClosedAt:   now,
		Lines: []contracts.ClosedOrderLine{
			{LineID: "line-1", Name: "Broken", Quantity: 0},
		},
	})
	if err == nil {
		t.Fatal("expected malformed closed-order line to fail")
	}
}

func TestIssueTokenRejectsDuplicateGeneratedItemIDs(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 7, 29, 9, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }

	token, err := service.IssueToken(context.Background(), contracts.ClosedOrder{
		OrderID:    "order-1",
		LocationID: "loc-1",
		ClosedAt:   now,
		Lines: []contracts.ClosedOrderLine{
			{LineID: "line-1", Name: "Bowl A", Quantity: 2},
			{LineID: "line-1#1", Name: "Bowl B", Quantity: 1},
		},
	})
	if err != nil {
		t.Fatalf("IssueToken returned error: %v", err)
	}
	if len(token.Items) != 3 {
		t.Fatalf("item count = %d, want 3", len(token.Items))
	}
	seen := map[string]bool{}
	for _, item := range token.Items {
		if seen[item.ItemID] {
			t.Fatalf("duplicate item id %q", item.ItemID)
		}
		seen[item.ItemID] = true
	}
}
