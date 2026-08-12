package handlers_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gurshaan17/soundlflow/internal/api"
	"github.com/gurshaan17/soundlflow/internal/storage/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

const testDatabaseURLEnv = "TEST_DATABASE_URL"

// tablesTruncated lists every SoundFlow table the tests wipe before running.
// Pre-existing tables that are not part of SoundFlow (e.g. `links`) are
// intentionally excluded and never touched.
const tablesTruncated = `outbox_events, job_steps, jobs, episodes, shows, transcoded_files, waveforms, episode_metadata`

func newTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	dsn := os.Getenv(testDatabaseURLEnv)
	if dsn == "" {
		dsn = os.Getenv("DATABASE_URL")
	}
	if dsn == "" {
		t.Skip("neither TEST_DATABASE_URL nor DATABASE_URL is set")
	}

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Skipf("cannot parse test database URL: %v", err)
	}
	pingCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		t.Skipf("integration database unreachable (%s): %v", dsn, err)
	}
	t.Cleanup(pool.Close)

	if err := postgres.RunMigrations(dsn); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	if _, err := pool.Exec(context.Background(), "TRUNCATE "+tablesTruncated+" RESTART IDENTITY CASCADE"); err != nil {
		t.Fatalf("truncate tables: %v", err)
	}
	return pool
}

func newTestRouter(t *testing.T, pool *pgxpool.Pool) http.Handler {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	return api.NewRouter(postgres.NewStore(pool), logger)
}

func insertShow(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	var id string
	err := pool.QueryRow(context.Background(),
		`INSERT INTO shows (title) VALUES ($1) RETURNING id`, "Test Show").Scan(&id)
	if err != nil {
		t.Fatalf("insert show: %v", err)
	}
	return id
}

func postEpisode(t *testing.T, h http.Handler, idemKey, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/v1/episodes", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", idemKey)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

type createResponse struct {
	EpisodeID string `json:"episode_id"`
	JobID     string `json:"job_id"`
	Status    string `json:"status"`
}

func decodeCreateResponse(t *testing.T, rec *httptest.ResponseRecorder) createResponse {
	t.Helper()
	var resp createResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response %q: %v", rec.Body.String(), err)
	}
	return resp
}

func countRows(t *testing.T, pool *pgxpool.Pool, query string, args ...any) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatalf("count query %q: %v", query, err)
	}
	return n
}

func episodeBody(showID string) string {
	return fmt.Sprintf(`{"show_id":%q,"episode_number":1,"title":"Ep 1","raw_object_key":"raw/ep1.mp3"}`, showID)
}

// TestCreateEpisodeCreatesJobAndOutboxAtomically verifies a single POST
// inserts exactly one episode, one job, and one outbox event in one
// transaction.
func TestCreateEpisodeCreatesJobAndOutboxAtomically(t *testing.T) {
	pool := newTestPool(t)
	router := newTestRouter(t, pool)
	showID := insertShow(t, pool)

	rec := postEpisode(t, router, "key-atomic", episodeBody(showID))
	if rec.Code != http.StatusAccepted {
		t.Fatalf("POST status = %d, want %d (body %s)", rec.Code, http.StatusAccepted, rec.Body.String())
	}
	resp := decodeCreateResponse(t, rec)
	if resp.Status != "QUEUED" {
		t.Fatalf("status = %q, want QUEUED", resp.Status)
	}
	if resp.EpisodeID == "" || resp.JobID == "" {
		t.Fatalf("missing ids in response: %+v", resp)
	}

	if got := countRows(t, pool, `SELECT count(*) FROM episodes WHERE id = $1`, resp.EpisodeID); got != 1 {
		t.Fatalf("episodes rows = %d, want 1", got)
	}
	if got := countRows(t, pool, `SELECT count(*) FROM jobs WHERE id = $1 AND episode_id = $2 AND status = 'QUEUED' AND idempotency_key = 'key-atomic'`, resp.JobID, resp.EpisodeID); got != 1 {
		t.Fatalf("jobs rows = %d, want 1", got)
	}
	if got := countRows(t, pool, `SELECT count(*) FROM outbox_events WHERE aggregate_id = $1 AND topic = 'audio.jobs.validate' AND sent_at IS NULL`, resp.JobID); got != 1 {
		t.Fatalf("outbox rows = %d, want 1", got)
	}

	var payload string
	if err := pool.QueryRow(context.Background(),
		`SELECT payload FROM outbox_events WHERE aggregate_id = $1`, resp.JobID).Scan(&payload); err != nil {
		t.Fatalf("read outbox payload: %v", err)
	}
	var payloadMap map[string]string
	if err := json.Unmarshal([]byte(payload), &payloadMap); err != nil {
		t.Fatalf("outbox payload is not JSON: %v", err)
	}
	if payloadMap["episode_id"] != resp.EpisodeID || payloadMap["job_id"] != resp.JobID {
		t.Fatalf("outbox payload = %v, want episode_id=%s job_id=%s", payloadMap, resp.EpisodeID, resp.JobID)
	}
}

// TestCreateEpisodeIdempotencyKeyReusesExistingJob verifies a repeated request
// with the same Idempotency-Key returns the existing job (200) and creates no
// new episode, job, or outbox rows.
func TestCreateEpisodeIdempotencyKeyReusesExistingJob(t *testing.T) {
	pool := newTestPool(t)
	router := newTestRouter(t, pool)
	showID := insertShow(t, pool)
	body := episodeBody(showID)

	first := postEpisode(t, router, "key-replay", body)
	if first.Code != http.StatusAccepted {
		t.Fatalf("first POST status = %d, want %d (body %s)", first.Code, http.StatusAccepted, first.Body.String())
	}
	firstResp := decodeCreateResponse(t, first)

	second := postEpisode(t, router, "key-replay", body)
	if second.Code != http.StatusOK {
		t.Fatalf("replay status = %d, want %d (body %s)", second.Code, http.StatusOK, second.Body.String())
	}
	secondResp := decodeCreateResponse(t, second)
	if secondResp.JobID != firstResp.JobID || secondResp.EpisodeID != firstResp.EpisodeID {
		t.Fatalf("replay returned a different resource: first=%+v second=%+v", firstResp, secondResp)
	}

	if got := countRows(t, pool, `SELECT count(*) FROM jobs WHERE episode_id = $1`, firstResp.EpisodeID); got != 1 {
		t.Fatalf("jobs rows = %d, want 1 (replay must not duplicate)", got)
	}
	if got := countRows(t, pool, `SELECT count(*) FROM outbox_events`); got != 1 {
		t.Fatalf("outbox rows = %d, want 1", got)
	}
	if got := countRows(t, pool, `SELECT count(*) FROM episodes WHERE show_id = $1`, showID); got != 1 {
		t.Fatalf("episodes rows = %d, want 1", got)
	}
}

// TestCreateEpisodeIdempotencyKeyConcurrent fires the same request with the
// same Idempotency-Key from several goroutines at once. Exactly one job and
// one outbox row must survive, and every response must reference it.
func TestCreateEpisodeIdempotencyKeyConcurrent(t *testing.T) {
	pool := newTestPool(t)
	router := newTestRouter(t, pool)
	showID := insertShow(t, pool)
	body := episodeBody(showID)

	const n = 8
	type result struct {
		code int
		resp createResponse
		err  error
	}
	results := make([]result, n)

	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodPost, "/v1/episodes", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Idempotency-Key", "key-concurrent")
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			var resp createResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				results[i] = result{err: fmt.Errorf("decode %q: %w", rec.Body.String(), err)}
				return
			}
			results[i] = result{code: rec.Code, resp: resp}
		}(i)
	}
	wg.Wait()

	if got := countRows(t, pool, `SELECT count(*) FROM jobs`); got != 1 {
		t.Fatalf("jobs rows = %d, want exactly 1", got)
	}
	if got := countRows(t, pool, `SELECT count(*) FROM outbox_events`); got != 1 {
		t.Fatalf("outbox rows = %d, want exactly 1", got)
	}

	var jobID string
	for i, r := range results {
		if r.err != nil {
			t.Fatalf("request %d failed: %v", i, r.err)
		}
		if r.code != http.StatusAccepted && r.code != http.StatusOK {
			t.Fatalf("request %d status = %d, want 200 or 202", i, r.code)
		}
		if r.resp.JobID == "" || r.resp.EpisodeID == "" {
			t.Fatalf("request %d returned empty ids: %+v", i, r.resp)
		}
		if i == 0 {
			jobID = r.resp.JobID
			continue
		}
		if r.resp.JobID != jobID {
			t.Fatalf("request %d returned job %s, want %s", i, r.resp.JobID, jobID)
		}
	}
}
