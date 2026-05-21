// Background TTL expiry goroutine

package store

import (
	"context"
	"log/slog"
	"time"
)

func (s *Store) StartCleanup(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				slog.Info("cleanup stopped")
				return
			case <-ticker.C:
				bg := context.Background()
				if err := s.DeleteExpiredPayloads(bg); err != nil {
					slog.Error("payload cleanup", "error", err)
				}
				if err := s.DeleteExpiredSessions(bg); err != nil {
					slog.Error("session cleanup", "error", err)
				}
				if err := s.DeleteExpiredPairingCodes(bg); err != nil {
					slog.Error("pairing code cleanup", "error", err)
				}
			}
		}
	}()
}
