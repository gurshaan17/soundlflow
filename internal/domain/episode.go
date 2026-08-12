package domain

import "time"

type EpisodeStatus string

const (
	EpisodeStatusUploaded EpisodeStatus = "UPLOADED"
)

type Episode struct {
	ID            string
	ShowID        string
	EpisodeNumber int
	Title         *string
	RawObjectKey  string
	RawChecksum   *string
	Status        EpisodeStatus
	CreatedAt     time.Time
	UpdatedAt     time.Time
}
