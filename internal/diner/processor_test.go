package diner

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/ruth411/circle/internal/contracts"
	"github.com/ruth411/circle/internal/core/ingredient"
	"github.com/ruth411/circle/internal/platform/events"
)

func TestProcessorConsumesClosedOrderEvenAfterInventoryConsumer(t *testing.T) {
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	order := contracts.ClosedOrder{
		OrderID:    "order-1",
		LocationID: "loc-1",
		ClosedAt:   now,
		Lines: []contracts.ClosedOrderLine{
			{
				LineID:          "line-1",
				Name:            "Bowl",
				Quantity:        2,
				ResolvedMacros:  ingredient.MacroValues{Calories: 600, ProteinGrams: 40},
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

	if err := outbox.MarkPublished(context.Background(), "inventory", "evt-1", now); err != nil {
		t.Fatalf("MarkPublished inventory returned error: %v", err)
	}

	service := NewService()
	service.now = func() time.Time { return now }
	processor := NewProcessor(outbox, service)

	processed, err := processor.ProcessPendingClosedOrders(context.Background(), 10)
	if err != nil {
		t.Fatalf("ProcessPendingClosedOrders returned error: %v", err)
	}
	if processed != 1 {
		t.Fatalf("processed = %d, want 1", processed)
	}

	tokens, err := service.ResolveTokenByOrder(context.Background(), order.LocationID, order.OrderID)
	if err != nil {
		t.Fatalf("ResolveTokenByOrder returned error: %v", err)
	}
	if len(tokens.Items) != 2 {
		t.Fatalf("token item count = %d, want 2", len(tokens.Items))
	}
}

func TestProcessorSkipsMalformedClosedOrderAndContinues(t *testing.T) {
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	malformedPayload, err := json.Marshal(contracts.ClosedOrder{
		OrderID:    "order-bad",
		LocationID: "loc-1",
		ClosedAt:   now,
		Lines: []contracts.ClosedOrderLine{
			{LineID: "line-bad", Name: "Broken", Quantity: 0},
		},
	})
	if err != nil {
		t.Fatalf("Marshal malformed returned error: %v", err)
	}
	validPayload, err := json.Marshal(contracts.ClosedOrder{
		OrderID:    "order-good",
		LocationID: "loc-1",
		ClosedAt:   now,
		Lines: []contracts.ClosedOrderLine{
			{LineID: "line-1", Name: "Bowl", Quantity: 1, ResolvedMacros: ingredient.MacroValues{Calories: 600}},
		},
	})
	if err != nil {
		t.Fatalf("Marshal valid returned error: %v", err)
	}

	outbox := &events.MemoryOutbox{}
	for _, event := range []events.Event{
		{
			ID:          "evt-bad",
			Name:        contracts.ClosedOrderEventName,
			AggregateID: "order-bad",
			LocationID:  "loc-1",
			Payload:     malformedPayload,
			OccurredAt:  now,
		},
		{
			ID:          "evt-good",
			Name:        contracts.ClosedOrderEventName,
			AggregateID: "order-good",
			LocationID:  "loc-1",
			Payload:     validPayload,
			OccurredAt:  now.Add(time.Second),
		},
	} {
		if err := outbox.Append(context.Background(), event); err != nil {
			t.Fatalf("Append returned error: %v", err)
		}
	}

	service := NewService()
	service.now = func() time.Time { return now }
	processor := NewProcessor(outbox, service)

	processed, err := processor.ProcessPendingClosedOrders(context.Background(), 10)
	if err != nil {
		t.Fatalf("ProcessPendingClosedOrders returned error: %v", err)
	}
	if processed != 1 {
		t.Fatalf("processed = %d, want 1", processed)
	}

	if _, err := service.ResolveTokenByOrder(context.Background(), "loc-1", "order-good"); err != nil {
		t.Fatalf("ResolveTokenByOrder returned error: %v", err)
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
	if failures[0].FailureKind != "invalid_closed_order" {
		t.Fatalf("failure_kind = %q, want invalid_closed_order", failures[0].FailureKind)
	}
}
