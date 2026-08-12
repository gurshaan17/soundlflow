package postgres

import (
	"context"
	"encoding/json"

	"github.com/gurshaan17/soundlflow/internal/domain"
)

type OutboxRepo struct{}

const (
	outboxInsert = `
INSERT INTO outbox_events (aggregate_id, topic, payload)
VALUES ($1, $2, $3::jsonb)
RETURNING id, aggregate_id, topic, payload, created_at, sent_at`
)

func (r *OutboxRepo) Create(ctx context.Context, q Querier, evt domain.OutboxEvent) (domain.OutboxEvent, error) {
	var (
		out     domain.OutboxEvent
		payload string
	)
	err := q.QueryRow(ctx, outboxInsert,
		evt.AggregateID,
		evt.Topic,
		string(evt.Payload),
	).Scan(
		&out.ID,
		&out.AggregateID,
		&out.Topic,
		&payload,
		&out.CreatedAt,
		&out.SentAt,
	)
	if err != nil {
		return domain.OutboxEvent{}, err
	}
	out.Payload = json.RawMessage(payload)
	return out, nil
}
