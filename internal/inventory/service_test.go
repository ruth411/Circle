package inventory

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/ruth411/circle/internal/contracts"
	"github.com/ruth411/circle/internal/core/ingredient"
	"github.com/ruth411/circle/internal/platform/events"
)

func TestRecordDepletionIsAppendOnlyAndIdempotent(t *testing.T) {
	now := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)
	service := NewService(newMemoryRepository(map[string]ingredient.Unit{
		"chicken": ingredient.UnitGram,
		"rice":    ingredient.UnitGram,
	}))

	order := contracts.ClosedOrder{
		OrderID:    "order-1",
		LocationID: "loc-1",
		ClosedAt:   now,
		Lines: []contracts.ClosedOrderLine{
			{
				LineID:          "line-1",
				IngredientUsage: map[string]float64{"chicken": 150, "rice": 100},
				IngredientUnits: map[string]ingredient.Unit{"chicken": ingredient.UnitGram, "rice": ingredient.UnitGram},
			},
		},
	}

	movements, err := service.RecordDepletion(context.Background(), order)
	if err != nil {
		t.Fatalf("RecordDepletion returned error: %v", err)
	}
	if len(movements) != 2 {
		t.Fatalf("movement count = %d, want 2", len(movements))
	}
	if movements[0].Quantity >= 0 {
		t.Fatalf("movement quantity = %v, want negative", movements[0].Quantity)
	}
	if movements[0].Unit != ingredient.UnitGram {
		t.Fatalf("movement unit = %s, want g", movements[0].Unit)
	}

	movements, err = service.RecordDepletion(context.Background(), order)
	if err != nil {
		t.Fatalf("second RecordDepletion returned error: %v", err)
	}
	if len(movements) != 0 {
		t.Fatalf("second call produced %d movements, want 0", len(movements))
	}
}

func TestProcessPendingClosedOrdersRecordsMovements(t *testing.T) {
	now := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)
	order := contracts.ClosedOrder{
		OrderID:    "order-1",
		LocationID: "loc-1",
		ClosedAt:   now,
		Lines: []contracts.ClosedOrderLine{
			{
				LineID:          "line-1",
				IngredientUsage: map[string]float64{"chicken": 150},
				IngredientUnits: map[string]ingredient.Unit{"chicken": ingredient.UnitGram},
			},
		},
	}
	payload, err := json.Marshal(order)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}

	outbox := &events.MemoryOutbox{}
	if err := outbox.Append(context.Background(), events.Event{
		ID:          "evt-1",
		Name:        contracts.ClosedOrderEventName,
		AggregateID: order.OrderID,
		LocationID:  order.LocationID,
		Payload:     payload,
		OccurredAt:  now,
	}); err != nil {
		t.Fatalf("Append returned error: %v", err)
	}

	service := NewService(newMemoryRepository(map[string]ingredient.Unit{
		"chicken": ingredient.UnitGram,
	}))
	processor := NewProcessor(outbox, service)

	processed, err := processor.ProcessPendingClosedOrders(context.Background(), 10)
	if err != nil {
		t.Fatalf("ProcessPendingClosedOrders returned error: %v", err)
	}
	if processed != 1 {
		t.Fatalf("processed = %d, want 1", processed)
	}

	movements, err := service.Movements(context.Background(), "loc-1")
	if err != nil {
		t.Fatalf("Movements returned error: %v", err)
	}
	if len(movements) != 1 {
		t.Fatalf("movement count = %d, want 1", len(movements))
	}

	recorded := outbox.Events()
	if len(recorded) != 1 || recorded[0].PublishedAt == nil {
		t.Fatalf("published_at = %v, want populated", recorded[0].PublishedAt)
	}

	dinerPending, err := outbox.ListUnpublished(context.Background(), "diner", contracts.ClosedOrderEventName, 10)
	if err != nil {
		t.Fatalf("ListUnpublished diner returned error: %v", err)
	}
	if len(dinerPending) != 1 {
		t.Fatalf("diner pending = %d, want 1", len(dinerPending))
	}
}

func TestRecordDepletionUsesClosedOrderUnitSnapshot(t *testing.T) {
	now := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)
	service := NewService(newMemoryRepository(map[string]ingredient.Unit{
		"chicken": ingredient.UnitEach,
	}))

	movements, err := service.RecordDepletion(context.Background(), contracts.ClosedOrder{
		OrderID:    "order-1",
		LocationID: "loc-1",
		ClosedAt:   now,
		Lines: []contracts.ClosedOrderLine{
			{
				LineID:          "line-1",
				IngredientUsage: map[string]float64{"chicken": 150},
				IngredientUnits: map[string]ingredient.Unit{"chicken": ingredient.UnitGram},
			},
		},
	})
	if err != nil {
		t.Fatalf("RecordDepletion returned error: %v", err)
	}
	if len(movements) != 1 {
		t.Fatalf("movement count = %d, want 1", len(movements))
	}
	if movements[0].Unit != ingredient.UnitGram {
		t.Fatalf("movement unit = %s, want g", movements[0].Unit)
	}
}

func TestRecordReceiptIsAppendOnlyAndIdempotent(t *testing.T) {
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	service := NewService(newMemoryRepository(nil))

	receipt := contracts.PurchaseReceipt{
		ReceiptID:  "rec-1",
		LocationID: "loc-1",
		OccurredAt: now,
		SourceType: contracts.PurchaseReceiptSourceType,
		SourceID:   "rec-1",
		Lines: []contracts.PurchaseReceiptLine{
			{
				IngredientID:      "chicken",
				QuantityBaseUnits: 1500,
				Unit:              ingredient.UnitGram,
			},
		},
	}

	movements, err := service.RecordReceipt(context.Background(), receipt)
	if err != nil {
		t.Fatalf("RecordReceipt returned error: %v", err)
	}
	if len(movements) != 1 {
		t.Fatalf("movement count = %d, want 1", len(movements))
	}
	if movements[0].Quantity != 1500 {
		t.Fatalf("movement quantity = %v, want 1500", movements[0].Quantity)
	}
	if movements[0].SourceType != contracts.PurchaseReceiptSourceType {
		t.Fatalf("movement source type = %q, want %q", movements[0].SourceType, contracts.PurchaseReceiptSourceType)
	}
	if movements[0].SourceID != "rec-1" {
		t.Fatalf("movement source id = %q, want rec-1", movements[0].SourceID)
	}

	movements, err = service.RecordReceipt(context.Background(), receipt)
	if err != nil {
		t.Fatalf("second RecordReceipt returned error: %v", err)
	}
	if len(movements) != 0 {
		t.Fatalf("second call produced %d movements, want 0", len(movements))
	}
}

func TestRecordReceiptAggregatesSameIngredientLines(t *testing.T) {
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	service := NewService(newMemoryRepository(nil))

	receipt := contracts.PurchaseReceipt{
		ReceiptID:  "rec-1",
		LocationID: "loc-1",
		OccurredAt: now,
		SourceType: contracts.PurchaseReceiptSourceType,
		SourceID:   "rec-1",
		Lines: []contracts.PurchaseReceiptLine{
			{
				IngredientID:      "chicken",
				QuantityBaseUnits: 1500,
				Unit:              ingredient.UnitGram,
			},
			{
				IngredientID:      "chicken",
				QuantityBaseUnits: 500,
				Unit:              ingredient.UnitGram,
			},
		},
	}

	movements, err := service.RecordReceipt(context.Background(), receipt)
	if err != nil {
		t.Fatalf("RecordReceipt returned error: %v", err)
	}
	if len(movements) != 1 {
		t.Fatalf("movement count = %d, want 1", len(movements))
	}
	if movements[0].Quantity != 2000 {
		t.Fatalf("movement quantity = %v, want 2000", movements[0].Quantity)
	}
}

func TestMovementIDsAreNamespacedBySourceType(t *testing.T) {
	now := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	service := NewService(newMemoryRepository(nil))

	depletion, err := service.RecordDepletion(context.Background(), contracts.ClosedOrder{
		OrderID:    "shared-1",
		LocationID: "loc-1",
		ClosedAt:   now,
		Lines: []contracts.ClosedOrderLine{
			{
				LineID:          "line-1",
				IngredientUsage: map[string]float64{"chicken": 100},
				IngredientUnits: map[string]ingredient.Unit{"chicken": ingredient.UnitGram},
			},
		},
	})
	if err != nil {
		t.Fatalf("RecordDepletion returned error: %v", err)
	}

	receipt, err := service.RecordReceipt(context.Background(), contracts.PurchaseReceipt{
		ReceiptID:  "shared-1",
		LocationID: "loc-1",
		OccurredAt: now,
		SourceType: contracts.PurchaseReceiptSourceType,
		SourceID:   "shared-1",
		Lines: []contracts.PurchaseReceiptLine{
			{
				IngredientID:      "chicken",
				QuantityBaseUnits: 100,
				Unit:              ingredient.UnitGram,
			},
		},
	})
	if err != nil {
		t.Fatalf("RecordReceipt returned error: %v", err)
	}

	if depletion[0].ID == receipt[0].ID {
		t.Fatalf("movement ids = %q and %q, want distinct ids across source types", depletion[0].ID, receipt[0].ID)
	}
}

func TestProcessPendingClosedOrdersSkipsMalformedEventAndContinues(t *testing.T) {
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	validOrder := contracts.ClosedOrder{
		OrderID:    "order-good",
		LocationID: "loc-1",
		ClosedAt:   now,
		Lines: []contracts.ClosedOrderLine{
			{
				LineID:          "line-1",
				IngredientUsage: map[string]float64{"chicken": 150},
				IngredientUnits: map[string]ingredient.Unit{"chicken": ingredient.UnitGram},
			},
		},
	}
	validPayload, err := json.Marshal(validOrder)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}

	outbox := &events.MemoryOutbox{}
	for _, event := range []events.Event{
		{
			ID:          "evt-bad",
			Name:        contracts.ClosedOrderEventName,
			AggregateID: "order-bad",
			LocationID:  "loc-1",
			Payload:     []byte(`{"OrderID":`),
			OccurredAt:  now,
		},
		{
			ID:          "evt-good",
			Name:        contracts.ClosedOrderEventName,
			AggregateID: validOrder.OrderID,
			LocationID:  validOrder.LocationID,
			Payload:     validPayload,
			OccurredAt:  now.Add(time.Second),
		},
	} {
		if err := outbox.Append(context.Background(), event); err != nil {
			t.Fatalf("Append returned error: %v", err)
		}
	}

	service := NewService(newMemoryRepository(map[string]ingredient.Unit{
		"chicken": ingredient.UnitGram,
	}))
	processor := NewProcessor(outbox, service)

	processed, err := processor.ProcessPendingClosedOrders(context.Background(), 10)
	if err != nil {
		t.Fatalf("ProcessPendingClosedOrders returned error: %v", err)
	}
	if processed != 1 {
		t.Fatalf("processed = %d, want 1", processed)
	}

	movements, err := service.Movements(context.Background(), "loc-1")
	if err != nil {
		t.Fatalf("Movements returned error: %v", err)
	}
	if len(movements) != 1 {
		t.Fatalf("movement count = %d, want 1", len(movements))
	}

	pending, err := outbox.ListUnpublished(context.Background(), outboxConsumer, contracts.ClosedOrderEventName, 10)
	if err != nil {
		t.Fatalf("ListUnpublished returned error: %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("pending = %d, want 0", len(pending))
	}

	failures := outbox.Failures()
	if len(failures) != 1 {
		t.Fatalf("failure count = %d, want 1", len(failures))
	}
	if failures[0].ConsumerName != outboxConsumer {
		t.Fatalf("consumer = %q, want %q", failures[0].ConsumerName, outboxConsumer)
	}
	if failures[0].FailureKind != "invalid_json" {
		t.Fatalf("failure_kind = %q, want invalid_json", failures[0].FailureKind)
	}
}
