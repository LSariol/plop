package store

import (
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("not found")

type Store struct {
	pool       *pgxpool.Pool
	payloadDir string
}

func New(pool *pgxpool.Pool, payloadDir string) *Store {
	return &Store{
		pool:       pool,
		payloadDir: payloadDir,
	}
}
