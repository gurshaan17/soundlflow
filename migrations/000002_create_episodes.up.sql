CREATE TABLE episodes (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    show_id         UUID NOT NULL REFERENCES shows(id),
    episode_number  INT NOT NULL,
    title           TEXT,
    raw_object_key  TEXT NOT NULL,        -- S3 key of uploaded raw file
    raw_checksum    TEXT,                  -- for corruption/dedup checks
    status          TEXT NOT NULL DEFAULT 'UPLOADED',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (show_id, episode_number)
);
