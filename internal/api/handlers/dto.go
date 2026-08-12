package handlers

import (
	"time"

	"github.com/gurshaan17/soundlflow/internal/domain"
)

type episodeResponse struct {
	ID            string       `json:"id"`
	ShowID        string       `json:"show_id"`
	EpisodeNumber int          `json:"episode_number"`
	Title         *string      `json:"title"`
	RawObjectKey  string       `json:"raw_object_key"`
	RawChecksum   *string      `json:"raw_checksum"`
	Status        string       `json:"status"`
	CreatedAt     time.Time    `json:"created_at"`
	UpdatedAt     time.Time    `json:"updated_at"`
	CurrentJob    *jobResponse `json:"current_job"`
}

type jobResponse struct {
	ID           string    `json:"id"`
	EpisodeID    string    `json:"episode_id"`
	Status       string    `json:"status"`
	CurrentStep  *string   `json:"current_step"`
	Attempt      int       `json:"attempt"`
	MaxAttempts  int       `json:"max_attempts"`
	ErrorMessage *string   `json:"error_message"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type jobStepResponse struct {
	ID           string     `json:"id"`
	JobID        string     `json:"job_id"`
	StepName     string     `json:"step_name"`
	Status       string     `json:"status"`
	Attempt      int        `json:"attempt"`
	StartedAt    *time.Time `json:"started_at"`
	FinishedAt   *time.Time `json:"finished_at"`
	ErrorMessage *string    `json:"error_message"`
}

type getJobResponse struct {
	Job   *jobResponse      `json:"job"`
	Steps []jobStepResponse `json:"steps"`
}

func episodeToResponse(ep domain.Episode, currentJob *jobResponse) episodeResponse {
	return episodeResponse{
		ID:            ep.ID,
		ShowID:        ep.ShowID,
		EpisodeNumber: ep.EpisodeNumber,
		Title:         ep.Title,
		RawObjectKey:  ep.RawObjectKey,
		RawChecksum:   ep.RawChecksum,
		Status:        string(ep.Status),
		CreatedAt:     ep.CreatedAt,
		UpdatedAt:     ep.UpdatedAt,
		CurrentJob:    currentJob,
	}
}

func jobToResponse(j domain.Job) *jobResponse {
	currentStep := (*string)(nil)
	if j.CurrentStep != nil {
		s := string(*j.CurrentStep)
		currentStep = &s
	}
	return &jobResponse{
		ID:           j.ID,
		EpisodeID:    j.EpisodeID,
		Status:       string(j.Status),
		CurrentStep:  currentStep,
		Attempt:      j.Attempt,
		MaxAttempts:  j.MaxAttempts,
		ErrorMessage: j.ErrorMessage,
		CreatedAt:    j.CreatedAt,
		UpdatedAt:    j.UpdatedAt,
	}
}

func stepsToResponses(steps []domain.JobStep) []jobStepResponse {
	out := make([]jobStepResponse, 0, len(steps))
	for _, s := range steps {
		out = append(out, jobStepResponse{
			ID:           s.ID,
			JobID:        s.JobID,
			StepName:     string(s.StepName),
			Status:       string(s.Status),
			Attempt:      s.Attempt,
			StartedAt:    s.StartedAt,
			FinishedAt:   s.FinishedAt,
			ErrorMessage: s.ErrorMessage,
		})
	}
	return out
}
