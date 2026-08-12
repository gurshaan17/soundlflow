package domain

import (
	"encoding/json"
	"time"
)

type OutboxEvent struct {
	ID          int64
	AggregateID string
	Topic       string
	Payload     json.RawMessage
	CreatedAt   time.Time
	SentAt      *time.Time
}
