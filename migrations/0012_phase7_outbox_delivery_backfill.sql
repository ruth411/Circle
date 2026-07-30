INSERT INTO platform.outbox_event_deliveries (
    event_id,
    consumer_name,
    delivered_at
)
SELECT id, 'inventory', published_at
FROM platform.outbox_events
WHERE published_at IS NOT NULL
  AND name = 'ordering.closed_order'
ON CONFLICT (event_id, consumer_name) DO NOTHING;

INSERT INTO platform.outbox_event_deliveries (
    event_id,
    consumer_name,
    delivered_at
)
SELECT id, 'diner', published_at
FROM platform.outbox_events
WHERE published_at IS NOT NULL
  AND name = 'ordering.closed_order'
ON CONFLICT (event_id, consumer_name) DO NOTHING;
