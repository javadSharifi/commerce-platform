package settings

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"psychology/api/internal/platform/cache"
	"psychology/api/internal/platform/tenantctx"
	"psychology/api/internal/settings/settingsdb"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrInvalidColor      = errors.New("settings: invalid hex color format (must be #RRGGBB)")
	ErrEmptyTitle        = errors.New("settings: title cannot be empty")
	ErrClinicNotFound    = errors.New("settings: clinic not found or inactive")
	ErrInvalidTemplateID = errors.New("settings: invalid template_id (must match ^[a-z0-9-]{2,40}$)")
	ErrInvalidRadius     = errors.New("settings: invalid theme_radius (must be sm, md, or lg)")
	ErrInvalidScale      = errors.New("settings: invalid theme_scale (must be sm, md, or lg)")
)

var (
	hexColorRegex   = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
	templateIDRegex = regexp.MustCompile(`^[a-z0-9-]{2,40}$`)
)

// activeTemplateVersions tracks the current active versions of supported templates.
var activeTemplateVersions = map[string]string{
	"minimal":  "1.0.0",
	"clinical": "1.0.0",
}

type PublicSiteProfile struct {
	Name            string  `json:"name"`
	Title           string  `json:"title"`
	Description     string  `json:"description"`
	Phone           string  `json:"phone,omitempty"`
	Address         string  `json:"address,omitempty"`
	PrimaryColor    string  `json:"primary_color"`
	LogoURL         *string `json:"logo_url,omitempty"`
	TemplateID      string  `json:"template_id"`
	TemplateVersion string  `json:"template_version"`
	ThemeRadius     string  `json:"theme_radius"`
	ThemeScale      string  `json:"theme_scale"`
}

type SiteSettings struct {
	TenantID        uuid.UUID  `json:"tenant_id"`
	Title           string     `json:"title"`
	Description     string     `json:"description"`
	Phone           string     `json:"phone"`
	Address         string     `json:"address"`
	PrimaryColor    string     `json:"primary_color"`
	LogoMediaID     *uuid.UUID `json:"logo_media_id,omitempty"`
	TemplateID      string     `json:"template_id"`
	TemplateVersion string     `json:"template_version"`
	ThemeRadius     string     `json:"theme_radius"`
	ThemeScale      string     `json:"theme_scale"`
}

type UpdateSettingsParams struct {
	Title        string
	Description  string
	Phone        string
	Address      string
	PrimaryColor string
	LogoMediaID  *uuid.UUID
	TemplateID   string
	ThemeRadius  string
	ThemeScale   string
}

type SettingsQuerier interface {
	GetSiteSettings(ctx context.Context, tenantID pgtype.UUID) (settingsdb.SiteSetting, error)
	UpsertSiteSettings(ctx context.Context, arg settingsdb.UpsertSiteSettingsParams) (settingsdb.SiteSetting, error)
	GetPublicSiteProfile(ctx context.Context, id pgtype.UUID) (settingsdb.GetPublicSiteProfileRow, error)
	GetPrimaryVerifiedDomain(ctx context.Context, tenantID pgtype.UUID) (string, error)
}

// TenantStore executes operations within a tenant-scoped transaction that enforces RLS.
type TenantStore interface {
	WithTenantTx(ctx context.Context, fn func(tx pgx.Tx, tenantID uuid.UUID) error) error
}

type Service struct {
	store   TenantStore
	queries SettingsQuerier
}

// NewService creates a settings Service backed by a direct SettingsQuerier (useful for mock/unit tests).
func NewService(q SettingsQuerier) *Service {
	return &Service{queries: q}
}

// NewServiceWithStore creates a settings Service that executes operations inside Store.WithTenantTx.
func NewServiceWithStore(store TenantStore) *Service {
	return &Service{store: store}
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
	if s.store != nil {
		if _, ok := tenantctx.ID(ctx); !ok {
			ctx = tenantctx.With(ctx, tenantID)
		}
		var profile *PublicSiteProfile
		err := s.store.WithTenantTx(ctx, func(tx pgx.Tx, tid uuid.UUID) error {
			row, err := settingsdb.New(tx).GetPublicSiteProfile(ctx, toPgUUID(tid))
			if err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return ErrClinicNotFound
				}
				return err
			}

			primaryColor := row.PrimaryColor.String
			if !row.PrimaryColor.Valid || primaryColor == "" {
				primaryColor = "#0284c7"
			}

			title := row.Title.String
			if !row.Title.Valid || title == "" {
				title = row.ClinicName
			}

			tplID := row.TemplateID
			if tplID == "" {
				tplID = "minimal"
			}
			tplVer := row.TemplateVersion
			if tplVer == "" {
				tplVer = "1.0.0"
			}
			radius := row.ThemeRadius
			if radius == "" {
				radius = "md"
			}
			scale := row.ThemeScale
			if scale == "" {
				scale = "md"
			}

			profile = &PublicSiteProfile{
				Name:            row.ClinicName,
				Title:           title,
				Description:     row.Description.String,
				Phone:           row.Phone.String,
				Address:         row.Address.String,
				PrimaryColor:    primaryColor,
				TemplateID:      tplID,
				TemplateVersion: tplVer,
				ThemeRadius:     radius,
				ThemeScale:      scale,
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
		return profile, nil
	}

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

	tplID := row.TemplateID
	if tplID == "" {
		tplID = "minimal"
	}
	tplVer := row.TemplateVersion
	if tplVer == "" {
		tplVer = "1.0.0"
	}
	radius := row.ThemeRadius
	if radius == "" {
		radius = "md"
	}
	scale := row.ThemeScale
	if scale == "" {
		scale = "md"
	}

	return &PublicSiteProfile{
		Name:            row.ClinicName,
		Title:           title,
		Description:     row.Description.String,
		Phone:           row.Phone.String,
		Address:         row.Address.String,
		PrimaryColor:    primaryColor,
		TemplateID:      tplID,
		TemplateVersion: tplVer,
		ThemeRadius:     radius,
		ThemeScale:      scale,
	}, nil
}

// GetSettings returns full settings for the clinic admin panel.
func (s *Service) GetSettings(ctx context.Context, tenantID uuid.UUID) (*SiteSettings, error) {
	if s.store != nil {
		if _, ok := tenantctx.ID(ctx); !ok {
			ctx = tenantctx.With(ctx, tenantID)
		}
		var siteSettings *SiteSettings
		err := s.store.WithTenantTx(ctx, func(tx pgx.Tx, tid uuid.UUID) error {
			row, err := settingsdb.New(tx).GetSiteSettings(ctx, toPgUUID(tid))
			if err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					siteSettings = &SiteSettings{
						TenantID:        tid,
						Title:           "",
						Description:     "",
						Phone:           "",
						Address:         "",
						PrimaryColor:    "#0284c7",
						TemplateID:      "minimal",
						TemplateVersion: "1.0.0",
						ThemeRadius:     "md",
						ThemeScale:      "md",
					}
					return nil
				}
				return err
			}

			var logoID *uuid.UUID
			if row.LogoMediaID.Valid {
				id := fromPgUUID(row.LogoMediaID)
				logoID = &id
			}

			tplID := row.TemplateID
			if tplID == "" {
				tplID = "minimal"
			}
			tplVer := row.TemplateVersion
			if tplVer == "" {
				tplVer = "1.0.0"
			}
			radius := row.ThemeRadius
			if radius == "" {
				radius = "md"
			}
			scale := row.ThemeScale
			if scale == "" {
				scale = "md"
			}

			siteSettings = &SiteSettings{
				TenantID:        fromPgUUID(row.TenantID),
				Title:           row.Title,
				Description:     row.Description,
				Phone:           row.Phone,
				Address:         row.Address,
				PrimaryColor:    row.PrimaryColor,
				LogoMediaID:     logoID,
				TemplateID:      tplID,
				TemplateVersion: tplVer,
				ThemeRadius:     radius,
				ThemeScale:      scale,
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
		return siteSettings, nil
	}

	row, err := s.queries.GetSiteSettings(ctx, toPgUUID(tenantID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Return default settings if none saved yet
			return &SiteSettings{
				TenantID:        tenantID,
				Title:           "",
				Description:     "",
				Phone:           "",
				Address:         "",
				PrimaryColor:    "#0284c7",
				TemplateID:      "minimal",
				TemplateVersion: "1.0.0",
				ThemeRadius:     "md",
				ThemeScale:      "md",
			}, nil
		}
		return nil, err
	}

	var logoID *uuid.UUID
	if row.LogoMediaID.Valid {
		id := fromPgUUID(row.LogoMediaID)
		logoID = &id
	}

	tplID := row.TemplateID
	if tplID == "" {
		tplID = "minimal"
	}
	tplVer := row.TemplateVersion
	if tplVer == "" {
		tplVer = "1.0.0"
	}
	radius := row.ThemeRadius
	if radius == "" {
		radius = "md"
	}
	scale := row.ThemeScale
	if scale == "" {
		scale = "md"
	}

	return &SiteSettings{
		TenantID:        fromPgUUID(row.TenantID),
		Title:           row.Title,
		Description:     row.Description,
		Phone:           row.Phone,
		Address:         row.Address,
		PrimaryColor:    row.PrimaryColor,
		LogoMediaID:     logoID,
		TemplateID:      tplID,
		TemplateVersion: tplVer,
		ThemeRadius:     radius,
		ThemeScale:      scale,
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

	templateID := strings.TrimSpace(params.TemplateID)
	if templateID == "" {
		templateID = "minimal"
	} else if !templateIDRegex.MatchString(templateID) {
		return nil, ErrInvalidTemplateID
	}

	templateVersion, ok := activeTemplateVersions[templateID]
	if !ok {
		templateVersion = "1.0.0"
	}

	themeRadius := strings.TrimSpace(params.ThemeRadius)
	if themeRadius == "" {
		themeRadius = "md"
	} else if themeRadius != "sm" && themeRadius != "md" && themeRadius != "lg" {
		return nil, ErrInvalidRadius
	}

	themeScale := strings.TrimSpace(params.ThemeScale)
	if themeScale == "" {
		themeScale = "md"
	} else if themeScale != "sm" && themeScale != "md" && themeScale != "lg" {
		return nil, ErrInvalidScale
	}

	if s.store != nil {
		if _, ok := tenantctx.ID(ctx); !ok {
			ctx = tenantctx.With(ctx, tenantID)
		}
		var updated *SiteSettings
		err := s.store.WithTenantTx(ctx, func(tx pgx.Tx, tid uuid.UUID) error {
			var logoMediaID pgtype.UUID
			if params.LogoMediaID != nil {
				logoMediaID = toPgUUID(*params.LogoMediaID)
			}

			q := settingsdb.New(tx)
			row, err := q.UpsertSiteSettings(ctx, settingsdb.UpsertSiteSettingsParams{
				TenantID:        toPgUUID(tid),
				Title:           title,
				Description:     strings.TrimSpace(params.Description),
				Phone:           strings.TrimSpace(params.Phone),
				Address:         strings.TrimSpace(params.Address),
				PrimaryColor:    primaryColor,
				LogoMediaID:     logoMediaID,
				TemplateID:      templateID,
				TemplateVersion: templateVersion,
				ThemeRadius:     themeRadius,
				ThemeScale:      themeScale,
			})
			if err != nil {
				return err
			}

			// Record Outbox cache invalidation event within the SAME transaction (AP-11 / PSY-5.3)
			host, err := q.GetPrimaryVerifiedDomain(ctx, toPgUUID(tid))
			if err == nil && host != "" {
				_ = cache.InsertCacheInvalidationEvent(ctx, tx, tid, "site.updated", host, []string{"/", "/preview"})
			}

			var savedLogoID *uuid.UUID
			if row.LogoMediaID.Valid {
				id := fromPgUUID(row.LogoMediaID)
				savedLogoID = &id
			}

			updated = &SiteSettings{
				TenantID:        fromPgUUID(row.TenantID),
				Title:           row.Title,
				Description:     row.Description,
				Phone:           row.Phone,
				Address:         row.Address,
				PrimaryColor:    row.PrimaryColor,
				LogoMediaID:     savedLogoID,
				TemplateID:      row.TemplateID,
				TemplateVersion: row.TemplateVersion,
				ThemeRadius:     row.ThemeRadius,
				ThemeScale:      row.ThemeScale,
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
		return updated, nil
	}

	var logoMediaID pgtype.UUID
	if params.LogoMediaID != nil {
		logoMediaID = toPgUUID(*params.LogoMediaID)
	}

	row, err := s.queries.UpsertSiteSettings(ctx, settingsdb.UpsertSiteSettingsParams{
		TenantID:        toPgUUID(tenantID),
		Title:           title,
		Description:     strings.TrimSpace(params.Description),
		Phone:           strings.TrimSpace(params.Phone),
		Address:         strings.TrimSpace(params.Address),
		PrimaryColor:    primaryColor,
		LogoMediaID:     logoMediaID,
		TemplateID:      templateID,
		TemplateVersion: templateVersion,
		ThemeRadius:     themeRadius,
		ThemeScale:      themeScale,
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
		TenantID:        fromPgUUID(row.TenantID),
		Title:           row.Title,
		Description:     row.Description,
		Phone:           row.Phone,
		Address:         row.Address,
		PrimaryColor:    row.PrimaryColor,
		LogoMediaID:     savedLogoID,
		TemplateID:      row.TemplateID,
		TemplateVersion: row.TemplateVersion,
		ThemeRadius:     row.ThemeRadius,
		ThemeScale:      row.ThemeScale,
	}, nil
}
