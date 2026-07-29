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

func (o *SQLOutbox) ListUnpublished(ctx context.Context, name string, limit int) ([]Event, error) {
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
WHERE name = $1
  AND published_at IS NULL
ORDER BY occurred_at, created_at, id
LIMIT $2;
`, name, limit)
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

func (o *SQLOutbox) MarkPublished(ctx context.Context, id string, publishedAt time.Time) error {
	result, err := o.db.ExecContext(ctx, `
UPDATE platform.outbox_events
SET published_at = $2
WHERE id = $1
  AND published_at IS NULL;
`, id, publishedAt.UTC())
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
