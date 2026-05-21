package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/lsariol/plop/internal/store"
)

func (h *Handler) GetDesktops(w http.ResponseWriter, r *http.Request) {
	userID := usernameFromCtx(r)
	desktops, err := h.store.GetUserDesktops(r.Context(), userID)
	if err != nil {
		slog.Error("get desktops", "user", userID, "error", err)
		writeError(w, "internal error", http.StatusInternalServerError)
		return
	}
	if desktops == nil {
		desktops = []store.DesktopInfo{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(desktops)
}

func (h *Handler) DeleteDesktop(w http.ResponseWriter, r *http.Request) {
	userID := usernameFromCtx(r)
	desktopID := r.PathValue("id")

	if err := h.store.DeleteDesktop(r.Context(), desktopID, userID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, "desktop not found", http.StatusNotFound)
			return
		}
		slog.Error("delete desktop", "desktop_id", desktopID, "user", userID, "error", err)
		writeError(w, "internal error", http.StatusInternalServerError)
		return
	}

	slog.Info("desktop revoked", "desktop_id", desktopID, "user", userID)
	w.WriteHeader(http.StatusNoContent)
}
