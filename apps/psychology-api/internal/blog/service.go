package blog

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"

	"psychology/api/internal/blog/blogdb"
	"psychology/api/internal/platform/db"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrPostNotFound        = errors.New("blog: post not found")
	ErrDuplicateSlug       = errors.New("blog: slug already exists for this clinic")
	ErrInvalidSlug         = errors.New("blog: invalid slug format (alphanumeric and dashes only)")
	ErrInvalidTitle        = errors.New("blog: title is required")
	ErrInvalidContentBlock = errors.New("blog: invalid content block structure or type")
	ErrScriptInjection     = errors.New("blog: raw html or script execution tags are forbidden")
	ErrInvalidStatus       = errors.New("blog: invalid status")
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
	PostMetadata
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
	PostMetadata
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
	mediaTx pgx.Tx
	store   *db.Store
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
	}
	if blocks == nil {
		blocks = make([]ContentBlock, 0)
	}

	var pubAt *time.Time
	if row.PublishedAt.Valid {
		t := row.PublishedAt.Time
		pubAt = &t
	}

	return &BlogPost{
		PostMetadata:   metadata(row.AuthorName, row.Category, row.IsFeatured, row.CoverMediaID),
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

func (s *Service) WithStore(store *db.Store) *Service {
	s.store = store
	return s
}

// ListPublishedPosts lists published blog posts for a tenant (Drafts are strictly excluded).
func (s *Service) ListPublishedPosts(ctx context.Context, tenantID uuid.UUID) ([]PublicBlogPostItem, error) {
	return db.ReadTenant(ctx, s.store, tenantID, publicBlogQueries{Querier: s.queries}, bindPublicBlog,
		func(q publicBlogQueries) ([]PublicBlogPostItem, error) {
			if tenantID == uuid.Nil {
				return nil, errors.New("blog: tenant_id cannot be nil")
			}

			rows, err := q.ListPublishedBlogPosts(ctx, toPgUUID(tenantID))
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
				meta := metadata(r.AuthorName, r.Category, r.IsFeatured, r.CoverMediaID)
				if err := q.checkCover(ctx, tenantID, &meta); err != nil {
					return nil, err
				}
				res = append(res, PublicBlogPostItem{
					PostMetadata:   meta,
					ID:             fromPgUUID(r.ID),
					Title:          r.Title,
					Slug:           r.Slug,
					SeoDescription: r.SeoDescription,
					PublishedAt:    pubAt,
				})
			}
			return res, nil
		},
	)
}

type PublicBlogPostDetail struct {
	PublicBlogPostItem
	ContentBlocks []ContentBlock `json:"content_blocks"`
}

// GetPublishedPostBySlug returns a published post. If the post is a draft or not found, ErrPostNotFound is returned.
func (s *Service) GetPublishedPostBySlug(ctx context.Context, tenantID uuid.UUID, slug string) (*PublicBlogPostDetail, error) {
	return db.ReadTenant(ctx, s.store, tenantID, publicBlogQueries{Querier: s.queries}, bindPublicBlog,
		func(q publicBlogQueries) (*PublicBlogPostDetail, error) {
			if tenantID == uuid.Nil {
				return nil, errors.New("blog: tenant_id cannot be nil")
			}
			slug = strings.TrimSpace(slug)
			if slug == "" {
				return nil, ErrPostNotFound
			}

			row, err := q.GetPublishedBlogPostBySlug(ctx, blogdb.GetPublishedBlogPostBySlugParams{
				TenantID: toPgUUID(tenantID),
				Slug:     slug,
			})
			if err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return nil, ErrPostNotFound
				}
				return nil, err
			}

			post, err := mapDBToBlogPost(row)
			if err != nil {
				return nil, err
			}
			if err := q.checkCover(ctx, tenantID, &post.PostMetadata); err != nil {
				return nil, err
			}
			return &PublicBlogPostDetail{
				PublicBlogPostItem: PublicBlogPostItem{
					PostMetadata: post.PostMetadata,
					ID:           post.ID, Title: post.Title, Slug: post.Slug,
					SeoDescription: post.SeoDescription, PublishedAt: post.PublishedAt,
				},
				ContentBlocks: post.ContentBlocks,
			}, nil
		},
	)
}

// ---------------- Admin Methods ----------------

type CreatePostParams struct {
	MetadataInput
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
	if s.store != nil {
		return s.mutatePost(ctx, p.TenantID, uuid.Nil, false, func(scoped *Service, _ blogdb.BlogPost) (*BlogPost, error) {
			return scoped.CreatePost(ctx, p)
		})
	}
	author, category, featured, cover, err := parseMetadata(p.MetadataInput)
	if err != nil {
		return nil, err
	}
	if err := validateCoverReference(ctx, s.mediaTx, p.TenantID, cover); err != nil {
		return nil, err
	}

	if p.ContentBlocks == nil {
		p.ContentBlocks = []ContentBlock{}
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
		AuthorName:     author.String,
		Category:       category.String,
		IsFeatured:     featured.Bool,
		CoverMediaID:   cover,
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
	MetadataInput
	TenantID       uuid.UUID
	ID             uuid.UUID
	Title          string
	Slug           string
	SeoDescription *string
	ContentBlocks  []ContentBlock
}

func (s *Service) UpdatePost(ctx context.Context, p UpdatePostParams) (*BlogPost, error) {
	if p.TenantID == uuid.Nil || p.ID == uuid.Nil {
		return nil, errors.New("blog: tenant_id and post id cannot be nil")
	}
	if s.store != nil {
		return s.mutatePost(ctx, p.TenantID, p.ID, false, func(scoped *Service, previous blogdb.BlogPost) (*BlogPost, error) {
			if p.Title == "" {
				p.Title = previous.Title
			}
			if p.Slug == "" {
				p.Slug = previous.Slug
			}
			return scoped.UpdatePost(ctx, p)
		})
	}
	author, category, featured, cover, err := parseMetadata(p.MetadataInput)
	if err != nil {
		return nil, err
	}
	if err := validateCoverReference(ctx, s.mediaTx, p.TenantID, cover); err != nil {
		return nil, err
	}

	var seo pgtype.Text
	if p.SeoDescription != nil {
		seo = pgtype.Text{String: *p.SeoDescription, Valid: true}
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

	var rawBlocks []byte
	if p.ContentBlocks != nil {
		rawBlocks, err = json.Marshal(p.ContentBlocks)
		if err != nil {
			return nil, err
		}
	}

	row, err := s.queries.UpdateBlogPost(ctx, blogdb.UpdateBlogPostParams{
		TenantID:       toPgUUID(p.TenantID),
		ID:             toPgUUID(p.ID),
		Title:          p.Title,
		Slug:           p.Slug,
		SeoDescription: seo,
		AuthorName:     author, Category: category, IsFeatured: featured,
		CoverMediaID: cover, ReplaceCover: p.CoverMediaID != nil,
		ContentBlocks: rawBlocks,
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
	if s.store != nil {
		return s.mutatePost(ctx, tenantID, id, true, func(scoped *Service, _ blogdb.BlogPost) (*BlogPost, error) {
			return scoped.PublishPost(ctx, tenantID, id)
		})
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
	if s.store != nil {
		return s.mutatePost(ctx, tenantID, id, false, func(scoped *Service, _ blogdb.BlogPost) (*BlogPost, error) {
			return scoped.UnpublishPost(ctx, tenantID, id)
		})
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
	if s.store != nil {
		return db.ReadTenant(ctx, s.store, tenantID, s.queries,
			func(tx pgx.Tx) Querier { return blogdb.New(tx) },
			func(q Querier) (*BlogPost, error) { return NewService(q).GetAdminPost(ctx, tenantID, id) },
		)
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
	if s.store != nil {
		return db.ReadTenant(ctx, s.store, tenantID, s.queries,
			func(tx pgx.Tx) Querier { return blogdb.New(tx) },
			func(q Querier) ([]BlogPost, error) { return NewService(q).ListAdminPosts(ctx, tenantID, statusFilter) },
		)
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
	if s.store != nil {
		_, err := s.mutatePost(ctx, tenantID, id, false, func(scoped *Service, _ blogdb.BlogPost) (*BlogPost, error) {
			return nil, scoped.DeletePost(ctx, tenantID, id)
		})
		return err
	}

	return s.queries.DeleteBlogPost(ctx, blogdb.DeleteBlogPostParams{
		TenantID: toPgUUID(tenantID),
		ID:       toPgUUID(id),
	})
}
