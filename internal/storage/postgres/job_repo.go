package postgres

import (
	"context"
	"errors"

	"github.com/gurshaan17/soundlflow/internal/domain"
	"github.com/jackc/pgx/v5/pgconn"
)

type JobRepo struct{}

const (
	jobColumns = "id, episode_id, status, current_step, attempt, max_attempts, idempotency_key, error_message, created_at, updated_at"

	jobInsert = `
INSERT INTO jobs (episode_id, status, current_step, attempt, max_attempts, idempotency_key, error_message)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING ` + jobColumns

	jobSelectByID = `
SELECT ` + jobColumns + `
FROM jobs
WHERE id = $1`

	jobSelectByEpisodeLatest = `
SELECT ` + jobColumns + `
FROM jobs
WHERE episode_id = $1
ORDER BY created_at DESC, id DESC
LIMIT 1`

	jobSelectByIdempotencyKey = `
SELECT ` + jobColumns + `
FROM jobs
WHERE idempotency_key = $1`
)

func (r *JobRepo) Create(ctx context.Context, q Querier, job domain.Job) (domain.Job, error) {
	out, err := r.scanJob(q.QueryRow(ctx, jobInsert,
		job.EpisodeID,
		string(job.Status),
		stepNamePtr(job.CurrentStep),
		job.Attempt,
		job.MaxAttempts,
		job.IdempotencyKey,
		job.ErrorMessage,
	))
	if err != nil {
		return domain.Job{}, r.mapCreateError(err)
	}
	return out, nil
}

func (r *JobRepo) GetByID(ctx context.Context, q Querier, id string) (domain.Job, error) {
	return r.scanJob(q.QueryRow(ctx, jobSelectByID, id))
}

// GetLatestByEpisode returns the most recently created job for an episode.
func (r *JobRepo) GetLatestByEpisode(ctx context.Context, q Querier, episodeID string) (domain.Job, error) {
	return r.scanJob(q.QueryRow(ctx, jobSelectByEpisodeLatest, episodeID))
}

// GetByIDempotencyKey returns the job previously created for the given
// idempotency key, if any.
func (r *JobRepo) GetByIDempotencyKey(ctx context.Context, q Querier, key string) (domain.Job, error) {
	return r.scanJob(q.QueryRow(ctx, jobSelectByIdempotencyKey, key))
}

// GetSteps returns every job_steps row for a job.
func (r *JobRepo) GetSteps(ctx context.Context, q Querier, jobID string) ([]domain.JobStep, error) {
	rows, err := q.Query(ctx, `
SELECT id, job_id, step_name, status, attempt, started_at, finished_at, error_message
FROM job_steps
WHERE job_id = $1
ORDER BY started_at NULLS LAST, id`, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	steps := make([]domain.JobStep, 0)
	for rows.Next() {
		var (
			s        domain.JobStep
			stepName string
			status   string
			errMsg   *string
		)
		if err := rows.Scan(&s.ID, &s.JobID, &stepName, &status, &s.Attempt, &s.StartedAt, &s.FinishedAt, &errMsg); err != nil {
			return nil, err
		}
		s.StepName = domain.StepName(stepName)
		s.Status = domain.JobStepStatus(status)
		s.ErrorMessage = errMsg
		steps = append(steps, s)
	}
	return steps, rows.Err()
}

func (r *JobRepo) scanJob(row Row) (domain.Job, error) {
	var (
		out     domain.Job
		status  string
		step    *string
		idemKey *string
		errMsg  *string
	)
	err := row.Scan(
		&out.ID,
		&out.EpisodeID,
		&status,
		&step,
		&out.Attempt,
		&out.MaxAttempts,
		&idemKey,
		&errMsg,
		&out.CreatedAt,
		&out.UpdatedAt,
	)
	if err != nil {
		return domain.Job{}, mapNoRows(err, domain.ErrJobNotFound)
	}
	out.Status = domain.JobStatus(status)
	out.CurrentStep = stringToStepNamePtr(step)
	out.IdempotencyKey = idemKey
	out.ErrorMessage = errMsg
	return out, nil
}

func (r *JobRepo) mapCreateError(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return err
	}
	switch {
	case pgErr.Code == "23505" && pgErr.ConstraintName == "jobs_idempotency_key":
		return domain.ErrIdempotencyKeyTaken
	case pgErr.Code == "23503" && pgErr.ConstraintName == "jobs_episode_id_fkey":
		return domain.ErrEpisodeNotFound
	default:
		return err
	}
}
