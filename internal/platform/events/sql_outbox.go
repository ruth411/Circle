package events

import (
	"context"
	"database/sql"
	"time"
)

type SQLOutbox struct {
	db *sql.DB
}

type sqlExecContext interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func NewSQLOutbox(db *sql.DB) *SQLOutbox {
	return &SQLOutbox{db: db}
}

func (o *SQLOutbox) Append(ctx context.Context, event Event) error {
	return AppendSQL(ctx, o.db, event)
}

func AppendSQL(ctx context.Context, db sqlExecContext, event Event) error {
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}
	_, err := db.ExecContext(ctx, `
INSERT INTO platform.outbox_events (
    id,
    name,
    aggregate_id,
    location_id,
    payload,
    occurred_at,
    created_at,
    published_at
)
VALUES ($1, $2, $3, $4, $5::jsonb, $6, $7, $8);
`, event.ID, event.Name, event.AggregateID, event.LocationID, event.Payload, event.OccurredAt.UTC(), event.CreatedAt.UTC(), event.PublishedAt)
	return err
}

func (o *SQLOutbox) ListUnpublished(ctx context.Context, consumer string, name string, limit int) ([]Event, error) {
	if limit <= 0 {
		limit = 100
	}

	rows, err := o.db.QueryContext(ctx, `
SELECT
    id,
    name,
    aggregate_id,
    location_id,
    payload,
    occurred_at,
    created_at,
    published_at
FROM platform.outbox_events
LEFT JOIN platform.outbox_event_deliveries
  ON platform.outbox_event_deliveries.event_id = platform.outbox_events.id
 AND platform.outbox_event_deliveries.consumer_name = $1
WHERE name = $2
  AND platform.outbox_event_deliveries.event_id IS NULL
ORDER BY occurred_at, created_at, id
LIMIT $3;
`, consumer, name, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		var event Event
		var payload []byte
		var publishedAt sql.NullTime
		if err := rows.Scan(&event.ID, &event.Name, &event.AggregateID, &event.LocationID, &payload, &event.OccurredAt, &event.CreatedAt, &publishedAt); err != nil {
			return nil, err
		}
		event.Payload = append([]byte(nil), payload...)
		if publishedAt.Valid {
			value := publishedAt.Time.UTC()
			event.PublishedAt = &value
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

func (o *SQLOutbox) MarkPublished(ctx context.Context, consumer string, id string, publishedAt time.Time) error {
	publishedAt = publishedAt.UTC()
	result, err := o.db.ExecContext(ctx, `
WITH inserted AS (
    INSERT INTO platform.outbox_event_deliveries (
        event_id,
        consumer_name,
        delivered_at
    )
    VALUES ($1, $2, $3)
    ON CONFLICT (event_id, consumer_name) DO NOTHING
    RETURNING event_id
)
UPDATE platform.outbox_events
SET published_at = COALESCE(published_at, $3)
WHERE id = $1
  AND EXISTS (SELECT 1 FROM platform.outbox_events WHERE id = $1);
`, id, consumer, publishedAt)
	if err != nil {
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (o *SQLOutbox) MarkInvalid(ctx context.Context, consumer string, id string, failureKind string, errorMessage string, failedAt time.Time) error {
	failedAt = failedAt.UTC()
	result, err := o.db.ExecContext(ctx, `
WITH failed AS (
    INSERT INTO platform.outbox_event_failures (
        event_id,
        consumer_name,
        failure_kind,
        error_message,
        first_failed_at,
        last_failed_at,
        failure_count
    )
    VALUES ($1, $2, $3, $4, $5, $5, 1)
    ON CONFLICT (event_id, consumer_name) DO UPDATE
    SET failure_kind = EXCLUDED.failure_kind,
        error_message = EXCLUDED.error_message,
        last_failed_at = EXCLUDED.last_failed_at,
        failure_count = platform.outbox_event_failures.failure_count + 1
), delivered AS (
    INSERT INTO platform.outbox_event_deliveries (
        event_id,
        consumer_name,
        delivered_at
    )
    VALUES ($1, $2, $5)
    ON CONFLICT (event_id, consumer_name) DO NOTHING
)
UPDATE platform.outbox_events
SET published_at = COALESCE(published_at, $5)
WHERE id = $1
  AND EXISTS (SELECT 1 FROM platform.outbox_events WHERE id = $1);
`, id, consumer, failureKind, errorMessage, failedAt)
	if err != nil {
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return sql.ErrNoRows
	}
	return nil
}
