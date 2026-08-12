package postgres

import (
	"context"
	"errors"

	"github.com/gurshaan17/soundlflow/internal/domain"
	"github.com/jackc/pgx/v5/pgconn"
)

type EpisodeRepo struct{}

const (
	episodeColumns = "id, show_id, episode_number, title, raw_object_key, raw_checksum, status, created_at, updated_at"

	episodeInsert = `
INSERT INTO episodes (show_id, episode_number, title, raw_object_key, raw_checksum, status)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING ` + episodeColumns

	episodeSelectByID = `
SELECT ` + episodeColumns + `
FROM episodes
WHERE id = $1`
)

func (r *EpisodeRepo) Create(ctx context.Context, q Querier, ep domain.Episode) (domain.Episode, error) {
	var (
		out      domain.Episode
		title    *string
		checksum *string
		status   string
	)
	err := q.QueryRow(ctx, episodeInsert,
		ep.ShowID,
		ep.EpisodeNumber,
		ep.Title,
		ep.RawObjectKey,
		ep.RawChecksum,
		string(ep.Status),
	).Scan(
		&out.ID,
		&out.ShowID,
		&out.EpisodeNumber,
		&title,
		&out.RawObjectKey,
		&checksum,
		&status,
		&out.CreatedAt,
		&out.UpdatedAt,
	)
	if err != nil {
		return domain.Episode{}, r.mapCreateError(err)
	}
	out.Title = title
	out.RawChecksum = checksum
	out.Status = domain.EpisodeStatus(status)
	return out, nil
}

func (r *EpisodeRepo) GetByID(ctx context.Context, q Querier, id string) (domain.Episode, error) {
	var (
		out      domain.Episode
		title    *string
		checksum *string
		status   string
	)
	err := q.QueryRow(ctx, episodeSelectByID, id).Scan(
		&out.ID,
		&out.ShowID,
		&out.EpisodeNumber,
		&title,
		&out.RawObjectKey,
		&checksum,
		&status,
		&out.CreatedAt,
		&out.UpdatedAt,
	)
	if err != nil {
		return domain.Episode{}, mapNoRows(err, domain.ErrEpisodeNotFound)
	}
	out.Title = title
	out.RawChecksum = checksum
	out.Status = domain.EpisodeStatus(status)
	return out, nil
}

func (r *EpisodeRepo) mapCreateError(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return err
	}
	switch {
	case pgErr.Code == "23505" && pgErr.ConstraintName == "episodes_show_id_episode_number_key":
		return domain.ErrEpisodeConflict
	case pgErr.Code == "23503" && pgErr.ConstraintName == "episodes_show_id_fkey":
		return domain.ErrShowNotFound
	default:
		return err
	}
}
