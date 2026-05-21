package api

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/lsariol/plop/internal/auth"
	"github.com/lsariol/plop/internal/config"
	"github.com/lsariol/plop/internal/notify"
	"github.com/lsariol/plop/internal/sse"
	"github.com/lsariol/plop/internal/store"
)

type Handler struct {
	pool       *pgxpool.Pool
	hub        *notify.Hub
	sseHub     *sse.Hub
	store      *store.Store
	cfg        config.Config
	webVersion string // hash of embedded web assets; injected into sw.js CACHE_NAME
}

func New(pool *pgxpool.Pool, hub *notify.Hub, sseHub *sse.Hub, store *store.Store, cfg config.Config) *Handler {
	return &Handler{
		pool:       pool,
		hub:        hub,
		sseHub:     sseHub,
		store:      store,
		cfg:        cfg,
		webVersion: computeWebVersion(),
	}
}

func writeError(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

// usernameFromCtx returns the authenticated username stored by RequireSession middleware.
func usernameFromCtx(r *http.Request) string {
	u, _ := r.Context().Value(auth.UsernameKey).(string)
	return u
}

// desktopUserFromCtx returns the user ID stored by RequireDesktopToken middleware.
func desktopUserFromCtx(r *http.Request) string {
	u, _ := r.Context().Value(auth.DesktopUserKey).(string)
	return u
}

// clientIP returns the best-guess client IP. When TRUSTED_PROXY=true it reads
// X-Real-IP first, then the leftmost entry of X-Forwarded-For. Both headers
// must only be set by a trusted proxy — never enable this without one.
func (h *Handler) clientIP(r *http.Request) string {
	if h.cfg.TrustedProxy {
		if ip := r.Header.Get("X-Real-IP"); ip != "" {
			return strings.TrimSpace(ip)
		}
		if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
			return strings.TrimSpace(strings.SplitN(fwd, ",", 2)[0])
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
