package domain

import (
	"context"
	"errors"
	"net"
	"strings"

	"psychology/api/internal/domain/domaindb"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	ErrDomainNotFound    = errors.New("domain: host not found")
	ErrDomainNotVerified = errors.New("domain: host is not verified")
	ErrInvalidHost       = errors.New("domain: invalid host format")
)

type DomainQuerier interface {
	GetDomainByHost(ctx context.Context, host string) (domaindb.Domain, error)
}

type Resolver struct {
	queries DomainQuerier
}

func NewResolver(q DomainQuerier) *Resolver {
	return &Resolver{queries: q}
}

// CleanHost normalizes raw host by stripping port numbers, trimming whitespace and lowercasing.
func CleanHost(rawHost string) (string, error) {
	h := strings.TrimSpace(rawHost)
	if h == "" {
		return "", ErrInvalidHost
	}

	if host, _, err := net.SplitHostPort(h); err == nil {
		h = host
	}

	h = strings.ToLower(strings.Trim(h, "."))
	if h == "" {
		return "", ErrInvalidHost
	}

	return h, nil
}

// Resolve resolves a host to its corresponding verified tenantID.
func (r *Resolver) Resolve(ctx context.Context, rawHost string) (uuid.UUID, error) {
	host, err := CleanHost(rawHost)
	if err != nil {
		return uuid.Nil, err
	}

	domain, err := r.queries.GetDomainByHost(ctx, host)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, ErrDomainNotFound
		}
		return uuid.Nil, err
	}

	if !domain.IsVerified {
		return uuid.Nil, ErrDomainNotVerified
	}

	tenantUUID, err := uuid.FromBytes(domain.TenantID.Bytes[:])
	if err != nil {
		return uuid.Nil, err
	}

	return tenantUUID, nil
}
