CREATE TABLE IF NOT EXISTS platform.outbox_event_failures (
    event_id TEXT NOT NULL REFERENCES platform.outbox_events(id) ON DELETE CASCADE,
    consumer_name TEXT NOT NULL,
    failure_kind TEXT NOT NULL,
    error_message TEXT NOT NULL,
    first_failed_at TIMESTAMPTZ NOT NULL,
    last_failed_at TIMESTAMPTZ NOT NULL,
    failure_count INTEGER NOT NULL,
    PRIMARY KEY (event_id, consumer_name)
);

CREATE INDEX IF NOT EXISTS outbox_event_failures_consumer_idx
    ON platform.outbox_event_failures (consumer_name, last_failed_at, event_id);
