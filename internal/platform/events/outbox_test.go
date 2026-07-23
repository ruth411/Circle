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
