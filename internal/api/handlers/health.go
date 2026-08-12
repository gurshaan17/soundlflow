package handlers

import (
	"net/http"

	"github.com/gurshaan17/soundlflow/internal/storage/postgres"
)

// HealthHandler serves liveness and readiness probes.
type HealthHandler struct {
	store *postgres.Store
}

func NewHealthHandler(store *postgres.Store) *HealthHandler {
	return &HealthHandler{store: store}
}

func (h *HealthHandler) Healthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func (h *HealthHandler) Readyz(w http.ResponseWriter, r *http.Request) {
	if err := h.store.Ping(r.Context()); err != nil {
		writeError(w, http.StatusServiceUnavailable, "database unreachable")
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}
