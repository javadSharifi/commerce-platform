package blog_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"psychology/api/internal/auth"
	"psychology/api/internal/auth/authdb"
	"psychology/api/internal/blog"
	"psychology/api/internal/blog/blogdb"
	"psychology/api/internal/domain"
	"psychology/api/internal/domain/domaindb"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

type mockBlogQuerier struct {
	mu    sync.Mutex
	posts map[string]blogdb.BlogPost
}

func newMockBlogQuerier() *mockBlogQuerier {
	return &mockBlogQuerier{
		posts: make(map[string]blogdb.BlogPost),
	}
}

func toPgUUID(u uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: u, Valid: true}
}

func fromPgUUID(p pgtype.UUID) uuid.UUID {
	var u uuid.UUID
	copy(u[:], p.Bytes[:])
	return u
}

func (m *mockBlogQuerier) CreateBlogPost(ctx context.Context, arg blogdb.CreateBlogPostParams) (blogdb.BlogPost, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check unique (tenant_id, slug)
	for _, p := range m.posts {
		if fromPgUUID(p.TenantID) == fromPgUUID(arg.TenantID) && p.Slug == arg.Slug {
			return blogdb.BlogPost{}, &pgconn.PgError{Code: "23505", Message: "duplicate slug"}
		}
	}

	id := uuid.New()
	now := time.Now()
	bp := blogdb.BlogPost{
		ID:             toPgUUID(id),
		TenantID:       arg.TenantID,
		Title:          arg.Title,
		Slug:           arg.Slug,
		SeoDescription: arg.SeoDescription,
		ContentBlocks:  arg.ContentBlocks,
		Status:         arg.Status,
		PublishedAt:    arg.PublishedAt,
		CreatedAt:      pgtype.Timestamptz{Time: now, Valid: true},
		UpdatedAt:      pgtype.Timestamptz{Time: now, Valid: true},
	}
	m.posts[id.String()] = bp
	return bp, nil
}

func (m *mockBlogQuerier) GetBlogPostByID(ctx context.Context, arg blogdb.GetBlogPostByIDParams) (blogdb.BlogPost, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	bp, ok := m.posts[fromPgUUID(arg.ID).String()]
	if !ok || fromPgUUID(bp.TenantID) != fromPgUUID(arg.TenantID) {
		return blogdb.BlogPost{}, pgx.ErrNoRows
	}
	return bp, nil
}

func (m *mockBlogQuerier) GetPublishedBlogPostBySlug(ctx context.Context, arg blogdb.GetPublishedBlogPostBySlugParams) (blogdb.BlogPost, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, bp := range m.posts {
		if fromPgUUID(bp.TenantID) == fromPgUUID(arg.TenantID) && bp.Slug == arg.Slug && bp.Status == "published" {
			return bp, nil
		}
	}
	return blogdb.BlogPost{}, pgx.ErrNoRows
}

func (m *mockBlogQuerier) ListPublishedBlogPosts(ctx context.Context, tenantID pgtype.UUID) ([]blogdb.ListPublishedBlogPostsRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var res []blogdb.ListPublishedBlogPostsRow
	for _, bp := range m.posts {
		if fromPgUUID(bp.TenantID) == fromPgUUID(tenantID) && bp.Status == "published" {
			res = append(res, blogdb.ListPublishedBlogPostsRow{
				ID:             bp.ID,
				TenantID:       bp.TenantID,
				Title:          bp.Title,
				Slug:           bp.Slug,
				SeoDescription: bp.SeoDescription,
				PublishedAt:    bp.PublishedAt,
			})
		}
	}
	return res, nil
}

func (m *mockBlogQuerier) ListAdminBlogPosts(ctx context.Context, arg blogdb.ListAdminBlogPostsParams) ([]blogdb.BlogPost, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var res []blogdb.BlogPost
	for _, bp := range m.posts {
		if fromPgUUID(bp.TenantID) == fromPgUUID(arg.TenantID) {
			if !arg.Status.Valid || bp.Status == arg.Status.String {
				res = append(res, bp)
			}
		}
	}
	return res, nil
}

func (m *mockBlogQuerier) UpdateBlogPost(ctx context.Context, arg blogdb.UpdateBlogPostParams) (blogdb.BlogPost, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	bp, ok := m.posts[fromPgUUID(arg.ID).String()]
	if !ok || fromPgUUID(bp.TenantID) != fromPgUUID(arg.TenantID) {
		return blogdb.BlogPost{}, pgx.ErrNoRows
	}

	// Check slug unique
	for id, other := range m.posts {
		if id != fromPgUUID(arg.ID).String() && fromPgUUID(other.TenantID) == fromPgUUID(arg.TenantID) && other.Slug == arg.Slug {
			return blogdb.BlogPost{}, &pgconn.PgError{Code: "23505", Message: "duplicate slug"}
		}
	}

	bp.Title = arg.Title
	bp.Slug = arg.Slug
	bp.SeoDescription = arg.SeoDescription
	bp.ContentBlocks = arg.ContentBlocks
	bp.UpdatedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	m.posts[fromPgUUID(arg.ID).String()] = bp
	return bp, nil
}

func (m *mockBlogQuerier) PublishBlogPost(ctx context.Context, arg blogdb.PublishBlogPostParams) (blogdb.BlogPost, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	bp, ok := m.posts[fromPgUUID(arg.ID).String()]
	if !ok || fromPgUUID(bp.TenantID) != fromPgUUID(arg.TenantID) {
		return blogdb.BlogPost{}, pgx.ErrNoRows
	}

	bp.Status = "published"
	if !bp.PublishedAt.Valid {
		bp.PublishedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	}
	bp.UpdatedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	m.posts[fromPgUUID(arg.ID).String()] = bp
	return bp, nil
}

func (m *mockBlogQuerier) UnpublishBlogPost(ctx context.Context, arg blogdb.UnpublishBlogPostParams) (blogdb.BlogPost, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	bp, ok := m.posts[fromPgUUID(arg.ID).String()]
	if !ok || fromPgUUID(bp.TenantID) != fromPgUUID(arg.TenantID) {
		return blogdb.BlogPost{}, pgx.ErrNoRows
	}

	bp.Status = "draft"
	bp.UpdatedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	m.posts[fromPgUUID(arg.ID).String()] = bp
	return bp, nil
}

func (m *mockBlogQuerier) DeleteBlogPost(ctx context.Context, arg blogdb.DeleteBlogPostParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.posts, fromPgUUID(arg.ID).String())
	return nil
}

// ---------------- Mock Auth ----------------
type mockAuthQuerier struct {
	memberships map[string]authdb.Membership
	sessions    map[string]authdb.PanelSession
	identities  map[string]authdb.PanelIdentity
}

func (m *mockAuthQuerier) GetPanelIdentityByEmail(ctx context.Context, email string) (authdb.PanelIdentity, error) {
	return authdb.PanelIdentity{}, pgx.ErrNoRows
}
func (m *mockAuthQuerier) GetPanelIdentityByID(ctx context.Context, id pgtype.UUID) (authdb.PanelIdentity, error) {
	i, ok := m.identities[fromPgUUID(id).String()]
	if !ok {
		return authdb.PanelIdentity{}, pgx.ErrNoRows
	}
	return i, nil
}
func (m *mockAuthQuerier) CreatePanelIdentity(ctx context.Context, arg authdb.CreatePanelIdentityParams) (authdb.PanelIdentity, error) {
	return authdb.PanelIdentity{}, nil
}
func (m *mockAuthQuerier) CreatePanelSession(ctx context.Context, arg authdb.CreatePanelSessionParams) (authdb.PanelSession, error) {
	return authdb.PanelSession{}, nil
}
func (m *mockAuthQuerier) GetPanelSessionByTokenHash(ctx context.Context, tokenHash string) (authdb.PanelSession, error) {
	s, ok := m.sessions[tokenHash]
	if !ok {
		return authdb.PanelSession{}, pgx.ErrNoRows
	}
	return s, nil
}
func (m *mockAuthQuerier) DeletePanelSession(ctx context.Context, tokenHash string) error {
	return nil
}
func (m *mockAuthQuerier) ListMembershipsByUserID(ctx context.Context, userID pgtype.UUID) ([]authdb.ListMembershipsByUserIDRow, error) {
	return nil, nil
}
func (m *mockAuthQuerier) GetMembershipByUserAndTenant(ctx context.Context, arg authdb.GetMembershipByUserAndTenantParams) (authdb.Membership, error) {
	key := fromPgUUID(arg.UserID).String() + ":" + fromPgUUID(arg.TenantID).String()
	mem, ok := m.memberships[key]
	if !ok {
		return authdb.Membership{}, pgx.ErrNoRows
	}
	return mem, nil
}
func (m *mockAuthQuerier) CreateMembership(ctx context.Context, arg authdb.CreateMembershipParams) (authdb.Membership, error) {
	return authdb.Membership{}, nil
}

// ---------------- Mock Domain ----------------
type mockDomainQuerier struct {
	domains map[string]domaindb.Domain
}

func (m *mockDomainQuerier) GetDomainByHost(ctx context.Context, host string) (domaindb.Domain, error) {
	d, ok := m.domains[host]
	if !ok {
		return domaindb.Domain{}, pgx.ErrNoRows
	}
	return d, nil
}

// ----------------------------------------------------
// TESTS
// ----------------------------------------------------

func TestPublicBlog_DraftIsolation(t *testing.T) {
	blogMock := newMockBlogQuerier()
	svc := blog.NewService(blogMock)

	tenantID := uuid.New()
	domainMock := &mockDomainQuerier{
		domains: map[string]domaindb.Domain{
			"clinic.ir": {
				ID:         toPgUUID(uuid.New()),
				TenantID:   toPgUUID(tenantID),
				Host:       "clinic.ir",
				IsVerified: true,
			},
		},
	}
	resolver := domain.NewResolver(domainMock)
	handler := blog.NewHandler(svc, resolver, nil)

	r := chi.NewRouter()
	handler.MountRoutes(r)

	// 1. Create a draft post
	draftPost, err := svc.CreatePost(context.Background(), blog.CreatePostParams{
		TenantID:       tenantID,
		Title:          "راهنمای تنظیم هیجان و کاهش استرس",
		Slug:           "stress-management-guide",
		SeoDescription: "تکنیک‌های عملی خودآگاهی",
		ContentBlocks: []blog.ContentBlock{
			{
				Type: blog.BlockParagraph,
				Data: map[string]any{"text": "مقدمه‌ای بر کنترل اضطراب..."},
			},
		},
		Status: "draft",
	})
	if err != nil {
		t.Fatalf("failed to create draft post: %v", err)
	}

	// 2. Query public list: draft MUST NOT be returned!
	reqList := httptest.NewRequest("GET", "/public/blog", nil)
	reqList.Host = "clinic.ir"
	recList := httptest.NewRecorder()
	r.ServeHTTP(recList, reqList)

	if recList.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got: %d", recList.Code)
	}
	var listItems []blog.PublicBlogPostItem
	_ = json.NewDecoder(recList.Body).Decode(&listItems)
	if len(listItems) != 0 {
		t.Errorf("expected 0 published posts, got %d (draft leaked!)", len(listItems))
	}

	// 3. Query public detail by slug: draft MUST return 404 NOT_FOUND!
	reqDetail := httptest.NewRequest("GET", "/public/blog/stress-management-guide", nil)
	reqDetail.Host = "clinic.ir"
	recDetail := httptest.NewRecorder()
	r.ServeHTTP(recDetail, reqDetail)

	if recDetail.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found for draft post, got: %d", recDetail.Code)
	}

	// 4. Publish the post
	_, err = svc.PublishPost(context.Background(), tenantID, draftPost.ID)
	if err != nil {
		t.Fatalf("failed to publish post: %v", err)
	}

	// 5. Query public list now: MUST appear
	recList2 := httptest.NewRecorder()
	r.ServeHTTP(recList2, reqList)
	if recList2.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got: %d", recList2.Code)
	}
	_ = json.NewDecoder(recList2.Body).Decode(&listItems)
	if len(listItems) != 1 {
		t.Fatalf("expected 1 published post, got %d", len(listItems))
	}
	if listItems[0].Slug != "stress-management-guide" {
		t.Errorf("expected slug 'stress-management-guide', got: %s", listItems[0].Slug)
	}

	// 6. Query public detail by slug: MUST return 200 OK
	recDetail2 := httptest.NewRecorder()
	r.ServeHTTP(recDetail2, reqDetail)
	if recDetail2.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for published post, got: %d", recDetail2.Code)
	}

	// 7. Unpublish post (revert to draft): MUST become invisible again
	_, err = svc.UnpublishPost(context.Background(), tenantID, draftPost.ID)
	if err != nil {
		t.Fatalf("failed to unpublish: %v", err)
	}

	recDetail3 := httptest.NewRecorder()
	r.ServeHTTP(recDetail3, reqDetail)
	if recDetail3.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found after unpublishing, got: %d", recDetail3.Code)
	}
}

func TestAdminBlog_ContentBlockValidation_And_ScriptInjection(t *testing.T) {
	blogMock := newMockBlogQuerier()
	svc := blog.NewService(blogMock)
	tenantID := uuid.New()

	ctx := context.Background()

	// 1. Valid blocks
	validBlocks := []blog.ContentBlock{
		{
			Type: blog.BlockHeading,
			Data: map[string]any{"text": "سرفصل اول", "level": 2},
		},
		{
			Type: blog.BlockParagraph,
			Data: map[string]any{"text": "متن پاراگراف معمولی و ایمن."},
		},
		{
			Type: blog.BlockImage,
			Data: map[string]any{"url": "https://cdn.example.com/image.webp", "alt": "تصویر"},
		},
	}
	_, err := svc.CreatePost(ctx, blog.CreatePostParams{
		TenantID:      tenantID,
		Title:         "پست معتبر",
		Slug:          "valid-post-1",
		ContentBlocks: validBlocks,
	})
	if err != nil {
		t.Fatalf("expected success for valid blocks, got: %v", err)
	}

	// 2. Script tag injection in paragraph -> must be rejected with ErrScriptInjection
	scriptBlocks := []blog.ContentBlock{
		{
			Type: blog.BlockParagraph,
			Data: map[string]any{"text": "<script>alert('xss')</script>"},
		},
	}
	_, err = svc.CreatePost(ctx, blog.CreatePostParams{
		TenantID:      tenantID,
		Title:         "پست حاوی اسکریپت",
		Slug:          "xss-post-1",
		ContentBlocks: scriptBlocks,
	})
	if !errors.Is(err, blog.ErrScriptInjection) {
		t.Errorf("expected ErrScriptInjection, got: %v", err)
	}

	// 3. Iframe injection -> must be rejected
	iframeBlocks := []blog.ContentBlock{
		{
			Type: blog.BlockParagraph,
			Data: map[string]any{"text": "<iframe src='https://malicious.com'></iframe>"},
		},
	}
	_, err = svc.CreatePost(ctx, blog.CreatePostParams{
		TenantID:      tenantID,
		Title:         "پست آی‌فریم",
		Slug:          "iframe-post",
		ContentBlocks: iframeBlocks,
	})
	if !errors.Is(err, blog.ErrScriptInjection) {
		t.Errorf("expected ErrScriptInjection for iframe, got: %v", err)
	}

	// 4. Invalid block type -> must be rejected with ErrInvalidContentBlock
	unknownTypeBlocks := []blog.ContentBlock{
		{
			Type: "raw_html",
			Data: map[string]any{"html": "<div>custom</div>"},
		},
	}
	_, err = svc.CreatePost(ctx, blog.CreatePostParams{
		TenantID:      tenantID,
		Title:         "نوع نامعتبر",
		Slug:          "unknown-type",
		ContentBlocks: unknownTypeBlocks,
	})
	if !errors.Is(err, blog.ErrInvalidContentBlock) {
		t.Errorf("expected ErrInvalidContentBlock, got: %v", err)
	}

	// 5. Heading with invalid level (>6) -> must be rejected
	invalidLevelBlocks := []blog.ContentBlock{
		{
			Type: blog.BlockHeading,
			Data: map[string]any{"text": "تیتر", "level": 10},
		},
	}
	_, err = svc.CreatePost(ctx, blog.CreatePostParams{
		TenantID:      tenantID,
		Title:         "تیتر نامعتبر",
		Slug:          "invalid-level",
		ContentBlocks: invalidLevelBlocks,
	})
	if !errors.Is(err, blog.ErrInvalidContentBlock) {
		t.Errorf("expected ErrInvalidContentBlock for level > 6, got: %v", err)
	}
}

func TestAdminBlog_SlugUniquenessPerTenant(t *testing.T) {
	blogMock := newMockBlogQuerier()
	svc := blog.NewService(blogMock)

	tenantAID := uuid.New()
	tenantBID := uuid.New()
	ctx := context.Background()

	// 1. Post in Tenant A with slug 'parenting-skills' -> OK
	_, err := svc.CreatePost(ctx, blog.CreatePostParams{
		TenantID:      tenantAID,
		Title:         "مهارت‌های فرزندپروری الف",
		Slug:          "parenting-skills",
		ContentBlocks: []blog.ContentBlock{{Type: blog.BlockParagraph, Data: map[string]any{"text": "متن"}}},
	})
	if err != nil {
		t.Fatalf("unexpected error creating first post: %v", err)
	}

	// 2. Duplicate slug in SAME Tenant A -> 409 ErrDuplicateSlug
	_, err = svc.CreatePost(ctx, blog.CreatePostParams{
		TenantID:      tenantAID,
		Title:         "مهارت‌های فرزندپروری دوم",
		Slug:          "parenting-skills",
		ContentBlocks: []blog.ContentBlock{{Type: blog.BlockParagraph, Data: map[string]any{"text": "متن ۲"}}},
	})
	if !errors.Is(err, blog.ErrDuplicateSlug) {
		t.Errorf("expected ErrDuplicateSlug for duplicate slug in same tenant, got: %v", err)
	}

	// 3. Same slug in DIFFERENT Tenant B -> Allowed! (Tenant-scoped uniqueness)
	_, err = svc.CreatePost(ctx, blog.CreatePostParams{
		TenantID:      tenantBID,
		Title:         "مهارت‌های فرزندپروری کلینیک ب",
		Slug:          "parenting-skills",
		ContentBlocks: []blog.ContentBlock{{Type: blog.BlockParagraph, Data: map[string]any{"text": "متن ب"}}},
	})
	if err != nil {
		t.Errorf("expected same slug to be allowed in different tenant, got: %v", err)
	}

	// 4. Invalid slug formats
	invalidSlugs := []string{
		"Invalid Uppercase",
		"slug with spaces",
		"slug_with_underscores",
		"-leading-dash",
		"trailing-dash-",
		"invalid/slash",
	}
	for _, badSlug := range invalidSlugs {
		_, err := svc.CreatePost(ctx, blog.CreatePostParams{
			TenantID:      tenantAID,
			Title:         "عنوان",
			Slug:          badSlug,
			ContentBlocks: []blog.ContentBlock{{Type: blog.BlockParagraph, Data: map[string]any{"text": "متن"}}},
		})
		if !errors.Is(err, blog.ErrInvalidSlug) {
			t.Errorf("expected ErrInvalidSlug for '%s', got: %v", badSlug, err)
		}
	}
}

func TestAdminBlog_RoleAccessControl(t *testing.T) {
	blogMock := newMockBlogQuerier()
	svc := blog.NewService(blogMock)

	tenantAID := uuid.New()
	ownerUID := uuid.New()
	managerUID := uuid.New()
	recepUID := uuid.New()

	mockAuth := &mockAuthQuerier{
		identities: map[string]authdb.PanelIdentity{
			ownerUID.String():   {ID: toPgUUID(ownerUID), Email: "owner@clinic.ir", FullName: "دکتر مالک", Status: "active"},
			managerUID.String(): {ID: toPgUUID(managerUID), Email: "manager@clinic.ir", FullName: "مدیر محتوا", Status: "active"},
			recepUID.String():   {ID: toPgUUID(recepUID), Email: "recep@clinic.ir", FullName: "پذیرش", Status: "active"},
		},
		sessions: map[string]authdb.PanelSession{
			auth.HashToken("token-owner"):   {ID: toPgUUID(uuid.New()), UserID: toPgUUID(ownerUID), TokenHash: auth.HashToken("token-owner")},
			auth.HashToken("token-manager"): {ID: toPgUUID(uuid.New()), UserID: toPgUUID(managerUID), TokenHash: auth.HashToken("token-manager")},
			auth.HashToken("token-recep"):   {ID: toPgUUID(uuid.New()), UserID: toPgUUID(recepUID), TokenHash: auth.HashToken("token-recep")},
		},
		memberships: map[string]authdb.Membership{
			ownerUID.String() + ":" + tenantAID.String():   {ID: toPgUUID(uuid.New()), TenantID: toPgUUID(tenantAID), UserID: toPgUUID(ownerUID), Role: "owner"},
			managerUID.String() + ":" + tenantAID.String(): {ID: toPgUUID(uuid.New()), TenantID: toPgUUID(tenantAID), UserID: toPgUUID(managerUID), Role: "content_manager"},
			recepUID.String() + ":" + tenantAID.String():   {ID: toPgUUID(uuid.New()), TenantID: toPgUUID(tenantAID), UserID: toPgUUID(recepUID), Role: "receptionist"},
		},
	}

	authSvc := auth.NewService(mockAuth)
	handler := blog.NewHandler(svc, nil, authSvc)

	r := chi.NewRouter()
	handler.MountRoutes(r)

	body, _ := json.Marshal(map[string]any{
		"title":           "مطلب آزمایشی",
		"slug":            "test-post-roles",
		"seo_description": "توضیح سئو",
		"content_blocks": []blog.ContentBlock{
			{Type: blog.BlockParagraph, Data: map[string]any{"text": "محتوای مقاله"}},
		},
	})

	// 1. Content Manager creates post -> 201 Created
	reqManager := httptest.NewRequest("POST", "/admin/blog", bytes.NewReader(body))
	reqManager.Header.Set("Authorization", "Bearer token-manager")
	reqManager.Header.Set("X-Tenant-ID", tenantAID.String())
	reqManager.Header.Set("Content-Type", "application/json")
	recManager := httptest.NewRecorder()
	r.ServeHTTP(recManager, reqManager)

	if recManager.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for content_manager, got: %d (%s)", recManager.Code, recManager.Body.String())
	}

	var createdPost blog.BlogPost
	_ = json.NewDecoder(recManager.Body).Decode(&createdPost)

	// 2. Receptionist attempts to create post -> 403 Forbidden
	body2, _ := json.Marshal(map[string]any{
		"title": "مطلب غیرمجاز",
		"slug":  "unauthorized-post",
		"content_blocks": []blog.ContentBlock{
			{Type: blog.BlockParagraph, Data: map[string]any{"text": "تست"}},
		},
	})
	reqRecep := httptest.NewRequest("POST", "/admin/blog", bytes.NewReader(body2))
	reqRecep.Header.Set("Authorization", "Bearer token-recep")
	reqRecep.Header.Set("X-Tenant-ID", tenantAID.String())
	reqRecep.Header.Set("Content-Type", "application/json")
	recRecep := httptest.NewRecorder()
	r.ServeHTTP(recRecep, reqRecep)

	if recRecep.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for receptionist, got: %d", recRecep.Code)
	}

	// 3. Receptionist attempts to view admin post -> 403 Forbidden
	reqRecepGet := httptest.NewRequest("GET", "/admin/blog/"+createdPost.ID.String(), nil)
	reqRecepGet.Header.Set("Authorization", "Bearer token-recep")
	reqRecepGet.Header.Set("X-Tenant-ID", tenantAID.String())
	recRecepGet := httptest.NewRecorder()
	r.ServeHTTP(recRecepGet, reqRecepGet)

	if recRecepGet.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for receptionist viewing admin blog, got: %d", recRecepGet.Code)
	}

	// 4. Owner publishes post -> 200 OK
	reqPublish := httptest.NewRequest("POST", "/admin/blog/"+createdPost.ID.String()+"/publish", bytes.NewReader([]byte("{}")))
	reqPublish.Header.Set("Authorization", "Bearer token-owner")
	reqPublish.Header.Set("X-Tenant-ID", tenantAID.String())
	reqPublish.Header.Set("Content-Type", "application/json")
	recPublish := httptest.NewRecorder()
	r.ServeHTTP(recPublish, reqPublish)

	if recPublish.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for owner publishing, got: %d (%s)", recPublish.Code, recPublish.Body.String())
	}

	// 5. CSRF protection: cross-site -> 403 Forbidden
	reqCSRF := httptest.NewRequest("POST", "/admin/blog/"+createdPost.ID.String()+"/unpublish", bytes.NewReader([]byte("{}")))
	reqCSRF.Header.Set("Authorization", "Bearer token-owner")
	reqCSRF.Header.Set("X-Tenant-ID", tenantAID.String())
	reqCSRF.Header.Set("Content-Type", "application/json")
	reqCSRF.Header.Set("Sec-Fetch-Site", "cross-site")
	recCSRF := httptest.NewRecorder()
	r.ServeHTTP(recCSRF, reqCSRF)

	if recCSRF.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for cross-site CSRF attempt, got: %d", recCSRF.Code)
	}
}

func TestCrossTenantIsolation(t *testing.T) {
	blogMock := newMockBlogQuerier()
	svc := blog.NewService(blogMock)

	tenantAID := uuid.New()
	tenantBID := uuid.New()
	ownerA := uuid.New()

	mockAuth := &mockAuthQuerier{
		identities: map[string]authdb.PanelIdentity{
			ownerA.String(): {ID: toPgUUID(ownerA), Email: "owner-a@clinic.ir", FullName: "دکتر الف", Status: "active"},
		},
		sessions: map[string]authdb.PanelSession{
			auth.HashToken("token-a"): {ID: toPgUUID(uuid.New()), UserID: toPgUUID(ownerA), TokenHash: auth.HashToken("token-a")},
		},
		memberships: map[string]authdb.Membership{
			ownerA.String() + ":" + tenantAID.String(): {ID: toPgUUID(uuid.New()), TenantID: toPgUUID(tenantAID), UserID: toPgUUID(ownerA), Role: "owner"},
		},
	}

	authSvc := auth.NewService(mockAuth)
	handler := blog.NewHandler(svc, nil, authSvc)

	r := chi.NewRouter()
	handler.MountRoutes(r)

	// Post created in Tenant B
	postB, err := svc.CreatePost(context.Background(), blog.CreatePostParams{
		TenantID:      tenantBID,
		Title:         "پست اختصاصی کلینیک ب",
		Slug:          "clinic-b-post",
		ContentBlocks: []blog.ContentBlock{{Type: blog.BlockParagraph, Data: map[string]any{"text": "متن ب"}}},
	})
	if err != nil {
		t.Fatalf("failed to create post in tenant B: %v", err)
	}

	// Owner of A tries to access Tenant B post with Tenant A context -> 404 Not Found
	reqGet := httptest.NewRequest("GET", "/admin/blog/"+postB.ID.String(), nil)
	reqGet.Header.Set("Authorization", "Bearer token-a")
	reqGet.Header.Set("X-Tenant-ID", tenantAID.String())
	recGet := httptest.NewRecorder()
	r.ServeHTTP(recGet, reqGet)

	if recGet.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found when accessing another tenant's post, got: %d", recGet.Code)
	}

	// Owner of A tries to masquerade by setting X-Tenant-ID to Tenant B -> 403 Forbidden (not a member)
	reqMasquerade := httptest.NewRequest("GET", "/admin/blog/"+postB.ID.String(), nil)
	reqMasquerade.Header.Set("Authorization", "Bearer token-a")
	reqMasquerade.Header.Set("X-Tenant-ID", tenantBID.String())
	recMasquerade := httptest.NewRecorder()
	r.ServeHTTP(recMasquerade, reqMasquerade)

	if recMasquerade.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden when pretending to belong to another tenant, got: %d", recMasquerade.Code)
	}
}
