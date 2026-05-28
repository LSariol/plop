package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"golang.org/x/crypto/bcrypt"
)

func (h *Handler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<10)
	var body struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if body.CurrentPassword == "" || body.NewPassword == "" {
		writeError(w, "current_password and new_password are required", http.StatusBadRequest)
		return
	}
	if len(body.NewPassword) < 8 {
		writeError(w, "new password must be at least 8 characters", http.StatusBadRequest)
		return
	}

	username := usernameFromCtx(r)

	ok, err := h.store.Authenticate(r.Context(), username, body.CurrentPassword)
	if err != nil {
		slog.Error("change password: authenticate", "user", username, "error", err)
		writeError(w, "internal error", http.StatusInternalServerError)
		return
	}
	if !ok {
		writeError(w, "current password is incorrect", http.StatusUnauthorized)
		return
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(body.NewPassword), 12)
	if err != nil {
		slog.Error("change password: hash", "user", username, "error", err)
		writeError(w, "internal error", http.StatusInternalServerError)
		return
	}

	if err := h.store.UpdatePassword(r.Context(), username, string(newHash)); err != nil {
		slog.Error("change password: update", "user", username, "error", err)
		writeError(w, "internal error", http.StatusInternalServerError)
		return
	}

	slog.Info("password changed", "user", username, "ip", h.clientIP(r))
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) DeleteAccount(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<10)
	var body struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if body.Password == "" {
		writeError(w, "password is required", http.StatusBadRequest)
		return
	}

	username := usernameFromCtx(r)

	ok, err := h.store.Authenticate(r.Context(), username, body.Password)
	if err != nil {
		slog.Error("delete account: authenticate", "user", username, "error", err)
		writeError(w, "internal error", http.StatusInternalServerError)
		return
	}
	if !ok {
		writeError(w, "incorrect password", http.StatusUnauthorized)
		return
	}

	// Collect payload directories before the DB delete so we can clean up disk.
	ids, err := h.store.UserPayloadIDs(r.Context(), username)
	if err != nil {
		slog.Error("delete account: list payloads", "user", username, "error", err)
		writeError(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Delete the user — ON DELETE CASCADE removes sessions, desktops, pairing_codes, payloads.
	if err := h.store.DeleteUser(r.Context(), username); err != nil {
		slog.Error("delete account: delete user", "user", username, "error", err)
		writeError(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Best-effort disk cleanup; log failures but don't fail the response.
	for _, id := range ids {
		if err := os.RemoveAll(filepath.Join(h.cfg.PayloadDir, id)); err != nil {
			slog.Warn("delete account: remove payload dir", "id", id, "error", err)
		}
	}

	slog.Info("account deleted", "user", username, "ip", h.clientIP(r))

	// Clear the session cookie.
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "",
		HttpOnly: true,
		Secure:   r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https",
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
		MaxAge:   -1,
	})
	w.WriteHeader(http.StatusNoContent)
}
