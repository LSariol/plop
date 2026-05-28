package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"
)

// ErrUserExists is returned by CreateUser when the username is already taken.
var ErrUserExists = errors.New("username already taken")

func (s *Store) CreateUser(ctx context.Context, username, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	_, err = s.pool.Exec(ctx,
		`INSERT INTO users (username, password_hash) VALUES ($1, $2)`,
		username, string(hash),
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrUserExists
		}
		return fmt.Errorf("insert user: %w", err)
	}
	return nil
}

func (s *Store) UpdatePassword(ctx context.Context, username, newHash string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE users SET password_hash = $1 WHERE username = $2`,
		newHash, username,
	)
	if err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	return nil
}

func (s *Store) DeleteUser(ctx context.Context, username string) error {
	_, err := s.pool.Exec(ctx,
		`DELETE FROM users WHERE username = $1`,
		username,
	)
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	return nil
}

func (s *Store) Authenticate(ctx context.Context, username, password string) (bool, error) {
	var hash string
	err := s.pool.QueryRow(ctx,
		`SELECT password_hash FROM users WHERE username = $1`,
		username,
	).Scan(&hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("query user: %w", err)
	}
	err = bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil, nil
}
