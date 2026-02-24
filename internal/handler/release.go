package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"jiraiya/internal/service"
)

func (h *Handler) submitRelease(w http.ResponseWriter, r *http.Request) {
	var sub service.ReleaseSubmission
	if err := json.NewDecoder(r.Body).Decode(&sub); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}

	if err := h.svc.SubmitRelease(r.Context(), sub); err != nil {
		var ve *service.ValidationError
		if errors.As(err, &ve) {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"error":   "validation failed",
				"details": ve.Details,
			})
			return
		}
		h.log.Error("submit release failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) getReleases(w http.ResponseWriter, r *http.Request) {
	version := r.URL.Query().Get("version")
	platform := r.URL.Query().Get("platform")
	if version == "" && platform == "" {
		writeError(w, http.StatusBadRequest, "version or platform query param is required")
		return
	}

	releases, err := h.svc.GetReleases(r.Context(), version, platform)
	if err != nil {
		h.log.Error("get releases failed", "version", version, "platform", platform, "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, releases)
}

func (h *Handler) deleteRelease(w http.ResponseWriter, r *http.Request) {
	version := chi.URLParam(r, "version")
	if version == "" {
		writeError(w, http.StatusBadRequest, "version is required")
		return
	}

	if err := h.svc.DeleteRelease(r.Context(), version); err != nil {
		h.log.Error("delete release failed", "version", version, "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
