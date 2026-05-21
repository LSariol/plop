package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/lsariol/plop/web"
)

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<10)
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if body.Username == "" || body.Password == "" {
		writeError(w, "username and password are required", http.StatusBadRequest)
		return
	}

	authenticated, err := h.store.Authenticate(r.Context(), body.Username, body.Password)
	if err != nil {
		slog.Error("authenticate", "user", body.Username, "error", err)
		writeError(w, "internal error", http.StatusInternalServerError)
		return
	}
	if !authenticated {
		slog.Warn("login failed", "user", body.Username, "ip", h.clientIP(r))
		writeError(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	token, err := h.store.CreateSession(r.Context(), body.Username, 30*24*time.Hour)
	if err != nil {
		slog.Error("create session", "user", body.Username, "error", err)
		writeError(w, "internal error", http.StatusInternalServerError)
		return
	}

	slog.Info("login", "user", body.Username, "ip", h.clientIP(r))

	secure := r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    token,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
		MaxAge:   int((30 * 24 * time.Hour).Seconds()),
	})
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) ServeLogin(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie("session"); err == nil {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		var username string
		err := h.pool.QueryRow(ctx,
			`SELECT username FROM sessions
             WHERE token = $1 AND expires_at > now()`,
			cookie.Value,
		).Scan(&username)
		if err == nil {
			http.Redirect(w, r, "/app", http.StatusFound)
			return
		}
	}
	http.ServeFileFS(w, r, web.Files, "login.html")
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session")
	if err != nil {
		w.WriteHeader(http.StatusOK)
		return
	}

	if err := h.store.DeleteSession(r.Context(), cookie.Value); err != nil {
		slog.Error("delete session", "error", err)
		writeError(w, "internal error", http.StatusInternalServerError)
		return
	}

	slog.Info("logout", "user", usernameFromCtx(r), "ip", h.clientIP(r))

	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "",
		HttpOnly: true,
		Secure:   r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https",
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
		MaxAge:   -1,
	})
	w.WriteHeader(http.StatusOK)
}
