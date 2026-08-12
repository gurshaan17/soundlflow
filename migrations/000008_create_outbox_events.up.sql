-- Outbox — atomic with job creation, relay publishes then marks sent
CREATE TABLE outbox_events (
    id           BIGSERIAL PRIMARY KEY,
    aggregate_id UUID NOT NULL,           -- job_id
    topic        TEXT NOT NULL,
    payload      JSONB NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    sent_at      TIMESTAMPTZ
);
CREATE INDEX idx_outbox_unsent ON outbox_events(sent_at) WHERE sent_at IS NULL;
