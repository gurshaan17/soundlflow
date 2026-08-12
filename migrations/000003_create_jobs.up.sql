-- Jobs (one per episode processing run; supports reprocessing = new row)
CREATE TABLE jobs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    episode_id      UUID NOT NULL REFERENCES episodes(id),
    status          TEXT NOT NULL DEFAULT 'QUEUED',
        -- QUEUED | PROCESSING | FAILED | RETRYING | COMPLETED | DLQ
    current_step    TEXT,
        -- VALIDATE | TRANSCODE | ANALYZE | WAVEFORM | NORMALIZE | UPLOAD
    attempt         INT NOT NULL DEFAULT 0,
    max_attempts    INT NOT NULL DEFAULT 3,
    idempotency_key TEXT UNIQUE,
    error_message   TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_jobs_status ON jobs(status);
CREATE INDEX idx_jobs_episode ON jobs(episode_id);
