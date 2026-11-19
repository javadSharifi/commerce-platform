package db

import (
	"context"
	"errors"
	"fmt"

	"psychology/api/internal/platform/tenantctx"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNoTenant = errors.New("db: no tenant in context")

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// WithTenantTx opens a transaction, sets app.tenant_id for RLS using set_config(..., true),
// and executes fn. If fn fails, the transaction is rolled back; otherwise committed.
func (s *Store) WithTenantTx(ctx context.Context, fn func(tx pgx.Tx, tenantID uuid.UUID) error) error {
	tenantID, ok := tenantctx.ID(ctx)
	if !ok {
		return ErrNoTenant
	}
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", tenantID.String()); err != nil {
			return fmt.Errorf("set tenant: %w", err)
		}
		return fn(tx, tenantID)
	})
}

// Pool returns the underlying pgxpool.Pool.
func (s *Store) Pool() *pgxpool.Pool {
	return s.pool
}
