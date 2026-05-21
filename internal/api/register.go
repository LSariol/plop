package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/lsariol/plop/internal/store"
)

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	if !h.cfg.AllowRegistration {
		writeError(w, "registration is disabled", http.StatusForbidden)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<10)
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if len(body.Username) < 2 || len(body.Username) > 40 {
		writeError(w, "username must be 2–40 characters", http.StatusBadRequest)
		return
	}
	if len(body.Password) < 8 {
		writeError(w, "password must be at least 8 characters", http.StatusBadRequest)
		return
	}

	if err := h.store.CreateUser(r.Context(), body.Username, body.Password); err != nil {
		if errors.Is(err, store.ErrUserExists) {
			slog.Warn("register conflict", "user", body.Username, "ip", h.clientIP(r))
			writeError(w, "username already taken", http.StatusConflict)
			return
		}
		slog.Error("register user", "user", body.Username, "error", err)
		writeError(w, "internal error", http.StatusInternalServerError)
		return
	}

	slog.Info("register", "user", body.Username, "ip", h.clientIP(r))
	w.WriteHeader(http.StatusCreated)
}
