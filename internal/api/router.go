package api

import (
	"log/slog"
	"net/http"

	"github.com/gurshaan17/soundlflow/internal/api/handlers"
	"github.com/gurshaan17/soundlflow/internal/api/middleware"
	"github.com/gurshaan17/soundlflow/internal/storage/postgres"
)

// NewRouter builds the HTTP handler for the API.
func NewRouter(store *postgres.Store, logger *slog.Logger) http.Handler {
	episodes := handlers.NewEpisodeHandler(store, logger)
	health := handlers.NewHealthHandler(store)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", health.Healthz)
	mux.HandleFunc("GET /readyz", health.Readyz)
	mux.HandleFunc("POST /v1/episodes", episodes.Create)
	mux.HandleFunc("GET /v1/episodes/{id}", episodes.GetEpisode)
	mux.HandleFunc("GET /v1/episodes/{id}/jobs/{job_id}", episodes.GetJob)

	return middleware.Recover(logger)(middleware.Logging(logger)(middleware.RequestID(mux)))
}
