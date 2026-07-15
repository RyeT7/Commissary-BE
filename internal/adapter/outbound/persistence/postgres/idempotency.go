package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"commissary/internal/application/port"
)

type IdempotencyStore struct {
	pool *pgxpool.Pool
}

func NewIdempotencyStore(pool *pgxpool.Pool) *IdempotencyStore { return &IdempotencyStore{pool: pool} }

var _ port.IdempotencyStore = (*IdempotencyStore)(nil)

func (s *IdempotencyStore) Begin(ctx context.Context, key, scope string) (port.IdempotencyRecord, bool, error) {
	var inserted string
	err := s.pool.QueryRow(ctx,
		`INSERT INTO idempotency_keys (key, scope) VALUES ($1, $2)
		 ON CONFLICT (key) DO NOTHING RETURNING key`, key, scope).Scan(&inserted)
	if err == nil {
		return port.IdempotencyRecord{}, false, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return port.IdempotencyRecord{}, false, err
	}

	var rec port.IdempotencyRecord
	err = s.pool.QueryRow(ctx,
		`SELECT scope, status_code, response_body, completed FROM idempotency_keys WHERE key = $1`, key).
		Scan(&rec.Scope, &rec.Status, &rec.Body, &rec.Completed)
	if err != nil {
		return port.IdempotencyRecord{}, false, err
	}
	return rec, true, nil
}

func (s *IdempotencyStore) Complete(ctx context.Context, key string, status int, body []byte) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE idempotency_keys SET status_code = $2, response_body = $3, completed = true WHERE key = $1`,
		key, status, body)
	return err
}

func (s *IdempotencyStore) Discard(ctx context.Context, key string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM idempotency_keys WHERE key = $1 AND completed = false`, key)
	return err
}

func (s *IdempotencyStore) DeleteExpired(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM idempotency_keys WHERE created_at < now() - interval '24 hours'`)
	return err
}
