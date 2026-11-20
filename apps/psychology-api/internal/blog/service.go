package blog

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"

	"psychology/api/internal/blog/blogdb"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrPostNotFound         = errors.New("blog: post not found")
	ErrDuplicateSlug        = errors.New("blog: slug already exists for this clinic")
	ErrInvalidSlug          = errors.New("blog: invalid slug format (alphanumeric and dashes only)")
	ErrInvalidTitle         = errors.New("blog: title is required")
	ErrInvalidContentBlock  = errors.New("blog: invalid content block structure or type")
	ErrScriptInjection      = errors.New("blog: raw html or script execution tags are forbidden")
	ErrInvalidStatus        = errors.New("blog: invalid status")
)

var validSlugRegex = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

type ContentBlockType string

const (
	BlockParagraph ContentBlockType = "paragraph"
	BlockHeading   ContentBlockType = "heading"
	BlockImage     ContentBlockType = "image"
	BlockVideo     ContentBlockType = "video"
	BlockAudio     ContentBlockType = "audio"
)

type ContentBlock struct {
	Type ContentBlockType `json:"type"`
	Data map[string]any   `json:"data"`
}

type BlogPost struct {
	ID             uuid.UUID      `json:"id"`
	TenantID       uuid.UUID      `json:"tenant_id"`
	Title          string         `json:"title"`
	Slug           string         `json:"slug"`
	SeoDescription string         `json:"seo_description"`
	ContentBlocks  []ContentBlock `json:"content_blocks"`
	Status         string         `json:"status"`
	PublishedAt    *time.Time     `json:"published_at,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

type PublicBlogPostItem struct {
	ID             uuid.UUID  `json:"id"`
	Title          string     `json:"title"`
	Slug           string     `json:"slug"`
	SeoDescription string     `json:"seo_description"`
	PublishedAt    *time.Time `json:"published_at,omitempty"`
}

type Querier interface {
	CreateBlogPost(ctx context.Context, arg blogdb.CreateBlogPostParams) (blogdb.BlogPost, error)
	GetBlogPostByID(ctx context.Context, arg blogdb.GetBlogPostByIDParams) (blogdb.BlogPost, error)
	GetPublishedBlogPostBySlug(ctx context.Context, arg blogdb.GetPublishedBlogPostBySlugParams) (blogdb.BlogPost, error)
	ListPublishedBlogPosts(ctx context.Context, tenantID pgtype.UUID) ([]blogdb.ListPublishedBlogPostsRow, error)
	ListAdminBlogPosts(ctx context.Context, arg blogdb.ListAdminBlogPostsParams) ([]blogdb.BlogPost, error)
	UpdateBlogPost(ctx context.Context, arg blogdb.UpdateBlogPostParams) (blogdb.BlogPost, error)
	PublishBlogPost(ctx context.Context, arg blogdb.PublishBlogPostParams) (blogdb.BlogPost, error)
	UnpublishBlogPost(ctx context.Context, arg blogdb.UnpublishBlogPostParams) (blogdb.BlogPost, error)
	DeleteBlogPost(ctx context.Context, arg blogdb.DeleteBlogPostParams) error
}

type Service struct {
	queries Querier
}

func NewService(q Querier) *Service {
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

// ValidateSlug ensures the slug conforms to safe URL format.
func ValidateSlug(slug string) error {
	slug = strings.TrimSpace(slug)
	if slug == "" || !validSlugRegex.MatchString(slug) {
		return ErrInvalidSlug
	}
	return nil
}

// ValidateContentBlocks checks structure and prevents raw HTML / script tag injection.
func ValidateContentBlocks(blocks []ContentBlock) error {
	for _, b := range blocks {
		switch b.Type {
		case BlockParagraph, BlockHeading, BlockImage, BlockVideo, BlockAudio:
			// valid type
		default:
			return ErrInvalidContentBlock
		}

		if b.Data == nil {
			return ErrInvalidContentBlock
		}

		// Security scan against script injection / raw HTML execution
		for k, v := range b.Data {
			strVal, ok := v.(string)
			if !ok {
				continue
			}
			lower := strings.ToLower(strVal)
			if strings.Contains(lower, "<script") ||
				strings.Contains(lower, "</script>") ||
				strings.Contains(lower, "javascript:") ||
				strings.Contains(lower, "<iframe") ||
				strings.Contains(lower, "<object") ||
				strings.Contains(lower, "<embed") ||
				strings.Contains(lower, "onload=") ||
				strings.Contains(lower, "onerror=") {
				return ErrScriptInjection
			}

			// Validate heading level
			if b.Type == BlockHeading && k == "level" {
				// level can be float64 from json
			}
		}

		// Type-specific field checks
		if b.Type == BlockParagraph {
			text, ok := b.Data["text"].(string)
			if !ok || strings.TrimSpace(text) == "" {
				return ErrInvalidContentBlock
			}
		}
		if b.Type == BlockHeading {
			text, ok := b.Data["text"].(string)
			if !ok || strings.TrimSpace(text) == "" {
				return ErrInvalidContentBlock
			}
			levelVal, exists := b.Data["level"]
			if exists {
				var level int
				switch l := levelVal.(type) {
				case float64:
					level = int(l)
				case int:
					level = l
				default:
					return ErrInvalidContentBlock
				}
				if level < 1 || level > 6 {
					return ErrInvalidContentBlock
				}
			}
		}
	}
	return nil
}

func mapDBToBlogPost(row blogdb.BlogPost) (*BlogPost, error) {
	var blocks []ContentBlock
	if len(row.ContentBlocks) > 0 {
		if err := json.Unmarshal(row.ContentBlocks, &blocks); err != nil {
			return nil, err
		}
	} else {
		blocks = make([]ContentBlock, 0)
	}

	var pubAt *time.Time
	if row.PublishedAt.Valid {
		t := row.PublishedAt.Time
		pubAt = &t
	}

	return &BlogPost{
		ID:             fromPgUUID(row.ID),
		TenantID:       fromPgUUID(row.TenantID),
		Title:          row.Title,
		Slug:           row.Slug,
		SeoDescription: row.SeoDescription,
		ContentBlocks:  blocks,
		Status:         row.Status,
		PublishedAt:    pubAt,
		CreatedAt:      row.CreatedAt.Time,
		UpdatedAt:      row.UpdatedAt.Time,
	}, nil
}

// ---------------- Public Methods ----------------

// ListPublishedPosts lists published blog posts for a tenant (Drafts are strictly excluded).
func (s *Service) ListPublishedPosts(ctx context.Context, tenantID uuid.UUID) ([]PublicBlogPostItem, error) {
	if tenantID == uuid.Nil {
		return nil, errors.New("blog: tenant_id cannot be nil")
	}

	rows, err := s.queries.ListPublishedBlogPosts(ctx, toPgUUID(tenantID))
	if err != nil {
		return nil, err
	}

	res := make([]PublicBlogPostItem, 0, len(rows))
	for _, r := range rows {
		var pubAt *time.Time
		if r.PublishedAt.Valid {
			t := r.PublishedAt.Time
			pubAt = &t
		}
		res = append(res, PublicBlogPostItem{
			ID:             fromPgUUID(r.ID),
			Title:          r.Title,
			Slug:           r.Slug,
			SeoDescription: r.SeoDescription,
			PublishedAt:    pubAt,
		})
	}
	return res, nil
}

// GetPublishedPostBySlug returns a published post. If the post is a draft or not found, ErrPostNotFound is returned.
func (s *Service) GetPublishedPostBySlug(ctx context.Context, tenantID uuid.UUID, slug string) (*BlogPost, error) {
	if tenantID == uuid.Nil {
		return nil, errors.New("blog: tenant_id cannot be nil")
	}
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return nil, ErrPostNotFound
	}

	row, err := s.queries.GetPublishedBlogPostBySlug(ctx, blogdb.GetPublishedBlogPostBySlugParams{
		TenantID: toPgUUID(tenantID),
		Slug:     slug,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrPostNotFound
		}
		return nil, err
	}

	return mapDBToBlogPost(row)
}

// ---------------- Admin Methods ----------------

type CreatePostParams struct {
	TenantID       uuid.UUID
	Title          string
	Slug           string
	SeoDescription string
	ContentBlocks  []ContentBlock
	Status         string // draft or published
}

func (s *Service) CreatePost(ctx context.Context, p CreatePostParams) (*BlogPost, error) {
	if p.TenantID == uuid.Nil {
		return nil, errors.New("blog: tenant_id cannot be nil")
	}
	p.Title = strings.TrimSpace(p.Title)
	if p.Title == "" {
		return nil, ErrInvalidTitle
	}

	p.Slug = strings.TrimSpace(strings.ToLower(p.Slug))
	if err := ValidateSlug(p.Slug); err != nil {
		return nil, err
	}

	if err := ValidateContentBlocks(p.ContentBlocks); err != nil {
		return nil, err
	}

	status := strings.TrimSpace(p.Status)
	if status == "" {
		status = "draft"
	}
	if status != "draft" && status != "published" {
		return nil, ErrInvalidStatus
	}

	var pubAt pgtype.Timestamptz
	if status == "published" {
		pubAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	}

	rawBlocks, err := json.Marshal(p.ContentBlocks)
	if err != nil {
		return nil, err
	}

	row, err := s.queries.CreateBlogPost(ctx, blogdb.CreateBlogPostParams{
		TenantID:       toPgUUID(p.TenantID),
		Title:          p.Title,
		Slug:           p.Slug,
		SeoDescription: p.SeoDescription,
		ContentBlocks:  rawBlocks,
		Status:         status,
		PublishedAt:    pubAt,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation
			return nil, ErrDuplicateSlug
		}
		return nil, err
	}

	return mapDBToBlogPost(row)
}

type UpdatePostParams struct {
	TenantID       uuid.UUID
	ID             uuid.UUID
	Title          string
	Slug           string
	SeoDescription string
	ContentBlocks  []ContentBlock
}

func (s *Service) UpdatePost(ctx context.Context, p UpdatePostParams) (*BlogPost, error) {
	if p.TenantID == uuid.Nil || p.ID == uuid.Nil {
		return nil, errors.New("blog: tenant_id and post id cannot be nil")
	}
	p.Title = strings.TrimSpace(p.Title)
	if p.Title == "" {
		return nil, ErrInvalidTitle
	}

	p.Slug = strings.TrimSpace(strings.ToLower(p.Slug))
	if err := ValidateSlug(p.Slug); err != nil {
		return nil, err
	}

	if err := ValidateContentBlocks(p.ContentBlocks); err != nil {
		return nil, err
	}

	rawBlocks, err := json.Marshal(p.ContentBlocks)
	if err != nil {
		return nil, err
	}

	row, err := s.queries.UpdateBlogPost(ctx, blogdb.UpdateBlogPostParams{
		TenantID:       toPgUUID(p.TenantID),
		ID:             toPgUUID(p.ID),
		Title:          p.Title,
		Slug:           p.Slug,
		SeoDescription: p.SeoDescription,
		ContentBlocks:  rawBlocks,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrPostNotFound
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrDuplicateSlug
		}
		return nil, err
	}

	return mapDBToBlogPost(row)
}

func (s *Service) PublishPost(ctx context.Context, tenantID, id uuid.UUID) (*BlogPost, error) {
	if tenantID == uuid.Nil || id == uuid.Nil {
		return nil, errors.New("blog: tenant_id and post id cannot be nil")
	}

	row, err := s.queries.PublishBlogPost(ctx, blogdb.PublishBlogPostParams{
		TenantID: toPgUUID(tenantID),
		ID:       toPgUUID(id),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrPostNotFound
		}
		return nil, err
	}

	return mapDBToBlogPost(row)
}

func (s *Service) UnpublishPost(ctx context.Context, tenantID, id uuid.UUID) (*BlogPost, error) {
	if tenantID == uuid.Nil || id == uuid.Nil {
		return nil, errors.New("blog: tenant_id and post id cannot be nil")
	}

	row, err := s.queries.UnpublishBlogPost(ctx, blogdb.UnpublishBlogPostParams{
		TenantID: toPgUUID(tenantID),
		ID:       toPgUUID(id),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrPostNotFound
		}
		return nil, err
	}

	return mapDBToBlogPost(row)
}

func (s *Service) GetAdminPost(ctx context.Context, tenantID, id uuid.UUID) (*BlogPost, error) {
	if tenantID == uuid.Nil || id == uuid.Nil {
		return nil, errors.New("blog: tenant_id and post id cannot be nil")
	}

	row, err := s.queries.GetBlogPostByID(ctx, blogdb.GetBlogPostByIDParams{
		TenantID: toPgUUID(tenantID),
		ID:       toPgUUID(id),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrPostNotFound
		}
		return nil, err
	}

	return mapDBToBlogPost(row)
}

func (s *Service) ListAdminPosts(ctx context.Context, tenantID uuid.UUID, statusFilter *string) ([]BlogPost, error) {
	if tenantID == uuid.Nil {
		return nil, errors.New("blog: tenant_id cannot be nil")
	}

	var statusParam pgtype.Text
	if statusFilter != nil && *statusFilter != "" {
		statusParam = pgtype.Text{String: *statusFilter, Valid: true}
	}

	rows, err := s.queries.ListAdminBlogPosts(ctx, blogdb.ListAdminBlogPostsParams{
		TenantID: toPgUUID(tenantID),
		Status:   statusParam,
	})
	if err != nil {
		return nil, err
	}

	res := make([]BlogPost, 0, len(rows))
	for _, r := range rows {
		bp, err := mapDBToBlogPost(r)
		if err != nil {
			return nil, err
		}
		res = append(res, *bp)
	}
	return res, nil
}

func (s *Service) DeletePost(ctx context.Context, tenantID, id uuid.UUID) error {
	if tenantID == uuid.Nil || id == uuid.Nil {
		return errors.New("blog: tenant_id and post id cannot be nil")
	}

	return s.queries.DeleteBlogPost(ctx, blogdb.DeleteBlogPostParams{
		TenantID: toPgUUID(tenantID),
		ID:       toPgUUID(id),
	})
}
