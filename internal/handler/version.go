package handler

import (
	"net/http"
)

func (h *Handler) getVersions(w http.ResponseWriter, r *http.Request) {
	platform := r.URL.Query().Get("platform")
	if platform == "" {
		writeError(w, http.StatusBadRequest, "platform query param is required")
		return
	}

	versions, err := h.svc.GetVersions(r.Context(), platform)
	if err != nil {
		h.log.Error("get versions failed", "platform", platform, "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, versions)
}
