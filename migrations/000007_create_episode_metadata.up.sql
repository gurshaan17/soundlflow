-- Extracted metadata / audio analysis
CREATE TABLE episode_metadata (
    episode_id     UUID PRIMARY KEY REFERENCES episodes(id),
    duration_secs  NUMERIC,
    sample_rate    INT,
    channels       INT,
    original_codec TEXT,
    loudness_lufs  NUMERIC,               -- pre-normalization measurement
    processing_ms  BIGINT,
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
