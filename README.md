# SoundFlow

A distributed audio processing pipeline in Go, modeled after a production
podcast/audio ingestion system (Pocket FM-style episode ingestion). Episode
uploads are accepted over HTTP, tracked as resumable multi-step jobs
(VALIDATE → TRANSCODE → ANALYZE → WAVEFORM → UPLOAD), and fanned out to
workers through an outbox-backed message queue.

**Current status — Step 1 (skeleton).** A working HTTP API persists episodes,
jobs, and outbox events atomically. No Kafka, workers, or FFmpeg yet; those
arrive in later phases and are deliberately absent so the skeleton stays honest
about what is actually implemented.

---

## Architecture
![SoundFlow architecture](architecture.png)

```
                     POST /v1/episodes (Idempotency-Key)
                                   │
                        ┌──────────▼──────────┐
                        │     HTTP API        │
                        │  stdlib net/http    │
                        └──────────┬──────────┘
                                   │  one Postgres transaction
                ┌──────────────────┼──────────────────┐
                ▼                  ▼                  ▼
            episodes            jobs              outbox_events
                               (QUEUED)     topic=audio.jobs.validate
                                              payload={episode_id, job_id}
```

A relay (next step) will poll `outbox_events` where `sent_at IS NULL` and
publish to Kafka, then mark the row sent. The outbox write is already atomic
with job creation, so no events can be lost between step 1 and step 2.

### Repo layout

```
cmd/api/main.go                  entrypoint; -migrate flag, graceful shutdown
internal/api/                    HTTP layer
  handlers/                      request/response DTOs + handlers
  middleware/                    request-id, logging, panic recovery
  router.go                      route wiring
internal/config/                 env + .env loading with local defaults
internal/domain/                 plain structs, typed statuses, sentinel errors
internal/storage/postgres/       pgx repositories + migration runner
migrations/                      golang-migrate .up.sql/.down.sql files
docker-compose.yml               optional local Postgres (for dev/CI only)
```

Dependency rules: `handlers` and `repos` depend on `domain`; `domain` imports
nothing but the stdlib. No ORM — SQL is explicit in the repository layer.

### Schema

- `shows` — podcasts/series
- `episodes` — unique per `(show_id, episode_number)`
- `jobs` — one per processing run; `status`, `current_step`, `attempt`,
  `max_attempts`, `idempotency_key`; reprocessing creates a new row
- `job_steps` — per-step state (`PENDING|RUNNING|SUCCESS|FAILED|SKIPPED`),
  what makes retries resumable
- `transcoded_files`, `waveforms`, `episode_metadata` — produced later by workers
- `outbox_events` — `aggregate_id` (the job id), `topic`, `payload jsonb`,
  `sent_at`; partial index on unsent rows for the relay

---

## Running locally

### Prerequisites

- Go 1.25+
- A Postgres 13+ database. SoundFlow is tested against Neon; `docker-compose.yml`
  provides a local Postgres 16 if you prefer that.

### 1. Configure the database

Create a `.env` file in the repo root (git-ignored):

```
DATABASE_URL=postgresql://USER:PASS@HOST:PORT/DB?sslmode=require
```

For Neon, use your project connection string. Real environment variables take
precedence over `.env`. If neither is set, the app falls back to the
docker-compose defaults (`postgres://soundflow:soundflow@localhost:5432/soundflow`).

### 2. Run migrations

```
make migrate
```

This runs the embedded golang-migrate migrations against your `DATABASE_URL`
and exits. Under the hood it connects to the *direct* Postgres endpoint rather
than a pooler: golang-migrate takes session-level `pg_advisory_lock`s, which
hang on transaction-mode poolers such as Neon's `-pooler` endpoint. The
migration runner strips `-pooler` from the hostname automatically.

> A fresh Neon project needs `gen_random_uuid()`; that's built into Postgres 13+,
> no extension required.

### 3. Start the API

```
make run
```

Listens on `:8080` by default (`PORT` to override), reads `DATABASE_URL` from
`.env`, and shuts down gracefully on SIGINT/SIGTERM.

```
curl -s localhost:8080/healthz   # 200 ok
curl -s localhost:8080/readyz    # 200 ok (pings the database)
```

### API

| Method | Path                                | Description                                  |
|--------|-------------------------------------|----------------------------------------------|
| POST   | `/v1/episodes`                      | Ingest an episode (see below)                |
| GET    | `/v1/episodes/{id}`                 | Episode + most recent job                    |
| GET    | `/v1/episodes/{id}/jobs/{job_id}`   | Job + all `job_steps` rows                   |
| GET    | `/healthz`                          | Liveness                                    |
| GET    | `/readyz`                           | Readiness (DB ping)                         |

`POST /v1/episodes` requires an `Idempotency-Key` header and a body:

```json
{ "show_id": "…", "episode_number": 1, "title": "Ep 1", "raw_object_key": "raw/ep1.mp3" }
```

In a single transaction it inserts the episode, a `QUEUED` job, and an outbox
event (`topic=audio.jobs.validate`, `payload={"episode_id","job_id"}`) and
returns `202`:

```json
{ "episode_id": "…", "job_id": "…", "status": "QUEUED" }
```

Replaying the same `Idempotency-Key` (sequentially or concurrently) returns the
existing job with `200` and never creates a duplicate episode, job, or outbox
row. `show_id` must reference an existing show (there is no show endpoint yet;
insert one directly to get started). A different key for the same
`(show_id, episode_number)` returns `409`.

### Tests

```
make test
```

The integration tests exercise the real HTTP router against the database in
`DATABASE_URL`: they create a show, post episodes, and assert on rows in
Postgres. They cover (a) atomic episode+job+outbox creation and (b) idempotent
replay, including concurrent same-key requests.

> **Warning:** the tests `TRUNCATE` the SoundFlow tables (`episodes`, `jobs`,
> `job_steps`, `outbox_events`, `shows`, `transcoded_files`, `waveforms`,
> `episode_metadata`) at the start of each test. Point them at a disposable
> database. To use a different target, set `TEST_DATABASE_URL`; tables not owned
> by SoundFlow (e.g. `links`) are never touched. Tests skip if the database is
> unreachable.

---

## Decisions & rationale

- **Router: stdlib `net/http` + `ServeMux`.** Go 1.22+ method+wildcard routing
  covers every route here; zero extra dependencies. Middleware is ~80 lines.
- **Driver: `pgx/v5` + `pgxpool`.** Explicit SQL, native transaction handling,
  and the `Querier` interface lets repository methods run against a pool or a
  `pgx.Tx` interchangeably.
- **Migrations: golang-migrate**, embedded in the binary via `embed.FS` — no CLI
  to install. Paired `.up.sql`/`.down.sql` files, run with `make migrate`.
- **IDs are strings in `domain`.** UUID columns scan into/out of Go strings via
  pgx, keeping `domain` free of any database dependency.

## Roadmap

1. ✅ HTTP API skeleton with atomic episode/job/outbox ingestion
2. ⬜ **Outbox relay** — poll `outbox_events` where `sent_at IS NULL` and publish
   to Kafka, then mark sent
3. ⬜ Worker pool + per-step state machine (VALIDATE → TRANSCODE → …) with
   resumable retries via `job_steps`
4. ⬜ FFmpeg integration, S3/MinIO object storage
5. ⬜ Audio analysis, waveform generation, metadata extraction
