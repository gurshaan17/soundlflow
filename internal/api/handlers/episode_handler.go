package handlers

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gurshaan17/soundlflow/internal/domain"
	"github.com/gurshaan17/soundlflow/internal/storage/postgres"
)

// EpisodeHandler handles /v1/episodes routes.
type EpisodeHandler struct {
	store  *postgres.Store
	logger *slog.Logger
}

func NewEpisodeHandler(store *postgres.Store, logger *slog.Logger) *EpisodeHandler {
	return &EpisodeHandler{store: store, logger: logger}
}

const (
	topicJobsValidate = "audio.jobs.validate"
	maxAttempts       = 3
)

type createEpisodeRequest struct {
	ShowID        string `json:"show_id"`
	EpisodeNumber int    `json:"episode_number"`
	Title         string `json:"title"`
	RawObjectKey  string `json:"raw_object_key"`
}

func (r createEpisodeRequest) validate() string {
	if !isUUID(r.ShowID) {
		return "show_id must be a valid UUID"
	}
	if r.EpisodeNumber < 1 {
		return "episode_number must be a positive integer"
	}
	if strings.TrimSpace(r.RawObjectKey) == "" {
		return "raw_object_key is required"
	}
	return ""
}

type createEpisodeResponse struct {
	EpisodeID string `json:"episode_id"`
	JobID     string `json:"job_id"`
	Status    string `json:"status"`
}

// Create handles POST /v1/episodes. In one transaction it inserts the episode,
// a QUEUED job, and an outbox event, unless the Idempotency-Key was already
// used, in which case the existing job is returned.
func (h *EpisodeHandler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	idemKey := r.Header.Get("Idempotency-Key")
	if idemKey == "" {
		writeError(w, http.StatusBadRequest, "Idempotency-Key header is required")
		return
	}
	if len(idemKey) > 255 {
		writeError(w, http.StatusBadRequest, "Idempotency-Key must be at most 255 characters")
		return
	}

	var req createEpisodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if msg := req.validate(); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}

	tx, err := h.store.Begin(ctx)
	if err != nil {
		h.logger.Error("begin transaction", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	defer tx.Rollback(ctx)

	// Fast path: this idempotency key already produced a job.
	existing, err := h.store.Jobs.GetByIDempotencyKey(ctx, tx, idemKey)
	if err == nil {
		if err := tx.Commit(ctx); err != nil {
			h.logger.Error("commit transaction", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		h.writeCreateResponse(w, http.StatusOK, existing)
		return
	}
	if !errors.Is(err, domain.ErrJobNotFound) {
		h.logger.Error("lookup idempotency key", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	episode, err := h.store.Episodes.Create(ctx, tx, domain.Episode{
		ShowID:        req.ShowID,
		EpisodeNumber: req.EpisodeNumber,
		Title:         strPtrOrNil(req.Title),
		RawObjectKey:  req.RawObjectKey,
		Status:        domain.EpisodeStatusUploaded,
	})
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrShowNotFound):
			writeError(w, http.StatusBadRequest, "show not found")
		case errors.Is(err, domain.ErrEpisodeConflict):
			writeError(w, http.StatusConflict, "episode already exists")
		default:
			h.logger.Error("insert episode", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	job, err := h.store.Jobs.Create(ctx, tx, domain.Job{
		EpisodeID:      episode.ID,
		Status:         domain.JobStatusQueued,
		Attempt:        0,
		MaxAttempts:    maxAttempts,
		IdempotencyKey: &idemKey,
	})
	if err != nil {
		if errors.Is(err, domain.ErrIdempotencyKeyTaken) {
			// A concurrent request with the same key won the race and this
			// transaction is now aborted. Return the existing job instead.
			if rerr := tx.Rollback(ctx); rerr != nil {
				h.logger.Error("rollback after idempotency conflict", "error", rerr)
			}
			winner, getErr := h.store.Jobs.GetByIDempotencyKey(ctx, h.store.Pool(), idemKey)
			if getErr != nil {
				h.logger.Error("refetch idempotent job", "error", getErr)
				writeError(w, http.StatusInternalServerError, "internal server error")
				return
			}
			h.writeCreateResponse(w, http.StatusOK, winner)
			return
		}
		h.logger.Error("insert job", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	payload, err := json.Marshal(map[string]string{
		"episode_id": episode.ID,
		"job_id":     job.ID,
	})
	if err != nil {
		h.logger.Error("marshal outbox payload", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	if _, err := h.store.Outbox.Create(ctx, tx, domain.OutboxEvent{
		AggregateID: job.ID,
		Topic:       topicJobsValidate,
		Payload:     payload,
	}); err != nil {
		h.logger.Error("insert outbox event", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	if err := tx.Commit(ctx); err != nil {
		h.logger.Error("commit transaction", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	h.writeCreateResponse(w, http.StatusAccepted, job)
}

// GetEpisode handles GET /v1/episodes/{id}, returning the episode plus its
// most recent job.
func (h *EpisodeHandler) GetEpisode(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")
	if !isUUID(id) {
		writeError(w, http.StatusBadRequest, "invalid episode id")
		return
	}

	episode, err := h.store.Episodes.GetByID(ctx, h.store.Pool(), id)
	if err != nil {
		if errors.Is(err, domain.ErrEpisodeNotFound) {
			writeError(w, http.StatusNotFound, "episode not found")
		} else {
			h.logger.Error("get episode", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	var currentJob *jobResponse
	job, err := h.store.Jobs.GetLatestByEpisode(ctx, h.store.Pool(), episode.ID)
	switch {
	case err == nil:
		currentJob = jobToResponse(job)
	case errors.Is(err, domain.ErrJobNotFound):
		currentJob = nil
	default:
		h.logger.Error("get latest job", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusOK, episodeToResponse(episode, currentJob))
}

// GetJob handles GET /v1/episodes/{id}/jobs/{job_id}, returning the job with
// all of its job_steps rows.
func (h *EpisodeHandler) GetJob(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	episodeID := r.PathValue("id")
	jobID := r.PathValue("job_id")
	if !isUUID(episodeID) || !isUUID(jobID) {
		writeError(w, http.StatusBadRequest, "invalid episode or job id")
		return
	}

	job, err := h.store.Jobs.GetByID(ctx, h.store.Pool(), jobID)
	if err != nil {
		if errors.Is(err, domain.ErrJobNotFound) {
			writeError(w, http.StatusNotFound, "job not found")
		} else {
			h.logger.Error("get job", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}
	if !strings.EqualFold(job.EpisodeID, episodeID) {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}

	steps, err := h.store.Jobs.GetSteps(ctx, h.store.Pool(), jobID)
	if err != nil {
		h.logger.Error("get job steps", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusOK, getJobResponse{
		Job:   jobToResponse(job),
		Steps: stepsToResponses(steps),
	})
}

func (h *EpisodeHandler) writeCreateResponse(w http.ResponseWriter, status int, job domain.Job) {
	writeJSON(w, status, createEpisodeResponse{
		EpisodeID: job.EpisodeID,
		JobID:     job.ID,
		Status:    string(job.Status),
	})
}

func strPtrOrNil(s string) *string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return &s
}
