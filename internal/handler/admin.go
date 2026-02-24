package handler

import (
	"net/http"
)

func (h *Handler) getTree(w http.ResponseWriter, r *http.Request) {
	platform := r.URL.Query().Get("platform")
	if platform == "" {
		writeError(w, http.StatusBadRequest, "platform query param is required")
		return
	}

	info, err := h.svc.GetTreeInfo(r.Context(), platform)
	if err != nil {
		h.log.Error("get tree failed", "platform", platform, "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, info)
}
