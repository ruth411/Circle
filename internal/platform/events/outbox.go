package events

import (
	"context"
	"sync"
	"time"
)

type Event struct {
	ID          string
	AggregateID string
	Name        string
	Payload     string
	OccurredAt  time.Time
}

type Outbox interface {
	Append(context.Context, Event) error
}

type MemoryOutbox struct {
	mu     sync.Mutex
	events []Event
}

func (o *MemoryOutbox) Append(_ context.Context, event Event) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.events = append(o.events, event)
	return nil
}

func (o *MemoryOutbox) Events() []Event {
	o.mu.Lock()
	defer o.mu.Unlock()

	out := make([]Event, len(o.events))
	copy(out, o.events)
	return out
}
