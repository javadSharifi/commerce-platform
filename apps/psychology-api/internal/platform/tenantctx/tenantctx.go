package tenantctx

import (
	"context"

	"github.com/google/uuid"
)

type key struct{}

// With attaches a validated tenantID to the context.
// Should ONLY be called by tenant resolution middleware:
// - Domain Resolver for public site (from verified Host)
// - Membership middleware for admin panel
func With(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, key{}, id)
}

// ID returns the tenant ID from context if present and valid.
func ID(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(key{}).(uuid.UUID)
	return id, ok && id != uuid.Nil
}
