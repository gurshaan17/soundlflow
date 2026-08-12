package postgres

import "github.com/gurshaan17/soundlflow/internal/domain"

// stepNamePtr converts a *StepName into a *string for use as a SQL parameter.
func stepNamePtr(s *domain.StepName) *string {
	if s == nil {
		return nil
	}
	v := string(*s)
	return &v
}

// stringToStepNamePtr converts a scanned *string into a *StepName.
func stringToStepNamePtr(s *string) *domain.StepName {
	if s == nil {
		return nil
	}
	v := domain.StepName(*s)
	return &v
}
