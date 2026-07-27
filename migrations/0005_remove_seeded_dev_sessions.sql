DELETE FROM identity.sessions
WHERE id IN (
    'session-chipotle-hq-dev',
    'session-chipotle-charlotte-dev',
    'session-chipotle-raleigh-dev'
);
