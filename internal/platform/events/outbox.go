package events

import (
	"context"
	"database/sql"
	"sync"
	"time"
)

type Event struct {
	ID          string
	Name        string
	AggregateID string
	LocationID  string
	Payload     []byte
	OccurredAt  time.Time
	CreatedAt   time.Time
	PublishedAt *time.Time
}

type Appender interface {
	Append(context.Context, Event) error
}

type Reader interface {
	ListUnpublished(context.Context, string, int) ([]Event, error)
	MarkPublished(context.Context, string, time.Time) error
}

type MemoryOutbox struct {
	mu     sync.Mutex
	events []Event
}

func (o *MemoryOutbox) Append(_ context.Context, event Event) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}
	o.events = append(o.events, event)
	return nil
}

func (o *MemoryOutbox) ListUnpublished(_ context.Context, name string, limit int) ([]Event, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	var out []Event
	for _, event := range o.events {
		if event.Name != name || event.PublishedAt != nil {
			continue
		}
		out = append(out, event)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (o *MemoryOutbox) MarkPublished(_ context.Context, id string, publishedAt time.Time) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	for i := range o.events {
		if o.events[i].ID != id {
			continue
		}
		publishedAt = publishedAt.UTC()
		o.events[i].PublishedAt = &publishedAt
		return nil
	}
	return sql.ErrNoRows
}

func (o *MemoryOutbox) Events() []Event {
	o.mu.Lock()
	defer o.mu.Unlock()

	out := make([]Event, len(o.events))
	copy(out, o.events)
	return out
}
