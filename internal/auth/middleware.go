// Cookie extraction and route protections
package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ctxKey is an unexported type for context keys in this package.
type ctxKey string

// UsernameKey is the context key under which RequireSession stores the
// authenticated username. Handlers can read it with:
//
//	username, _ := r.Context().Value(auth.UsernameKey).(string)
const UsernameKey ctxKey = "username"

// DesktopUserKey is the context key under which RequireDesktopToken stores the
// user ID associated with the authenticated desktop token.
const DesktopUserKey ctxKey = "desktop_user"

// RequireSession is middleware that protects a route behind a valid session cookie.
// On success it stores the authenticated username in the request context under
// UsernameKey before calling the next handler.
// On failure it redirects to /login.
func RequireSession(pool *pgxpool.Pool, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("session")
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		var username string
		err = pool.QueryRow(ctx,
			`SELECT username FROM sessions
			 WHERE token = $1 AND expires_at > now()`,
			cookie.Value,
		).Scan(&username)
		if errors.Is(err, pgx.ErrNoRows) {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), UsernameKey, username)))
	})
}

// RequireDesktopToken is middleware that protects desktop routes (WS, payload download).
// It looks up the Bearer token in the desktops table and stores the owning user ID
// in the request context under DesktopUserKey.
func RequireDesktopToken(pool *pgxpool.Pool, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if token == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		var userID string
		err := pool.QueryRow(ctx,
			`SELECT user_id FROM desktops WHERE token = $1`,
			token,
		).Scan(&userID)
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), DesktopUserKey, userID)))
	})
}

