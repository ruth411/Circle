DELETE FROM platform.outbox_event_deliveries
USING platform.outbox_events
WHERE platform.outbox_event_deliveries.event_id = platform.outbox_events.id
  AND platform.outbox_event_deliveries.consumer_name = 'diner'
  AND platform.outbox_events.name = 'ordering.closed_order'
  AND platform.outbox_events.published_at = platform.outbox_event_deliveries.delivered_at
  AND platform.outbox_events.occurred_at >= NOW() - INTERVAL '1 day';
