package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

func (s *Store) CreateSession(ctx context.Context, username string, ttl time.Duration) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	token := hex.EncodeToString(b)

	_, err := s.pool.Exec(ctx,
		`INSERT INTO sessions (token, username, expires_at)
         VALUES ($1, $2, now() + $3::interval)`,
		token, username, ttl.String(),
	)
	if err != nil {
		return "", fmt.Errorf("insert session: %w", err)
	}
	return token, nil
}

func (s *Store) ValidateSession(ctx context.Context, token string) (string, error) {
	var username string
	err := s.pool.QueryRow(ctx,
		`SELECT username FROM sessions
         WHERE token = $1 AND expires_at > now()`,
		token,
	).Scan(&username)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil // empty string signals invalid/expired — not an error
	}
	if err != nil {
		return "", fmt.Errorf("query session: %w", err)
	}
	return username, nil
}

func (s *Store) DeleteSession(ctx context.Context, token string) error {
	_, err := s.pool.Exec(ctx,
		`DELETE FROM sessions WHERE token = $1`,
		token,
	)
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

func (s *Store) DeleteExpiredSessions(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE expires_at < now()`)
	if err != nil {
		return fmt.Errorf("delete expired sessions: %w", err)
	}
	return nil
}
