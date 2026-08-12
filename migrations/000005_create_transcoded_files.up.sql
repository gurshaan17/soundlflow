-- Transcoded output variants
CREATE TABLE transcoded_files (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    episode_id   UUID NOT NULL REFERENCES episodes(id),
    job_id       UUID NOT NULL REFERENCES jobs(id),
    bitrate_kbps INT NOT NULL,            -- 64 / 128 / 256
    codec        TEXT NOT NULL,           -- aac / opus / mp3
    object_key   TEXT NOT NULL,           -- S3 key of output file
    file_size    BIGINT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
