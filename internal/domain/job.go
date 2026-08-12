package domain

import "time"

type JobStatus string

const (
	JobStatusQueued     JobStatus = "QUEUED"
	JobStatusProcessing JobStatus = "PROCESSING"
	JobStatusFailed     JobStatus = "FAILED"
	JobStatusRetrying   JobStatus = "RETRYING"
	JobStatusCompleted  JobStatus = "COMPLETED"
	JobStatusDLQ        JobStatus = "DLQ"
)

type StepName string

const (
	StepNameValidate  StepName = "VALIDATE"
	StepNameTranscode StepName = "TRANSCODE"
	StepNameAnalyze   StepName = "ANALYZE"
	StepNameWaveform  StepName = "WAVEFORM"
	StepNameNormalize StepName = "NORMALIZE"
	StepNameUpload    StepName = "UPLOAD"
)

type Job struct {
	ID             string
	EpisodeID      string
	Status         JobStatus
	CurrentStep    *StepName
	Attempt        int
	MaxAttempts    int
	IdempotencyKey *string
	ErrorMessage   *string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type JobStepStatus string

const (
	JobStepStatusPending  JobStepStatus = "PENDING"
	JobStepStatusRunning  JobStepStatus = "RUNNING"
	JobStepStatusSuccess  JobStepStatus = "SUCCESS"
	JobStepStatusFailed   JobStepStatus = "FAILED"
	JobStepStatusSkipped  JobStepStatus = "SKIPPED"
)

type JobStep struct {
	ID           string
	JobID        string
	StepName     StepName
	Status       JobStepStatus
	Attempt      int
	StartedAt    *time.Time
	FinishedAt   *time.Time
	ErrorMessage *string
}
