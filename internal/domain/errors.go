package domain

import "errors"

var (
	ErrNotFound = errors.New("soundflow: not found")

	ErrEpisodeNotFound = errors.New("soundflow: episode not found")
	ErrJobNotFound     = errors.New("soundflow: job not found")
	ErrShowNotFound    = errors.New("soundflow: show not found")

	// ErrConflict is returned when a uniqueness constraint is violated by a
	// genuine duplicate (not an idempotent retry), e.g. the same episode
	// re-posted under a different Idempotency-Key.
	ErrConflict        = errors.New("soundflow: conflict")
	ErrEpisodeConflict = errors.New("soundflow: episode already exists")

	// ErrIdempotencyKeyTaken is returned when a job insert races with another
	// request that already holds the same idempotency key. The caller should
	// fetch and return the existing job.
	ErrIdempotencyKeyTaken = errors.New("soundflow: idempotency key already taken")
)
