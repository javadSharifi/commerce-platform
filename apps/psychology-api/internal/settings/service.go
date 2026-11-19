package settings

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"psychology/api/internal/settings/settingsdb"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrInvalidColor   = errors.New("settings: invalid hex color format (must be #RRGGBB)")
	ErrEmptyTitle     = errors.New("settings: title cannot be empty")
	ErrClinicNotFound = errors.New("settings: clinic not found or inactive")
)

var hexColorRegex = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

type PublicSiteProfile struct {
	Name         string  `json:"name"`
	Title        string  `json:"title"`
	Description  string  `json:"description"`
	Phone        string  `json:"phone,omitempty"`
	Address      string  `json:"address,omitempty"`
	PrimaryColor string  `json:"primary_color"`
	LogoURL      *string `json:"logo_url,omitempty"`
}

type SiteSettings struct {
	TenantID     uuid.UUID  `json:"tenant_id"`
	Title        string     `json:"title"`
	Description  string     `json:"description"`
	Phone        string     `json:"phone"`
	Address      string     `json:"address"`
	PrimaryColor string     `json:"primary_color"`
	LogoMediaID  *uuid.UUID `json:"logo_media_id,omitempty"`
}

type UpdateSettingsParams struct {
	Title        string
	Description  string
	Phone        string
	Address      string
	PrimaryColor string
	LogoMediaID  *uuid.UUID
}

type SettingsQuerier interface {
	GetSiteSettings(ctx context.Context, tenantID pgtype.UUID) (settingsdb.SiteSetting, error)
	UpsertSiteSettings(ctx context.Context, arg settingsdb.UpsertSiteSettingsParams) (settingsdb.SiteSetting, error)
	GetPublicSiteProfile(ctx context.Context, id pgtype.UUID) (settingsdb.GetPublicSiteProfileRow, error)
}

type Service struct {
	queries SettingsQuerier
}

func NewService(q SettingsQuerier) *Service {
	return &Service{queries: q}
}

func toPgUUID(u uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: u, Valid: true}
}

func fromPgUUID(p pgtype.UUID) uuid.UUID {
	var u uuid.UUID
	copy(u[:], p.Bytes[:])
	return u
}

// GetPublicProfile returns only the public, publishable profile fields for a clinic.
func (s *Service) GetPublicProfile(ctx context.Context, tenantID uuid.UUID) (*PublicSiteProfile, error) {
	row, err := s.queries.GetPublicSiteProfile(ctx, toPgUUID(tenantID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrClinicNotFound
		}
		return nil, err
	}

	primaryColor := row.PrimaryColor.String
	if !row.PrimaryColor.Valid || primaryColor == "" {
		primaryColor = "#0284c7"
	}

	title := row.Title.String
	if !row.Title.Valid || title == "" {
		title = row.ClinicName
	}

	return &PublicSiteProfile{
		Name:         row.ClinicName,
		Title:        title,
		Description:  row.Description.String,
		Phone:        row.Phone.String,
		Address:      row.Address.String,
		PrimaryColor: primaryColor,
	}, nil
}

// GetSettings returns full settings for the clinic admin panel.
func (s *Service) GetSettings(ctx context.Context, tenantID uuid.UUID) (*SiteSettings, error) {
	row, err := s.queries.GetSiteSettings(ctx, toPgUUID(tenantID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Return default settings if none saved yet
			return &SiteSettings{
				TenantID:     tenantID,
				Title:        "",
				Description:  "",
				Phone:        "",
				Address:      "",
				PrimaryColor: "#0284c7",
			}, nil
		}
		return nil, err
	}

	var logoID *uuid.UUID
	if row.LogoMediaID.Valid {
		id := fromPgUUID(row.LogoMediaID)
		logoID = &id
	}

	return &SiteSettings{
		TenantID:     fromPgUUID(row.TenantID),
		Title:        row.Title,
		Description:  row.Description,
		Phone:        row.Phone,
		Address:      row.Address,
		PrimaryColor: row.PrimaryColor,
		LogoMediaID:  logoID,
	}, nil
}

// UpdateSettings validates and updates the clinic site settings.
func (s *Service) UpdateSettings(ctx context.Context, tenantID uuid.UUID, params UpdateSettingsParams) (*SiteSettings, error) {
	title := strings.TrimSpace(params.Title)
	if title == "" {
		return nil, ErrEmptyTitle
	}

	primaryColor := strings.TrimSpace(params.PrimaryColor)
	if !hexColorRegex.MatchString(primaryColor) {
		return nil, ErrInvalidColor
	}

	var logoMediaID pgtype.UUID
	if params.LogoMediaID != nil {
		logoMediaID = toPgUUID(*params.LogoMediaID)
	}

	row, err := s.queries.UpsertSiteSettings(ctx, settingsdb.UpsertSiteSettingsParams{
		TenantID:     toPgUUID(tenantID),
		Title:        title,
		Description:  strings.TrimSpace(params.Description),
		Phone:        strings.TrimSpace(params.Phone),
		Address:      strings.TrimSpace(params.Address),
		PrimaryColor: primaryColor,
		LogoMediaID:  logoMediaID,
	})
	if err != nil {
		return nil, err
	}

	var savedLogoID *uuid.UUID
	if row.LogoMediaID.Valid {
		id := fromPgUUID(row.LogoMediaID)
		savedLogoID = &id
	}

	return &SiteSettings{
		TenantID:     fromPgUUID(row.TenantID),
		Title:        row.Title,
		Description:  row.Description,
		Phone:        row.Phone,
		Address:      row.Address,
		PrimaryColor: row.PrimaryColor,
		LogoMediaID:  savedLogoID,
	}, nil
}
