package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"psychology/api/internal/platform/db"
	"psychology/api/internal/platform/tenantctx"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func checkBlogCatalog(t *testing.T, store *db.Store, tenant, owner uuid.UUID, host, otherHost, token string, foreignCover uuid.UUID,
	serve func(http.ResponseWriter, *http.Request)) {
	t.Helper()
	ctx := context.Background()
	var cover uuid.UUID
	if err := store.WithTenantTx(tenantctx.With(ctx, tenant), func(tx pgx.Tx, tid uuid.UUID) error {
		return tx.QueryRow(ctx, `INSERT INTO media_assets
			(tenant_id,storage_key,original_name,mime_type,size_bytes,access_class,validation_status,uploaded_by)
			VALUES ($1,$2,'article.png','image/png',128,'public','verified',$3) RETURNING id`,
			tid, tid.String()+"/test-article.png", owner).Scan(&cover)
	}); err != nil {
		t.Fatal(err)
	}

	request := func(method, requestHost, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Host = requestHost
		r.Header.Set("X-Forwarded-Host", host)
		if strings.HasPrefix(path, "/v1/admin/") {
			r.Header.Set("Authorization", "Bearer "+token)
			r.Header.Set("X-Tenant-ID", tenant.String())
			r.Header.Set("Content-Type", "application/json")
		}
		rec := httptest.NewRecorder()
		serve(rec, r)
		return rec
	}
	admin := "/v1/admin/blog"
	list := "/v1/public/blog"
	detail := list + "/blog-catalog-a"
	image := "/v1/public/media/" + cover.String() + "/image"
	stream := "/v1/public/media/" + cover.String() + "/stream-url"
	body := func(coverID uuid.UUID) string {
		return `{"title":"مقالهٔ مطب الف","slug":"blog-catalog-a","author_name":"  نویسندهٔ   مطب الف ",` +
			`"category":"روابط","is_featured":true,"content_blocks":[{"type":"paragraph","data":{"text":"متن مقاله"}}],"cover_media_id":"` + coverID.String() + `"}`
	}
	t.Run("blog metadata can only be published by its clinic with verified media", func(t *testing.T) {
		if res := request(http.MethodPost, host, admin, body(foreignCover)); res.Code != http.StatusBadRequest {
			t.Errorf("foreign cover must be rejected: %d %s", res.Code, res.Body.String())
		}
		for _, path := range []string{image, stream} {
			if res := request(http.MethodGet, host, path, ""); res.Code != http.StatusNotFound {
				t.Errorf("unpublished cover URL issued: %d", res.Code)
			}
		}
		res := request(http.MethodPost, host, admin, body(cover))
		var article struct {
			ID uuid.UUID `json:"id"`
		}
		if err := json.Unmarshal(res.Body.Bytes(), &article); err != nil || res.Code != http.StatusCreated || article.ID == uuid.Nil {
			t.Fatalf("create blog draft: %d %s, %v", res.Code, res.Body.String(), err)
		}
		if res := request(http.MethodGet, host, detail, ""); res.Code != http.StatusNotFound {
			t.Fatalf("draft detail is accessible: %d %s", res.Code, res.Body.String())
		}
		adminPost := admin + "/" + article.ID.String()
		if res := request(http.MethodPut, host, adminPost, `{"category":"  روابط "}`); res.Code != http.StatusOK ||
			!strings.Contains(res.Body.String(), `"slug":"blog-catalog-a"`) ||
			!strings.Contains(res.Body.String(), "متن مقاله") || !strings.Contains(res.Body.String(), cover.String()) {
			t.Fatalf("metadata edit changed existing article content: %d %s", res.Code, res.Body.String())
		}
		if res := request(http.MethodPut, host, adminPost, `{"category":"`+strings.Repeat("ا", 81)+`"}`); res.Code != http.StatusBadRequest {
			t.Fatalf("long category was accepted: %d %s", res.Code, res.Body.String())
		}
		if res := request(http.MethodGet, host, image, ""); res.Code != http.StatusNotFound {
			t.Errorf("draft cover URL issued: %d", res.Code)
		}

		if res := request(http.MethodPost, host, adminPost+"/publish", ""); res.Code != http.StatusOK {
			t.Fatalf("publish blog post: %d %s", res.Code, res.Body.String())
		}
		for _, path := range []string{list, detail} {
			res := request(http.MethodGet, host, path, "")
			for _, field := range []string{`"author_name":"نویسندهٔ مطب الف"`, `"category":"روابط"`, `"is_featured":true`, cover.String()} {
				if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), field) {
					t.Errorf("%s omitted %s: %d %s", path, field, res.Code, res.Body.String())
				}
			}
			if strings.Contains(res.Body.String(), `"tenant_id"`) || (path == list && !strings.HasPrefix(res.Body.String(), "[")) {
				t.Errorf("wrong public response shape: %s", res.Body.String())
			}
		}
		if res := request(http.MethodGet, otherHost, list, ""); strings.Contains(res.Body.String(), "blog-catalog-a") {
			t.Errorf("foreign blog item listed: %s", res.Body.String())
		}
		for _, path := range []string{detail, image, stream} {
			if res := request(http.MethodGet, otherHost, path, ""); res.Code != http.StatusNotFound {
				t.Errorf("foreign path %s: %d %s", path, res.Code, res.Body.String())
			}
		}
		if res := request(http.MethodGet, host, image, ""); res.Code != http.StatusFound || !strings.Contains(res.Header().Get("Cache-Control"), "no-store") {
			t.Errorf("public blog cover: %d %v", res.Code, res.Header())
		}
		if res := request(http.MethodPost, host, adminPost+"/unpublish", ""); res.Code != http.StatusOK {
			t.Fatalf("unpublish: %d %s", res.Code, res.Body.String())
		}
		for _, path := range []string{detail, image, stream} {
			if res := request(http.MethodGet, host, path, ""); res.Code != http.StatusNotFound {
				t.Errorf("unpublished path %s: %d %s", path, res.Code, res.Body.String())
			}
		}
		var invalidations int
		if err := store.WithTenantTx(tenantctx.With(ctx, tenant), func(tx pgx.Tx, tid uuid.UUID) error {
			return tx.QueryRow(ctx, `SELECT count(*) FROM outbox_events WHERE tenant_id=$1 AND event_type='blog.updated'
				AND payload->>'host'=$2 AND (payload->'paths') ? '/blog/blog-catalog-a'`, tid, host).Scan(&invalidations)
		}); err != nil || invalidations != 2 {
			t.Errorf("expected cache invalidations on publish and unpublish: %d, %v", invalidations, err)
		}
	})
}
