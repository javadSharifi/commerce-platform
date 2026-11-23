package blog

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"

	"psychology/api/internal/blog/blogdb"
	"psychology/api/internal/domain/domaindb"
	"psychology/api/internal/media"
	"psychology/api/internal/platform/cache"
	"psychology/api/internal/platform/db"
	"psychology/api/internal/platform/tenantctx"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrInvalidMetadata = errors.New("blog: invalid author name or category")
	ErrInvalidCover    = errors.New("blog: cover must be a verified public image from this clinic")
)

type PostMetadata struct {
	AuthorName   string     `json:"author_name,omitempty"`
	Category     string     `json:"category,omitempty"`
	IsFeatured   bool       `json:"is_featured"`
	CoverMediaID *uuid.UUID `json:"cover_media_id,omitempty"`
}

type MetadataInput struct {
	AuthorName   *string `json:"author_name"`
	Category     *string `json:"category"`
	IsFeatured   *bool   `json:"is_featured"`
	CoverMediaID *string `json:"cover_media_id"`
}

func optionalText(value *string, limit int) (pgtype.Text, error) {
	if value == nil {
		return pgtype.Text{}, nil
	}
	normalized := strings.Join(strings.Fields(*value), " ")
	if utf8.RuneCountInString(normalized) > limit || strings.ContainsFunc(normalized, unicode.IsControl) {
		return pgtype.Text{}, ErrInvalidMetadata
	}
	return pgtype.Text{String: normalized, Valid: true}, nil
}

func parseMetadata(in MetadataInput) (author, category pgtype.Text, featured pgtype.Bool, cover pgtype.UUID, err error) {
	author, err = optionalText(in.AuthorName, 120)
	if err != nil {
		return
	}
	category, err = optionalText(in.Category, 80)
	if err != nil {
		return
	}
	if in.IsFeatured != nil {
		featured = pgtype.Bool{Bool: *in.IsFeatured, Valid: true}
	}
	if in.CoverMediaID != nil && *in.CoverMediaID != "" {
		id, parseErr := uuid.Parse(*in.CoverMediaID)
		if parseErr != nil || id == uuid.Nil {
			err = ErrInvalidCover
			return
		}
		cover = toPgUUID(id)
	}
	return
}

func metadata(author, category string, featured bool, cover pgtype.UUID) PostMetadata {
	result := PostMetadata{AuthorName: author, Category: category, IsFeatured: featured}
	if cover.Valid {
		id := fromPgUUID(cover)
		result.CoverMediaID = &id
	}
	return result
}

func unavailableImage(err error) bool {
	return errors.Is(err, media.ErrMediaNotFound) || errors.Is(err, media.ErrMediaNotPublic) ||
		errors.Is(err, media.ErrMediaNotVerified) || errors.Is(err, media.ErrInvalidMimeType) ||
		errors.Is(err, media.ErrFileSizeExceeded)
}

func validateCoverReference(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID, cover pgtype.UUID) error {
	if tx == nil || !cover.Valid {
		return nil
	}
	err := media.ValidatePublicImage(ctx, tx, tenantID, fromPgUUID(cover))
	if unavailableImage(err) {
		return ErrInvalidCover
	}
	return err
}

type publicBlogQueries struct {
	Querier
	tx pgx.Tx
}

func bindPublicBlog(tx pgx.Tx) publicBlogQueries {
	return publicBlogQueries{Querier: blogdb.New(tx), tx: tx}
}

func (q publicBlogQueries) checkCover(ctx context.Context, tenantID uuid.UUID, meta *PostMetadata) error {
	if q.tx == nil || meta.CoverMediaID == nil {
		return nil
	}
	err := media.ValidatePublicImage(ctx, q.tx, tenantID, *meta.CoverMediaID)
	if unavailableImage(err) {
		meta.CoverMediaID = nil
		return nil
	}
	return err
}

func (s *Service) mutatePost(ctx context.Context, tenantID, postID uuid.UUID, validateCover bool,
	apply func(scoped *Service, previous blogdb.BlogPost) (*BlogPost, error)) (*BlogPost, error) {
	if s.store == nil {
		return apply(s, blogdb.BlogPost{})
	}
	if id, ok := tenantctx.ID(ctx); !ok || id != tenantID {
		return nil, db.ErrNoTenant
	}
	var saved *BlogPost
	err := s.store.WithTenantTx(ctx, func(tx pgx.Tx, tid uuid.UUID) error {
		q := blogdb.New(tx)
		var previous blogdb.BlogPost
		var err error
		if postID != uuid.Nil {
			previous, err = q.LockBlogPost(ctx, blogdb.LockBlogPostParams{TenantID: toPgUUID(tid), ID: toPgUUID(postID)})
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrPostNotFound
			}
			if err != nil {
				return err
			}
		}
		saved, err = apply(&Service{queries: q, mediaTx: tx}, previous)
		if err != nil {
			return err
		}
		if validateCover && saved.CoverMediaID != nil {
			if err := validateCoverReference(ctx, tx, tid, toPgUUID(*saved.CoverMediaID)); err != nil {
				return err
			}
		}
		return invalidatePublishedBlog(ctx, tx, tid, previous, saved)
	})
	return saved, err
}

func invalidatePublishedBlog(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID, previous blogdb.BlogPost, saved *BlogPost) error {
	if previous.Status != "published" && (saved == nil || saved.Status != "published") {
		return nil
	}

	paths := []string{"/", "/blog", "/sitemap.xml"}
	var categories []string
	if previous.Status == "published" {
		paths = append(paths, "/blog/"+url.PathEscape(previous.Slug))
		categories = append(categories, previous.Category)
	}
	if saved != nil && saved.Status == "published" {
		if previous.Status != "published" || saved.Slug != previous.Slug {
			paths = append(paths, "/blog/"+url.PathEscape(saved.Slug))
		}
		categories = append(categories, saved.Category)
	}

	related, err := blogdb.New(tx).ListPublishedSlugsByCategories(ctx, blogdb.ListPublishedSlugsByCategoriesParams{
		TenantID: toPgUUID(tenantID), Categories: categories,
	})
	if err != nil {
		return err
	}
	for _, slug := range related {
		if slug != previous.Slug && (saved == nil || slug != saved.Slug) {
			paths = append(paths, "/blog/"+url.PathEscape(slug))
		}
	}

	domains, err := domaindb.New(tx).ListDomainsByTenant(ctx, toPgUUID(tenantID))
	if err != nil {
		return err
	}
	for _, domain := range domains {
		if domain.IsVerified {
			if err := cache.InsertCacheInvalidationEvent(ctx, tx, tenantID, "blog.updated", domain.Host, paths); err != nil {
				return err
			}
		}
	}
	return nil
}
