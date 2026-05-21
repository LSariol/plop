package api

import (
	"context"
	"net/http"
	"sync"
	"time"
)

type ipLimiter struct {
	mu      sync.Mutex
	entries map[string]*rlBucket
	max     int
	window  time.Duration
}

type rlBucket struct {
	count   int
	resetAt time.Time
}

// newIPLimiter creates a per-IP fixed-window rate limiter. The cleanup
// goroutine runs until ctx is cancelled, so pass the server's root context.
func newIPLimiter(ctx context.Context, max int, window time.Duration) *ipLimiter {
	l := &ipLimiter{entries: make(map[string]*rlBucket), max: max, window: window}
	go func() {
		t := time.NewTicker(5 * time.Minute)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				now := time.Now()
				l.mu.Lock()
				for ip, b := range l.entries {
					if now.After(b.resetAt) {
						delete(l.entries, ip)
					}
				}
				l.mu.Unlock()
			}
		}
	}()
	return l
}

func (l *ipLimiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	b, ok := l.entries[ip]
	now := time.Now()
	if !ok || now.After(b.resetAt) {
		l.entries[ip] = &rlBucket{count: 1, resetAt: now.Add(l.window)}
		return true
	}
	if b.count >= l.max {
		return false
	}
	b.count++
	return true
}

// withRateLimit wraps a handler with per-IP rate limiting. ipFn extracts the
// client IP from the request (use h.clientIP to respect TRUSTED_PROXY).
func withRateLimit(l *ipLimiter, ipFn func(*http.Request) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !l.allow(ipFn(r)) {
				writeError(w, "too many requests", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
