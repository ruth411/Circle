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
	ListUnpublished(context.Context, string, string, int) ([]Event, error)
	MarkPublished(context.Context, string, string, time.Time) error
	MarkInvalid(context.Context, string, string, string, string, time.Time) error
}

type Failure struct {
	EventID       string
	ConsumerName  string
	FailureKind   string
	ErrorMessage  string
	FirstFailedAt time.Time
	LastFailedAt  time.Time
	FailureCount  int
}

type MemoryOutbox struct {
	mu         sync.Mutex
	events     []Event
	deliveries map[string]map[string]time.Time
	failures   map[string]map[string]Failure
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

func (o *MemoryOutbox) ListUnpublished(_ context.Context, consumer string, name string, limit int) ([]Event, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	var out []Event
	for _, event := range o.events {
		if event.Name != name || o.deliveredLocked(consumer, event.ID) {
			continue
		}
		out = append(out, event)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (o *MemoryOutbox) MarkPublished(_ context.Context, consumer string, id string, publishedAt time.Time) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	return o.markPublishedLocked(consumer, id, publishedAt)
}

func (o *MemoryOutbox) MarkInvalid(_ context.Context, consumer string, id string, failureKind string, errorMessage string, failedAt time.Time) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	for i := range o.events {
		if o.events[i].ID != id {
			continue
		}
		if o.failures == nil {
			o.failures = map[string]map[string]Failure{}
		}
		if o.failures[consumer] == nil {
			o.failures[consumer] = map[string]Failure{}
		}

		failedAt = failedAt.UTC()
		failure, ok := o.failures[consumer][id]
		if !ok {
			failure = Failure{
				EventID:       id,
				ConsumerName:  consumer,
				FailureKind:   failureKind,
				ErrorMessage:  errorMessage,
				FirstFailedAt: failedAt,
			}
		}
		failure.FailureKind = failureKind
		failure.ErrorMessage = errorMessage
		failure.LastFailedAt = failedAt
		failure.FailureCount++
		o.failures[consumer][id] = failure

		return o.markPublishedLocked(consumer, id, failedAt)
	}
	return sql.ErrNoRows
}

func (o *MemoryOutbox) markPublishedLocked(consumer string, id string, publishedAt time.Time) error {
	for i := range o.events {
		if o.events[i].ID != id {
			continue
		}
		if o.deliveries == nil {
			o.deliveries = map[string]map[string]time.Time{}
		}
		if o.deliveries[consumer] == nil {
			o.deliveries[consumer] = map[string]time.Time{}
		}
		publishedAt = publishedAt.UTC()
		o.deliveries[consumer][id] = publishedAt
		if o.events[i].PublishedAt == nil {
			o.events[i].PublishedAt = &publishedAt
		}
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

func (o *MemoryOutbox) Failures() []Failure {
	o.mu.Lock()
	defer o.mu.Unlock()

	var out []Failure
	for _, consumerFailures := range o.failures {
		for _, failure := range consumerFailures {
			out = append(out, failure)
		}
	}
	return out
}

func (o *MemoryOutbox) deliveredLocked(consumer string, eventID string) bool {
	if o.deliveries == nil {
		return false
	}
	consumerDeliveries := o.deliveries[consumer]
	if consumerDeliveries == nil {
		return false
	}
	_, ok := consumerDeliveries[eventID]
	return ok
}
