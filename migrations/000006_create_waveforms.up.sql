-- Waveform data (small JSON array of amplitude peaks)
CREATE TABLE waveforms (
    episode_id   UUID PRIMARY KEY REFERENCES episodes(id),
    job_id       UUID NOT NULL REFERENCES jobs(id),
    peaks        JSONB NOT NULL,
    resolution   INT NOT NULL,            -- samples per second of audio
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
