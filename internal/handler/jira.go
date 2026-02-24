package handler

import (
	"net/http"
)

func (h *Handler) getJiras(w http.ResponseWriter, r *http.Request) {
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")
	if from == "" || to == "" {
		writeError(w, http.StatusBadRequest, "from and to query params are required")
		return
	}

	jiras, err := h.svc.GetJirasBetweenVersions(r.Context(), from, to)
	if err != nil {
		h.log.Error("get jiras failed", "from", from, "to", to, "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, jiras)
}

func (h *Handler) getFilters(w http.ResponseWriter, r *http.Request) {
	platform := r.URL.Query().Get("platform")
	if platform == "" {
		writeError(w, http.StatusBadRequest, "platform query param is required")
		return
	}

	filters, err := h.svc.GetFilters(r.Context(), platform)
	if err != nil {
		h.log.Error("get filters failed", "platform", platform, "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, filters)
}
