package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *Store) CreatePairingCode(ctx context.Context, userID string) (string, error) {
	code := generateCode(8)
	expiresAt := time.Now().Add(15 * time.Minute)
	_, err := s.pool.Exec(ctx,
		`INSERT INTO pairing_codes (code, user_id, expires_at)
         VALUES ($1, $2, $3)
         ON CONFLICT (code) DO UPDATE SET user_id = $2, expires_at = $3`,
		code, userID, expiresAt,
	)
	if err != nil {
		return "", fmt.Errorf("insert pairing code: %w", err)
	}
	return code, nil
}

func (s *Store) ConsumePairingCode(ctx context.Context, code string) (string, error) {
	var userID string
	err := s.pool.QueryRow(ctx,
		`DELETE FROM pairing_codes
         WHERE code = $1 AND expires_at > now()
         RETURNING user_id`,
		strings.ReplaceAll(strings.ToUpper(strings.TrimSpace(code)), " ", ""),
	).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("consume pairing code: %w", err)
	}
	return userID, nil
}

func (s *Store) CreateDesktop(ctx context.Context, userID, name string) (string, error) {
	id := uuid.New().String()
	token := generateToken(32)
	_, err := s.pool.Exec(ctx,
		`INSERT INTO desktops (id, user_id, name, token) VALUES ($1, $2, $3, $4)`,
		id, userID, name, token,
	)
	if err != nil {
		return "", fmt.Errorf("insert desktop: %w", err)
	}
	return token, nil
}

func (s *Store) GetDesktopUser(ctx context.Context, token string) (string, error) {
	var userID string
	err := s.pool.QueryRow(ctx,
		`SELECT user_id FROM desktops WHERE token = $1`,
		token,
	).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("get desktop user: %w", err)
	}
	return userID, nil
}

type DesktopInfo struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *Store) GetUserDesktops(ctx context.Context, userID string) ([]DesktopInfo, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, name, created_at FROM desktops
         WHERE user_id = $1 ORDER BY created_at ASC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("query desktops: %w", err)
	}
	defer rows.Close()

	var list []DesktopInfo
	for rows.Next() {
		var d DesktopInfo
		if err := rows.Scan(&d.ID, &d.Name, &d.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan desktop: %w", err)
		}
		list = append(list, d)
	}
	return list, rows.Err()
}

func (s *Store) DeleteDesktop(ctx context.Context, desktopID, userID string) error {
	tag, err := s.pool.Exec(ctx,
		`DELETE FROM desktops WHERE id = $1 AND user_id = $2`,
		desktopID, userID,
	)
	if err != nil {
		return fmt.Errorf("delete desktop: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeleteExpiredPairingCodes(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM pairing_codes WHERE expires_at < now()`)
	if err != nil {
		return fmt.Errorf("delete expired pairing codes: %w", err)
	}
	return nil
}

// codeChars excludes visually ambiguous characters (0/O, 1/I/L).
const codeChars = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

func generateCode(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand unavailable: " + err.Error())
	}
	for i := range b {
		b[i] = codeChars[int(b[i])%len(codeChars)]
	}
	return string(b)
}

func generateToken(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand unavailable: " + err.Error())
	}
	return hex.EncodeToString(b)
}
