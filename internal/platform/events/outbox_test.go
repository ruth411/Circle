package events

import (
	"context"
	"testing"
	"time"
)

func TestMemoryOutboxAppend(t *testing.T) {
	outbox := &MemoryOutbox{}
	event := Event{
		ID:          "evt-1",
		Name:        "OrderClosed",
		AggregateID: "order-1",
		LocationID:  "loc-1",
		Payload:     []byte(`{"order_id":"order-1"}`),
		OccurredAt:  time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC),
	}

	if err := outbox.Append(context.Background(), event); err != nil {
		t.Fatalf("Append returned error: %v", err)
	}

	events := outbox.Events()
	if len(events) != 1 {
		t.Fatalf("event count = %d, want 1", len(events))
	}
	if events[0].CreatedAt.IsZero() {
		t.Fatal("CreatedAt is zero, want auto-populated timestamp")
	}
	if events[0].LocationID != "loc-1" {
		t.Fatalf("LocationID = %q, want loc-1", events[0].LocationID)
	}
}

func TestMemoryOutboxTracksDeliveryPerConsumer(t *testing.T) {
	outbox := &MemoryOutbox{}
	event := Event{
		ID:          "evt-1",
		Name:        "ordering.closed_order",
		AggregateID: "order-1",
		LocationID:  "loc-1",
		Payload:     []byte(`{"order_id":"order-1"}`),
		OccurredAt:  time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC),
	}

	if err := outbox.Append(context.Background(), event); err != nil {
		t.Fatalf("Append returned error: %v", err)
	}

	inventoryPending, err := outbox.ListUnpublished(context.Background(), "inventory", event.Name, 10)
	if err != nil {
		t.Fatalf("ListUnpublished inventory returned error: %v", err)
	}
	if len(inventoryPending) != 1 {
		t.Fatalf("inventory pending = %d, want 1", len(inventoryPending))
	}

	if err := outbox.MarkPublished(context.Background(), "inventory", event.ID, time.Date(2026, 7, 29, 12, 1, 0, 0, time.UTC)); err != nil {
		t.Fatalf("MarkPublished inventory returned error: %v", err)
	}

	inventoryPending, err = outbox.ListUnpublished(context.Background(), "inventory", event.Name, 10)
	if err != nil {
		t.Fatalf("ListUnpublished inventory returned error: %v", err)
	}
	if len(inventoryPending) != 0 {
		t.Fatalf("inventory pending after publish = %d, want 0", len(inventoryPending))
	}

	dinerPending, err := outbox.ListUnpublished(context.Background(), "diner", event.Name, 10)
	if err != nil {
		t.Fatalf("ListUnpublished diner returned error: %v", err)
	}
	if len(dinerPending) != 1 {
		t.Fatalf("diner pending = %d, want 1", len(dinerPending))
	}
}

func TestMemoryOutboxTracksInvalidDeliveryPerConsumer(t *testing.T) {
	outbox := &MemoryOutbox{}
	event := Event{
		ID:          "evt-1",
		Name:        "ordering.closed_order",
		AggregateID: "order-1",
		LocationID:  "loc-1",
		Payload:     []byte(`{"order_id":"order-1"}`),
		OccurredAt:  time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC),
	}

	if err := outbox.Append(context.Background(), event); err != nil {
		t.Fatalf("Append returned error: %v", err)
	}
	if err := outbox.MarkInvalid(context.Background(), "inventory", event.ID, "invalid_json", "bad payload", time.Date(2026, 7, 29, 12, 1, 0, 0, time.UTC)); err != nil {
		t.Fatalf("MarkInvalid returned error: %v", err)
	}

	inventoryPending, err := outbox.ListUnpublished(context.Background(), "inventory", event.Name, 10)
	if err != nil {
		t.Fatalf("ListUnpublished inventory returned error: %v", err)
	}
	if len(inventoryPending) != 0 {
		t.Fatalf("inventory pending after invalid = %d, want 0", len(inventoryPending))
	}

	dinerPending, err := outbox.ListUnpublished(context.Background(), "diner", event.Name, 10)
	if err != nil {
		t.Fatalf("ListUnpublished diner returned error: %v", err)
	}
	if len(dinerPending) != 1 {
		t.Fatalf("diner pending = %d, want 1", len(dinerPending))
	}

	failures := outbox.Failures()
	if len(failures) != 1 {
		t.Fatalf("failure count = %d, want 1", len(failures))
	}
	if failures[0].FailureCount != 1 {
		t.Fatalf("failure_count = %d, want 1", failures[0].FailureCount)
	}
	if failures[0].FailureKind != "invalid_json" {
		t.Fatalf("failure_kind = %q, want invalid_json", failures[0].FailureKind)
	}
}
