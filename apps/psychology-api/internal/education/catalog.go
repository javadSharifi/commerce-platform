// Package education holds behavior shared by the course and workshop catalogs.
package education

import (
	"context"
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"

	"psychology/api/internal/domain/domaindb"
	"psychology/api/internal/platform/cache"
	"psychology/api/internal/platform/db"
	"psychology/api/internal/platform/tenantctx"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var ErrInvalidTopic = errors.New("موضوع آموزش باید حداکثر ۸۰ نویسه و بدون کاراکتر کنترلی باشد.")

func NormalizeTopic(topic string) (string, error) {
	topic = strings.Join(strings.Fields(topic), " ")
	if !utf8.ValidString(topic) || utf8.RuneCountInString(topic) > 80 || strings.ContainsFunc(topic, unicode.IsControl) {
		return "", ErrInvalidTopic
	}
	return topic, nil
}

// WriteCatalog binds mutations to the clinic selected by the authenticated admin middleware.
func WriteCatalog[T any](ctx context.Context, store *db.Store, tenantID uuid.UUID, write func(pgx.Tx) (T, error)) (T, error) {
	var result T
	id, ok := tenantctx.ID(ctx)
	if !ok || id != tenantID {
		return result, db.ErrNoTenant
	}
	err := store.WithTenantTx(ctx, func(tx pgx.Tx, _ uuid.UUID) error {
		var err error
		result, err = write(tx)
		return err
	})
	return result, err
}

// InvalidateCatalog records changes to publicly visible catalogs for each verified host.
func InvalidateCatalog(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID, paths ...string) error {
	domains, err := domaindb.New(tx).ListDomainsByTenant(ctx, pgtype.UUID{Bytes: tenantID, Valid: true})
	if err != nil {
		return err
	}
	for _, domain := range domains {
		if domain.IsVerified {
			if err := cache.InsertCacheInvalidationEvent(ctx, tx, tenantID, "education.updated", domain.Host, paths); err != nil {
				return err
			}
		}
	}
	return nil
}
