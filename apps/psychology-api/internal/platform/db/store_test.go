package db_test

import (
	"context"
	"testing"

	"psychology/api/internal/platform/db"
	"psychology/api/internal/platform/tenantctx"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestWithTenantTxWithoutTenant(t *testing.T) {
	store := db.NewStore(nil)
	ctx := context.Background()

	err := store.WithTenantTx(ctx, func(tx pgx.Tx, tenantID uuid.UUID) error {
		return nil
	})

	if err != db.ErrNoTenant {
		t.Fatalf("expected ErrNoTenant, got %v", err)
	}
}

func TestWithTenantTxContextDetection(t *testing.T) {
	tenantID := uuid.New()
	ctx := tenantctx.With(context.Background(), tenantID)

	id, ok := tenantctx.ID(ctx)
	if !ok || id != tenantID {
		t.Fatalf("expected to detect tenantID %s in context, got %s (ok=%v)", tenantID, id, ok)
	}
}
