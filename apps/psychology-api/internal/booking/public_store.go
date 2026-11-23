package booking

import (
	"context"

	"psychology/api/internal/booking/bookingdb"
	"psychology/api/internal/platform/db"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (h *Handler) WithPublicStore(store *db.Store) *Handler {
	h.publicStore = store
	return h
}

// publicBooking keeps each public operation, including any status transition, in one tenant transaction.
func publicBooking[T any](ctx context.Context, h *Handler, tenantID uuid.UUID, run func(*Service) (T, error)) (T, error) {
	return db.ReadTenant(ctx, h.publicStore, tenantID, h.service, func(tx pgx.Tx) *Service {
		return &Service{
			queries: bookingdb.New(tx), tehranLoc: h.service.tehranLoc,
			rateLimiter: h.service.rateLimiter, scoped: true,
		}
	}, run)
}
