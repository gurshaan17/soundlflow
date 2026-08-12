package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store groups the repositories behind a single pgx pool.
type Store struct {
	pool *pgxpool.Pool

	Episodes *EpisodeRepo
	Jobs     *JobRepo
	Outbox   *OutboxRepo
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{
		pool:     pool,
		Episodes: &EpisodeRepo{},
		Jobs:     &JobRepo{},
		Outbox:   &OutboxRepo{},
	}
}

// Begin starts a transaction that repository methods can run inside.
func (s *Store) Begin(ctx context.Context) (pgx.Tx, error) {
	return s.pool.Begin(ctx)
}

// Pool exposes the underlying pool, e.g. to re-read a row after a failed
// transaction.
func (s *Store) Pool() *pgxpool.Pool {
	return s.pool
}

func (s *Store) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

func (s *Store) Close() {
	s.pool.Close()
}

func mapNoRows(err error, notFound error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return notFound
	}
	return err
}

func isPgCode(err error, code string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == code
}

func isPgConstraint(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.ConstraintName == constraint
}
