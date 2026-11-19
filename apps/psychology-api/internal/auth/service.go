package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"psychology/api/internal/auth/authdb"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrInvalidCredentials = errors.New("auth: invalid email or password")
	ErrUserInactive       = errors.New("auth: user account is inactive")
	ErrSessionExpired     = errors.New("auth: session expired or not found")
	ErrNotAMember         = errors.New("auth: user is not a member of this tenant")
	ErrForbiddenRole      = errors.New("auth: insufficient permissions for this role")
)

type PanelUser struct {
	ID       uuid.UUID `json:"id"`
	Email    string    `json:"email"`
	FullName string    `json:"full_name"`
	Status   string    `json:"status"`
}

type Membership struct {
	ID         uuid.UUID `json:"id"`
	TenantID   uuid.UUID `json:"tenant_id"`
	TenantName string    `json:"tenant_name"`
	TenantSlug string    `json:"tenant_slug"`
	UserID     uuid.UUID `json:"user_id"`
	Role       string    `json:"role"`
}

type SessionInfo struct {
	Token     string
	TokenHash string
	ExpiresAt time.Time
	User      PanelUser
}

type AuthQuerier interface {
	GetPanelIdentityByEmail(ctx context.Context, email string) (authdb.PanelIdentity, error)
	GetPanelIdentityByID(ctx context.Context, id pgtype.UUID) (authdb.PanelIdentity, error)
	CreatePanelIdentity(ctx context.Context, arg authdb.CreatePanelIdentityParams) (authdb.PanelIdentity, error)
	CreatePanelSession(ctx context.Context, arg authdb.CreatePanelSessionParams) (authdb.PanelSession, error)
	GetPanelSessionByTokenHash(ctx context.Context, tokenHash string) (authdb.PanelSession, error)
	DeletePanelSession(ctx context.Context, tokenHash string) error
	ListMembershipsByUserID(ctx context.Context, userID pgtype.UUID) ([]authdb.ListMembershipsByUserIDRow, error)
	GetMembershipByUserAndTenant(ctx context.Context, arg authdb.GetMembershipByUserAndTenantParams) (authdb.Membership, error)
	CreateMembership(ctx context.Context, arg authdb.CreateMembershipParams) (authdb.Membership, error)
}

type Service struct {
	queries AuthQuerier
}

func NewService(q AuthQuerier) *Service {
	return &Service{queries: q}
}

func HashToken(rawToken string) string {
	sum := sha256.Sum256([]byte(rawToken))
	return hex.EncodeToString(sum[:])
}

func GenerateSecureToken(byteLen int) (string, error) {
	b := make([]byte, byteLen)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func toPgUUID(u uuid.UUID) pgtype.UUID {
	return pgtype.UUID{
		Bytes: u,
		Valid: true,
	}
}

func fromPgUUID(p pgtype.UUID) uuid.UUID {
	var u uuid.UUID
	copy(u[:], p.Bytes[:])
	return u
}

// Login authenticates a user and creates a new server-side session.
func (s *Service) Login(ctx context.Context, email, password string) (*SessionInfo, error) {
	identity, err := s.queries.GetPanelIdentityByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}

	if identity.Status != "active" {
		return nil, ErrUserInactive
	}

	match, err := CheckPassword(password, identity.PasswordHash)
	if err != nil || !match {
		return nil, ErrInvalidCredentials
	}

	token, err := GenerateSecureToken(32)
	if err != nil {
		return nil, err
	}
	tokenHash := HashToken(token)

	sessionID := uuid.New()
	expiresAt := time.Now().UTC().Add(7 * 24 * time.Hour)

	_, err = s.queries.CreatePanelSession(ctx, authdb.CreatePanelSessionParams{
		ID:        toPgUUID(sessionID),
		UserID:    identity.ID,
		TokenHash: tokenHash,
		ExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
	})
	if err != nil {
		return nil, err
	}

	return &SessionInfo{
		Token:     token,
		TokenHash: tokenHash,
		ExpiresAt: expiresAt,
		User: PanelUser{
			ID:       fromPgUUID(identity.ID),
			Email:    identity.Email,
			FullName: identity.FullName,
			Status:   identity.Status,
		},
	}, nil
}

// Logout terminates a panel session by deleting it.
func (s *Service) Logout(ctx context.Context, token string) error {
	tokenHash := HashToken(token)
	return s.queries.DeletePanelSession(ctx, tokenHash)
}

// ValidateSession verifies a session token and returns the authenticated user.
func (s *Service) ValidateSession(ctx context.Context, token string) (*PanelUser, error) {
	tokenHash := HashToken(token)
	session, err := s.queries.GetPanelSessionByTokenHash(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrSessionExpired
		}
		return nil, err
	}

	identity, err := s.queries.GetPanelIdentityByID(ctx, session.UserID)
	if err != nil {
		return nil, err
	}

	if identity.Status != "active" {
		return nil, ErrUserInactive
	}

	return &PanelUser{
		ID:       fromPgUUID(identity.ID),
		Email:    identity.Email,
		FullName: identity.FullName,
		Status:   identity.Status,
	}, nil
}

// ListMemberships returns all tenant memberships for a panel user.
func (s *Service) ListMemberships(ctx context.Context, userID uuid.UUID) ([]Membership, error) {
	rows, err := s.queries.ListMembershipsByUserID(ctx, toPgUUID(userID))
	if err != nil {
		return nil, err
	}

	memberships := make([]Membership, 0, len(rows))
	for _, r := range rows {
		memberships = append(memberships, Membership{
			ID:         fromPgUUID(r.ID),
			TenantID:   fromPgUUID(r.TenantID),
			TenantName: r.TenantName,
			TenantSlug: r.TenantSlug,
			UserID:     fromPgUUID(r.UserID),
			Role:       r.Role,
		})
	}
	return memberships, nil
}

// GetMembership verifies that the user is a member of the tenant and returns their role.
func (s *Service) GetMembership(ctx context.Context, userID, tenantID uuid.UUID) (*Membership, error) {
	m, err := s.queries.GetMembershipByUserAndTenant(ctx, authdb.GetMembershipByUserAndTenantParams{
		UserID:   toPgUUID(userID),
		TenantID: toPgUUID(tenantID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotAMember
		}
		return nil, err
	}

	return &Membership{
		ID:       fromPgUUID(m.ID),
		TenantID: fromPgUUID(m.TenantID),
		UserID:   fromPgUUID(m.UserID),
		Role:     m.Role,
	}, nil
}
