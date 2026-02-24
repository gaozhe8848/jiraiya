package handler

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"jiraiya/internal/service"
)

// Handler wraps the service and provides HTTP route registration.
type Handler struct {
	svc service.Service
	log *slog.Logger
}

// New creates a new Handler.
func New(svc service.Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

// Routes returns the chi router with all routes registered.
func (h *Handler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Use(requestLogger(h.log))

	r.Get("/api/releases", h.getReleases)
	r.Put("/api/releases", h.submitRelease)
	r.Delete("/api/releases/{version}", h.deleteRelease)
	r.Get("/api/filters", h.getFilters)
	r.Get("/api/versions", h.getVersions)
	r.Get("/api/jiras", h.getJiras)
	r.Get("/api/admin/tree", h.getTree)

	return r
}
