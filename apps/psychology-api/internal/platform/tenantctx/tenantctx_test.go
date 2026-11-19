package tenantctx_test

import (
	"context"
	"testing"

	"psychology/api/internal/platform/tenantctx"

	"github.com/google/uuid"
)

func TestTenantCtx(t *testing.T) {
	ctx := context.Background()

	// 1. Without tenant
	if id, ok := tenantctx.ID(ctx); ok || id != uuid.Nil {
		t.Fatalf("expected no tenant in empty context, got id=%s, ok=%v", id, ok)
	}

	// 2. With nil uuid
	ctxWithNil := tenantctx.With(ctx, uuid.Nil)
	if _, ok := tenantctx.ID(ctxWithNil); ok {
		t.Fatalf("expected ok=false for uuid.Nil")
	}

	// 3. With valid tenant ID
	expectedID := uuid.New()
	ctxWithTenant := tenantctx.With(ctx, expectedID)
	id, ok := tenantctx.ID(ctxWithTenant)
	if !ok {
		t.Fatalf("expected ok=true for valid tenant ID")
	}
	if id != expectedID {
		t.Fatalf("expected %s, got %s", expectedID, id)
	}
}
