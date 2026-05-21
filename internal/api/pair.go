package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/lsariol/plop/internal/store"
)

func (h *Handler) GeneratePairingCode(w http.ResponseWriter, r *http.Request) {
	userID := usernameFromCtx(r)
	code, err := h.store.CreatePairingCode(r.Context(), userID)
	if err != nil {
		slog.Error("generate pairing code", "user", userID, "error", err)
		writeError(w, "internal error", http.StatusInternalServerError)
		return
	}
	slog.Info("pairing code generated", "user", userID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"code":       code,
		"expires_in": 900,
	})
}

func (h *Handler) PairDesktop(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<10)
	var body struct {
		PairingCode string `json:"pairing_code"`
		MachineName string `json:"machine_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if body.PairingCode == "" || body.MachineName == "" {
		writeError(w, "pairing_code and machine_name are required", http.StatusBadRequest)
		return
	}

	userID, err := h.store.ConsumePairingCode(r.Context(), body.PairingCode)
	if errors.Is(err, store.ErrNotFound) {
		slog.Warn("invalid pairing code", "machine", body.MachineName, "ip", h.clientIP(r))
		writeError(w, "invalid or expired pairing code", http.StatusBadRequest)
		return
	}
	if err != nil {
		slog.Error("consume pairing code", "error", err)
		writeError(w, "internal error", http.StatusInternalServerError)
		return
	}

	token, err := h.store.CreateDesktop(r.Context(), userID, body.MachineName)
	if err != nil {
		slog.Error("create desktop", "user", userID, "error", err)
		writeError(w, "internal error", http.StatusInternalServerError)
		return
	}

	slog.Info("desktop paired", "machine", body.MachineName, "user", userID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"desktop_token": token})
}
