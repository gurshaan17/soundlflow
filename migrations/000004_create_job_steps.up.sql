-- Per-step state (this is what makes retries resumable)
CREATE TABLE job_steps (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id        UUID NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    step_name     TEXT NOT NULL,          -- VALIDATE, TRANSCODE, etc.
    status        TEXT NOT NULL DEFAULT 'PENDING',
        -- PENDING | RUNNING | SUCCESS | FAILED | SKIPPED
    attempt       INT NOT NULL DEFAULT 0,
    started_at    TIMESTAMPTZ,
    finished_at   TIMESTAMPTZ,
    error_message TEXT,
    UNIQUE (job_id, step_name)
);
