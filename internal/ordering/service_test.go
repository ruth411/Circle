package ordering

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ruth411/circle/internal/core/ingredient"
	"github.com/ruth411/circle/internal/core/recipe"
	"github.com/ruth411/circle/internal/platform/biztime"
)

func TestOrderLifecycleFreezesPaidOrder(t *testing.T) {
	service := NewService(MockProvider{})
	if err := service.RegisterSnapshot(recipe.MenuSnapshot{
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
	}); err != nil {
		t.Fatalf("RegisterSnapshot returned error: %v", err)
	}

	order, err := service.CreateOrder(context.Background(), CreateOrderInput{
		OrderID:      "order-1",
		LocationID:   "loc-1",
		SnapshotID:   "snap-1",
		BusinessDate: biztime.FromTime(time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC)),
	})
	if err != nil {
		t.Fatalf("CreateOrder returned error: %v", err)
	}

	line, err := service.AddLine(context.Background(), AddLineInput{
		LocationID:  "loc-1",
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
		LocationID: "loc-1",
		OrderID:    order.ID,
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

	if _, err := service.AddLine(context.Background(), AddLineInput{LocationID: "loc-1", OrderID: order.ID, MenuItemID: "bowl", ModifierIDs: []string{"extra"}, Quantity: 1}); err == nil {
		t.Fatal("expected AddLine to reject closed order")
	}
}

func TestCloseCheckMarksOrderClosingDuringPayment(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})

	service := NewService(blockingProvider{
		started: started,
		release: release,
	})
	if err := service.RegisterSnapshot(recipe.MenuSnapshot{
		ID:         "snap-1",
		LocationID: "loc-1",
		Version:    1,
		Items: []recipe.SnapshotItem{
			{
				MenuItemID:      "bowl",
				Name:            "Bowl",
				PriceMinor:      1200,
				Currency:        "USD",
				Macros:          ingredient.MacroValues{Calories: 500},
				IngredientUsage: map[string]float64{"chicken": 150},
			},
		},
	}); err != nil {
		t.Fatalf("RegisterSnapshot returned error: %v", err)
	}

	order, err := service.CreateOrder(context.Background(), CreateOrderInput{
		OrderID:      "order-1",
		LocationID:   "loc-1",
		SnapshotID:   "snap-1",
		BusinessDate: biztime.FromTime(time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC)),
	})
	if err != nil {
		t.Fatalf("CreateOrder returned error: %v", err)
	}

	if _, err := service.AddLine(context.Background(), AddLineInput{
		LocationID: "loc-1",
		OrderID:    order.ID,
		MenuItemID: "bowl",
		Quantity:   1,
	}); err != nil {
		t.Fatalf("AddLine returned error: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		_, err := service.CloseCheck(context.Background(), CloseCheckInput{
			LocationID: "loc-1",
			OrderID:    order.ID,
			Tender: Tender{
				ID:          "tender-1",
				CheckID:     order.CheckID,
				AmountMinor: 1200,
				Currency:    "USD",
				Kind:        "mock",
			},
		})
		done <- err
	}()

	<-started

	if _, err := service.AddLine(context.Background(), AddLineInput{LocationID: "loc-1", OrderID: order.ID, MenuItemID: "bowl", Quantity: 1}); err == nil {
		t.Fatal("expected AddLine to reject order while payment is in progress")
	}

	close(release)

	if err := <-done; err != nil {
		t.Fatalf("CloseCheck returned error: %v", err)
	}
}

func TestCloseCheckReopensOrderOnPaymentFailure(t *testing.T) {
	service := NewService(MockProvider{Err: errors.New("declined")})
	if err := service.RegisterSnapshot(recipe.MenuSnapshot{
		ID:         "snap-1",
		LocationID: "loc-1",
		Version:    1,
		Items: []recipe.SnapshotItem{
			{
				MenuItemID:      "bowl",
				Name:            "Bowl",
				PriceMinor:      1200,
				Currency:        "USD",
				Macros:          ingredient.MacroValues{Calories: 500},
				IngredientUsage: map[string]float64{"chicken": 150},
			},
		},
	}); err != nil {
		t.Fatalf("RegisterSnapshot returned error: %v", err)
	}

	order, err := service.CreateOrder(context.Background(), CreateOrderInput{
		OrderID:      "order-1",
		LocationID:   "loc-1",
		SnapshotID:   "snap-1",
		BusinessDate: biztime.FromTime(time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC)),
	})
	if err != nil {
		t.Fatalf("CreateOrder returned error: %v", err)
	}

	if _, err := service.AddLine(context.Background(), AddLineInput{
		LocationID: "loc-1",
		OrderID:    order.ID,
		MenuItemID: "bowl",
		Quantity:   1,
	}); err != nil {
		t.Fatalf("AddLine returned error: %v", err)
	}

	if _, err := service.CloseCheck(context.Background(), CloseCheckInput{
		LocationID: "loc-1",
		OrderID:    order.ID,
		Tender: Tender{
			ID:          "tender-1",
			CheckID:     order.CheckID,
			AmountMinor: 1200,
			Currency:    "USD",
			Kind:        "mock",
		},
	}); err == nil {
		t.Fatal("expected CloseCheck to return payment error")
	}

	if _, err := service.AddLine(context.Background(), AddLineInput{LocationID: "loc-1", OrderID: order.ID, MenuItemID: "bowl", Quantity: 1}); err != nil {
		t.Fatalf("expected order to reopen after payment failure, got %v", err)
	}
}

type failFinishOnceRepository struct {
	Repository
	failErr error
	calls   int
}

func (r *failFinishOnceRepository) FinishClose(ctx context.Context, locationID string, orderID string, tenderID string, closedAt time.Time) (Order, error) {
	r.calls++
	if r.calls == 1 {
		return Order{}, r.failErr
	}
	return r.Repository.FinishClose(ctx, locationID, orderID, tenderID, closedAt)
}

type countingProvider struct {
	calls int
}

func (p *countingProvider) Process(_ context.Context, _ Tender) error {
	p.calls++
	return nil
}

type failMarkTenderOnceRepository struct {
	Repository
	failErr error
	calls   int
}

func (r *failMarkTenderOnceRepository) MarkTenderSucceeded(ctx context.Context, locationID string, orderID string, tenderID string) error {
	r.calls++
	if r.calls == 1 {
		return r.failErr
	}
	return r.Repository.MarkTenderSucceeded(ctx, locationID, orderID, tenderID)
}

func TestCloseCheckFinishesChargedOrderOnRetryWithoutRecharging(t *testing.T) {
	payment := &countingProvider{}
	repo := &failFinishOnceRepository{
		Repository: newMemoryRepository(),
		failErr:    errors.New("transient close failure"),
	}
	service := NewServiceWithDependencies(repo, newMemorySnapshotStore(), payment)
	if err := service.RegisterSnapshot(recipe.MenuSnapshot{
		ID:         "snap-1",
		LocationID: "loc-1",
		Version:    1,
		Items: []recipe.SnapshotItem{
			{
				MenuItemID:      "bowl",
				Name:            "Bowl",
				PriceMinor:      1200,
				Currency:        "USD",
				Macros:          ingredient.MacroValues{Calories: 500},
				IngredientUsage: map[string]float64{"chicken": 150},
			},
		},
	}); err != nil {
		t.Fatalf("RegisterSnapshot returned error: %v", err)
	}

	order, err := service.CreateOrder(context.Background(), CreateOrderInput{
		OrderID:      "order-1",
		LocationID:   "loc-1",
		SnapshotID:   "snap-1",
		BusinessDate: biztime.FromTime(time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC)),
	})
	if err != nil {
		t.Fatalf("CreateOrder returned error: %v", err)
	}
	if _, err := service.AddLine(context.Background(), AddLineInput{
		LocationID: "loc-1",
		OrderID:    order.ID,
		MenuItemID: "bowl",
		Quantity:   1,
	}); err != nil {
		t.Fatalf("AddLine returned error: %v", err)
	}

	_, err = service.CloseCheck(context.Background(), CloseCheckInput{
		LocationID: "loc-1",
		OrderID:    order.ID,
		Tender: Tender{
			ID:          "tender-1",
			CheckID:     order.CheckID,
			AmountMinor: 1200,
			Currency:    "USD",
			Kind:        "mock",
		},
	})
	if err == nil {
		t.Fatal("expected first CloseCheck to fail finalization")
	}

	closed, err := service.CloseCheck(context.Background(), CloseCheckInput{
		LocationID: "loc-1",
		OrderID:    order.ID,
		Tender: Tender{
			ID:          "tender-1",
			CheckID:     order.CheckID,
			AmountMinor: 1200,
			Currency:    "USD",
			Kind:        "mock",
		},
	})
	if err != nil {
		t.Fatalf("expected retry to close charged order, got %v", err)
	}
	if closed.Status != OrderStatusClosed {
		t.Fatalf("status = %s, want %s", closed.Status, OrderStatusClosed)
	}
	if payment.calls != 1 {
		t.Fatalf("payment calls = %d, want 1", payment.calls)
	}
}

func TestCloseCheckRecoversWhenTenderSuccessWriteFailsOnce(t *testing.T) {
	payment := &countingProvider{}
	repo := &failMarkTenderOnceRepository{
		Repository: newMemoryRepository(),
		failErr:    errors.New("transient mark failure"),
	}
	service := NewServiceWithDependencies(repo, newMemorySnapshotStore(), payment)
	if err := service.RegisterSnapshot(recipe.MenuSnapshot{
		ID:         "snap-1",
		LocationID: "loc-1",
		Version:    1,
		Items: []recipe.SnapshotItem{
			{
				MenuItemID:      "bowl",
				Name:            "Bowl",
				PriceMinor:      1200,
				Currency:        "USD",
				Macros:          ingredient.MacroValues{Calories: 500},
				IngredientUsage: map[string]float64{"chicken": 150},
			},
		},
	}); err != nil {
		t.Fatalf("RegisterSnapshot returned error: %v", err)
	}

	order, err := service.CreateOrder(context.Background(), CreateOrderInput{
		OrderID:      "order-1",
		LocationID:   "loc-1",
		SnapshotID:   "snap-1",
		BusinessDate: biztime.FromTime(time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC)),
	})
	if err != nil {
		t.Fatalf("CreateOrder returned error: %v", err)
	}
	if _, err := service.AddLine(context.Background(), AddLineInput{
		LocationID: "loc-1",
		OrderID:    order.ID,
		MenuItemID: "bowl",
		Quantity:   1,
	}); err != nil {
		t.Fatalf("AddLine returned error: %v", err)
	}

	input := CloseCheckInput{
		LocationID: "loc-1",
		OrderID:    order.ID,
		Tender: Tender{
			ID:          "tender-1",
			CheckID:     order.CheckID,
			AmountMinor: 1200,
			Currency:    "USD",
			Kind:        "mock",
		},
	}
	if _, err := service.CloseCheck(context.Background(), input); err == nil {
		t.Fatal("expected first CloseCheck to fail on tender success write")
	}

	closed, err := service.CloseCheck(context.Background(), input)
	if err != nil {
		t.Fatalf("expected retry to recover charged order, got %v", err)
	}
	if closed.Status != OrderStatusClosed {
		t.Fatalf("status = %s, want %s", closed.Status, OrderStatusClosed)
	}
	if payment.calls != 1 {
		t.Fatalf("payment calls = %d, want 1", payment.calls)
	}
}

type blockingProvider struct {
	started chan<- struct{}
	release <-chan struct{}
}

func (b blockingProvider) Process(_ context.Context, _ Tender) error {
	close(b.started)
	<-b.release
	return nil
}

type failCloseRepository struct {
	Repository
	failErr error
}

func (r failCloseRepository) FailClose(_ context.Context, _ string, _ string, _ string) error {
	return r.failErr
}

func TestCloseCheckSurfacesRollbackFailureAfterPaymentFailure(t *testing.T) {
	rollbackErr := errors.New("db unavailable")
	service := NewServiceWithDependencies(
		failCloseRepository{
			Repository: newMemoryRepository(),
			failErr:    rollbackErr,
		},
		newMemorySnapshotStore(),
		MockProvider{Err: errors.New("declined")},
	)
	if err := service.RegisterSnapshot(recipe.MenuSnapshot{
		ID:         "snap-1",
		LocationID: "loc-1",
		Version:    1,
		Items: []recipe.SnapshotItem{
			{
				MenuItemID:      "bowl",
				Name:            "Bowl",
				PriceMinor:      1200,
				Currency:        "USD",
				Macros:          ingredient.MacroValues{Calories: 500},
				IngredientUsage: map[string]float64{"chicken": 150},
			},
		},
	}); err != nil {
		t.Fatalf("RegisterSnapshot returned error: %v", err)
	}

	order, err := service.CreateOrder(context.Background(), CreateOrderInput{
		OrderID:      "order-1",
		LocationID:   "loc-1",
		SnapshotID:   "snap-1",
		BusinessDate: biztime.FromTime(time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC)),
	})
	if err != nil {
		t.Fatalf("CreateOrder returned error: %v", err)
	}

	if _, err := service.AddLine(context.Background(), AddLineInput{
		LocationID: "loc-1",
		OrderID:    order.ID,
		MenuItemID: "bowl",
		Quantity:   1,
	}); err != nil {
		t.Fatalf("AddLine returned error: %v", err)
	}

	_, err = service.CloseCheck(context.Background(), CloseCheckInput{
		LocationID: "loc-1",
		OrderID:    order.ID,
		Tender: Tender{
			ID:          "tender-1",
			CheckID:     order.CheckID,
			AmountMinor: 1200,
			Currency:    "USD",
			Kind:        "mock",
		},
	})
	if err == nil {
		t.Fatal("expected CloseCheck to return rollback failure")
	}
	if !errors.Is(err, ErrPaymentFailed) {
		t.Fatalf("expected payment failure wrapper, got %v", err)
	}
	if !errors.Is(err, rollbackErr) {
		t.Fatalf("expected rollback failure wrapper, got %v", err)
	}
}

func TestCloseCheckRejectsSecondCloseWhilePaymentInProgress(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})

	service := NewService(blockingProvider{
		started: started,
		release: release,
	})
	if err := service.RegisterSnapshot(recipe.MenuSnapshot{
		ID:         "snap-1",
		LocationID: "loc-1",
		Version:    1,
		Items: []recipe.SnapshotItem{
			{
				MenuItemID: "bowl",
				Name:       "Bowl",
				PriceMinor: 1200,
				Currency:   "USD",
			},
		},
	}); err != nil {
		t.Fatalf("RegisterSnapshot returned error: %v", err)
	}

	order, err := service.CreateOrder(context.Background(), CreateOrderInput{
		OrderID:      "order-1",
		LocationID:   "loc-1",
		SnapshotID:   "snap-1",
		BusinessDate: biztime.FromTime(time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC)),
	})
	if err != nil {
		t.Fatalf("CreateOrder returned error: %v", err)
	}

	if _, err := service.AddLine(context.Background(), AddLineInput{
		LocationID: "loc-1",
		OrderID:    order.ID,
		MenuItemID: "bowl",
		Quantity:   1,
	}); err != nil {
		t.Fatalf("AddLine returned error: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		_, err := service.CloseCheck(context.Background(), CloseCheckInput{
			LocationID: "loc-1",
			OrderID:    order.ID,
			Tender: Tender{
				ID:          "tender-1",
				CheckID:     order.CheckID,
				AmountMinor: 1200,
				Currency:    "USD",
				Kind:        "mock",
			},
		})
		done <- err
	}()

	<-started

	if _, err := service.CloseCheck(context.Background(), CloseCheckInput{
		LocationID: "loc-1",
		OrderID:    order.ID,
		Tender: Tender{
			ID:          "tender-2",
			CheckID:     order.CheckID,
			AmountMinor: 1200,
			Currency:    "USD",
			Kind:        "mock",
		},
	}); err == nil || !strings.Contains(err.Error(), "already closing") {
		t.Fatalf("expected second close to be rejected, got %v", err)
	}

	close(release)

	if err := <-done; err != nil {
		t.Fatalf("first CloseCheck returned error: %v", err)
	}
}

func TestCreateOrderRejectsSnapshotLocationMismatch(t *testing.T) {
	service := NewService(MockProvider{})
	if err := service.RegisterSnapshot(recipe.MenuSnapshot{
		ID:         "snap-1",
		LocationID: "loc-2",
		Version:    1,
	}); err != nil {
		t.Fatalf("RegisterSnapshot returned error: %v", err)
	}

	_, err := service.CreateOrder(context.Background(), CreateOrderInput{
		OrderID:      "order-1",
		LocationID:   "loc-1",
		SnapshotID:   "snap-1",
		BusinessDate: biztime.FromTime(time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC)),
	})
	if !errors.Is(err, recipe.ErrSnapshotNotFound) {
		t.Fatalf("expected snapshot not found, got %v", err)
	}
}

func TestCreateOrderRejectsMismatchedDuplicateAttributes(t *testing.T) {
	service := NewService(MockProvider{})
	if err := service.RegisterSnapshot(recipe.MenuSnapshot{
		ID:         "snap-1",
		LocationID: "loc-1",
		Version:    1,
	}); err != nil {
		t.Fatalf("RegisterSnapshot returned error: %v", err)
	}

	input := CreateOrderInput{
		OrderID:      "order-1",
		CheckID:      "check-1",
		LocationID:   "loc-1",
		SnapshotID:   "snap-1",
		BusinessDate: biztime.FromTime(time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC)),
	}
	if _, err := service.CreateOrder(context.Background(), input); err != nil {
		t.Fatalf("CreateOrder returned error: %v", err)
	}

	_, err := service.CreateOrder(context.Background(), CreateOrderInput{
		OrderID:      "order-1",
		CheckID:      "check-2",
		LocationID:   "loc-1",
		SnapshotID:   "snap-1",
		BusinessDate: input.BusinessDate,
	})
	if err == nil {
		t.Fatal("expected duplicate order with different check ID to fail")
	}

	_, err = service.CreateOrder(context.Background(), CreateOrderInput{
		OrderID:      "order-1",
		CheckID:      "check-1",
		LocationID:   "loc-1",
		SnapshotID:   "snap-1",
		BusinessDate: biztime.FromTime(time.Date(2026, 7, 23, 0, 0, 0, 0, time.UTC)),
	})
	if err == nil {
		t.Fatal("expected duplicate order with different business date to fail")
	}
}

func TestCreateOrderKeepsBusinessDateAsCalendarDay(t *testing.T) {
	service := NewService(MockProvider{})
	if err := service.RegisterSnapshot(recipe.MenuSnapshot{
		ID:         "snap-1",
		LocationID: "loc-1",
		Version:    1,
	}); err != nil {
		t.Fatalf("RegisterSnapshot returned error: %v", err)
	}

	order, err := service.CreateOrder(context.Background(), CreateOrderInput{
		OrderID:      "order-1",
		LocationID:   "loc-1",
		SnapshotID:   "snap-1",
		BusinessDate: biztime.BusinessDate("2026-07-22"),
	})
	if err != nil {
		t.Fatalf("CreateOrder returned error: %v", err)
	}
	if order.BusinessDate != biztime.BusinessDate("2026-07-22") {
		t.Fatalf("business date = %s, want 2026-07-22", order.BusinessDate)
	}
}

func TestAddLineRejectsDuplicateLineID(t *testing.T) {
	service := NewService(MockProvider{})
	if err := service.RegisterSnapshot(recipe.MenuSnapshot{
		ID:         "snap-1",
		LocationID: "loc-1",
		Version:    1,
		Items: []recipe.SnapshotItem{
			{
				MenuItemID: "bowl",
				Name:       "Bowl",
				PriceMinor: 1200,
				Currency:   "USD",
			},
		},
	}); err != nil {
		t.Fatalf("RegisterSnapshot returned error: %v", err)
	}

	order, err := service.CreateOrder(context.Background(), CreateOrderInput{
		OrderID:      "order-1",
		LocationID:   "loc-1",
		SnapshotID:   "snap-1",
		BusinessDate: biztime.BusinessDate("2026-07-22"),
	})
	if err != nil {
		t.Fatalf("CreateOrder returned error: %v", err)
	}

	if _, err := service.AddLine(context.Background(), AddLineInput{
		LocationID: "loc-1",
		OrderID:    order.ID,
		LineID:     "line-1",
		MenuItemID: "bowl",
		Quantity:   1,
	}); err != nil {
		t.Fatalf("first AddLine returned error: %v", err)
	}

	_, err = service.AddLine(context.Background(), AddLineInput{
		LocationID: "loc-1",
		OrderID:    order.ID,
		LineID:     "line-1",
		MenuItemID: "bowl",
		Quantity:   1,
	})
	if !errors.Is(err, ErrInvalidOrder) {
		t.Fatalf("expected duplicate line id to return invalid order, got %v", err)
	}
}

func TestRegisterSnapshotRejectsUnsupportedSnapshotStore(t *testing.T) {
	service := NewServiceWithDependencies(newMemoryRepository(), snapshotLookupFunc(func(context.Context, string) (recipe.MenuSnapshot, error) {
		return recipe.MenuSnapshot{}, recipe.ErrSnapshotNotFound
	}), MockProvider{})

	err := service.RegisterSnapshot(recipe.MenuSnapshot{ID: "snap-1"})
	if err == nil {
		t.Fatal("expected RegisterSnapshot to reject unsupported snapshot store")
	}
}

type snapshotLookupFunc func(context.Context, string) (recipe.MenuSnapshot, error)

func (f snapshotLookupFunc) GetSnapshot(ctx context.Context, _ string, snapshotID string) (recipe.MenuSnapshot, error) {
	return f(ctx, snapshotID)
}
