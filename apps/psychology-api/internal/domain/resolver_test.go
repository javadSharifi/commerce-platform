package domain_test

import (
	"context"
	"testing"

	"psychology/api/internal/domain"
	"psychology/api/internal/domain/domaindb"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type mockDomainQuerier struct {
	records map[string]domaindb.Domain
}

func (m *mockDomainQuerier) GetDomainByHost(ctx context.Context, host string) (domaindb.Domain, error) {
	d, ok := m.records[host]
	if !ok {
		return domaindb.Domain{}, pgx.ErrNoRows
	}
	return d, nil
}

func toPgUUID(u uuid.UUID) pgtype.UUID {
	return pgtype.UUID{
		Bytes: u,
		Valid: true,
	}
}

func TestDomainResolver(t *testing.T) {
	tenantAID := uuid.New()
	tenantBID := uuid.New()
	tenantCID := uuid.New()

	mock := &mockDomainQuerier{
		records: map[string]domaindb.Domain{
			"clinic-a.ir": {
				Host:       "clinic-a.ir",
				TenantID:   toPgUUID(tenantAID),
				IsVerified: true,
			},
			"clinic-b.ir": {
				Host:       "clinic-b.ir",
				TenantID:   toPgUUID(tenantBID),
				IsVerified: true,
			},
			"unverified-c.ir": {
				Host:       "unverified-c.ir",
				TenantID:   toPgUUID(tenantCID),
				IsVerified: false,
			},
		},
	}

	resolver := domain.NewResolver(mock)
	ctx := context.Background()

	// Scenario 1: Host A (Verified) -> Tenant A
	resolvedA, err := resolver.Resolve(ctx, "clinic-a.ir")
	if err != nil {
		t.Fatalf("unexpected error for Host A: %v", err)
	}
	if resolvedA != tenantAID {
		t.Fatalf("expected Tenant A ID %s, got %s", tenantAID, resolvedA)
	}

	// Scenario 1b: Host A with Port and Uppercase -> Tenant A
	resolvedAPort, err := resolver.Resolve(ctx, "CLINIC-A.IR:8080")
	if err != nil {
		t.Fatalf("unexpected error for Host A with port: %v", err)
	}
	if resolvedAPort != tenantAID {
		t.Fatalf("expected Tenant A ID %s, got %s", tenantAID, resolvedAPort)
	}

	// Scenario 2: Host B (Verified) -> Tenant B
	resolvedB, err := resolver.Resolve(ctx, "clinic-b.ir")
	if err != nil {
		t.Fatalf("unexpected error for Host B: %v", err)
	}
	if resolvedB != tenantBID {
		t.Fatalf("expected Tenant B ID %s, got %s", tenantBID, resolvedB)
	}

	// Scenario 3: Host C (Unverified) -> ErrDomainNotVerified
	_, err = resolver.Resolve(ctx, "unverified-c.ir")
	if err != domain.ErrDomainNotVerified {
		t.Fatalf("expected ErrDomainNotVerified for Host C, got %v", err)
	}

	// Scenario 4: Nonexistent Host -> ErrDomainNotFound
	_, err = resolver.Resolve(ctx, "nonexistent.ir")
	if err != domain.ErrDomainNotFound {
		t.Fatalf("expected ErrDomainNotFound for nonexistent host, got %v", err)
	}
}
