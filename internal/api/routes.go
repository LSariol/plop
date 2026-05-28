package api

import (
	"context"
	"net/http"
	"time"

	"github.com/lsariol/plop/web"
)

func RegisterRoutes(
	mux *http.ServeMux,
	h *Handler,
	ctx context.Context,
	requireSession func(http.Handler) http.Handler,
	requireClient func(http.Handler) http.Handler,
) {
	// Static files — served from the embedded FS compiled into the binary.
	fileServer := http.FileServerFS(web.Files)
	mux.Handle("/static/", http.StripPrefix("/static/", fileServer))

	// Rate limiters — keyed by client IP, tied to server lifetime via ctx.
	loginLimiter    := newIPLimiter(ctx, 5, time.Minute)  // 5 attempts/min/IP
	registerLimiter := newIPLimiter(ctx, 3, time.Minute)  // 3 registrations/min/IP
	pairLimiter     := newIPLimiter(ctx, 10, time.Minute) // 10 pair attempts/min/IP
	uploadLimiter   := newIPLimiter(ctx, 30, time.Minute) // 30 uploads/min/IP

	// Public routes
	mux.Handle("GET /{$}", serveFileFS("home.html"))
	mux.HandleFunc("GET /login", h.ServeLogin)
	mux.Handle("GET /create-account", serveFileFS("createaccount.html"))
	mux.HandleFunc("GET /health", h.Health)
	mux.Handle("POST /auth/login", withRateLimit(loginLimiter, h.clientIP)(http.HandlerFunc(h.Login)))
	mux.Handle("POST /auth/register", withRateLimit(registerLimiter, h.clientIP)(http.HandlerFunc(h.Register)))

	// Service worker must be served from root so its scope covers the whole app.
	// Served via a handler (not the static embed) so the asset hash is injected.
	mux.HandleFunc("GET /sw.js", h.ServeServiceWorker)

	// PWA routes — session cookie required
	mux.Handle("GET /app", requireSession(serveFileFS("app.html")))
	mux.Handle("GET /settings", requireSession(serveFileFS("settings.html")))
	mux.Handle("POST /upload", requireSession(withRateLimit(uploadLimiter, h.clientIP)(http.HandlerFunc(h.Upload))))
	mux.Handle("POST /auth/logout", requireSession(http.HandlerFunc(h.Logout)))
	mux.Handle("PUT /auth/password", requireSession(http.HandlerFunc(h.ChangePassword)))
	mux.Handle("DELETE /auth/account", requireSession(http.HandlerFunc(h.DeleteAccount)))
	mux.Handle("POST /pairing-code", requireSession(http.HandlerFunc(h.GeneratePairingCode)))
	mux.Handle("GET /events", requireSession(http.HandlerFunc(h.Events)))
	mux.Handle("GET /desktops", requireSession(http.HandlerFunc(h.GetDesktops)))
	mux.Handle("DELETE /desktops/{id}", requireSession(http.HandlerFunc(h.DeleteDesktop)))

	// Pairing — no auth, pairing code is the credential; rate-limited
	mux.Handle("POST /pair", withRateLimit(pairLimiter, h.clientIP)(http.HandlerFunc(h.PairDesktop)))

	// Desktop client routes — bearer token required
	mux.Handle("GET /ws/client", requireClient(http.HandlerFunc(h.HandleWS)))
	mux.Handle("GET /payload/{id}", requireClient(http.HandlerFunc(h.GetPayload)))
	mux.Handle("POST /payload/{id}/ack", requireClient(http.HandlerFunc(h.AckPayload)))
}

func serveFileFS(name string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFileFS(w, r, web.Files, name)
	})
}
