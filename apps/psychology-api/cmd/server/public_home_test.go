package main

import (
	"bytes"
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

func checkPublicHome(t *testing.T, store *db.Store, serve func(http.ResponseWriter, *http.Request),
	tenantA, tenantB, ownerA, ownerB uuid.UUID, hostA, hostB, token string) {
	t.Helper()
	ctx := context.Background()
	seed := func(tenant, owner uuid.UUID, slug string) (uuid.UUID, uuid.UUID) {
		t.Helper()
		var workshop, portrait uuid.UUID
		err := store.WithTenantTx(tenantctx.With(ctx, tenant), func(tx pgx.Tx, id uuid.UUID) error {
			if err := tx.QueryRow(ctx, `INSERT INTO media_assets
				(tenant_id, storage_key, original_name, mime_type, size_bytes, access_class, validation_status, uploaded_by)
				VALUES ($1,$2,'portrait.png','image/png',128,'public','verified',$3) RETURNING id`,
				id, id.String()+"/portrait.png", owner).Scan(&portrait); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `INSERT INTO courses (tenant_id,title,slug,instructor_name,price_rials,status,cover_media_id)
				VALUES ($1,$2,$2,'مدرس',12000000,'published',$3),($1,'hidden-course','hidden-course','مدرس',0,'draft',NULL),($1,'archived-course','archived-course','مدرس',0,'archived',NULL)`, id, slug, portrait); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `INSERT INTO blog_posts (tenant_id,title,slug,status,published_at)
				VALUES ($1,$2,$2,'published',now()),($1,'hidden-post','hidden-post','draft',NULL)`, id, slug); err != nil {
				return err
			}
			return tx.QueryRow(ctx, `INSERT INTO workshops (tenant_id,title,slug,instructor_name,starts_at,ends_at,location,registration_deadline,capacity,status)
				VALUES ($1,$2,$2,'مدرس',now()+interval '2 days',now()+interval '3 days','مطب',now()+interval '1 day',1,'published') RETURNING id`, id, slug).Scan(&workshop)
		})
		if err != nil {
			t.Fatal(err)
		}
		return workshop, portrait
	}
	workshopA, portraitA := seed(tenantA, ownerA, "published-a")
	_, portraitB := seed(tenantB, ownerB, "published-b")
	request := func(method, host, route, body string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, route, strings.NewReader(body))
		req.Host = host
		if method == http.MethodPut {
			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set("X-Tenant-ID", tenantA.String())
			req.Header.Set("Content-Type", "application/json")
		}
		rec := httptest.NewRecorder()
		serve(rec, req)
		return rec
	}
	t.Run("published home lists and destinations", func(t *testing.T) {
		for _, path := range []string{"/v1/public/courses", "/v1/public/workshops", "/v1/public/blog"} {
			for _, clinic := range []struct{ host, slug, other string }{
				{hostA, "published-a", "published-b"},
				{hostB, "published-b", "published-a"},
			} {
				res := request(http.MethodGet, clinic.host, path, "")
				body := res.Body.String()
				if res.Code != http.StatusOK || !strings.Contains(body, clinic.slug) ||
					strings.Contains(body, clinic.other) || strings.Contains(body, "hidden-") {
					t.Errorf("%s on %s: %d %s", path, clinic.host, res.Code, body)
				}
			}
		}
		for _, path := range []string{"/v1/public/courses/published-a", "/v1/public/blog/published-a", "/v1/public/workshops/" + workshopA.String()} {
			if res := request(http.MethodGet, hostA, path, ""); res.Code != http.StatusOK {
				t.Errorf("public destination %s: %d %s", path, res.Code, res.Body.String())
			}
			if res := request(http.MethodGet, hostB, path, ""); res.Code != http.StatusNotFound {
				t.Errorf("foreign destination %s: %d %s", path, res.Code, res.Body.String())
			}
		}
		if res := request(http.MethodGet, hostA, "/v1/public/courses/archived-course", ""); res.Code != http.StatusNotFound {
			t.Errorf("archived course exposed: %d %s", res.Code, res.Body.String())
		}

	})
	t.Run("education topics use the same published host-scoped records", func(t *testing.T) {
		var courseA uuid.UUID
		err := store.WithTenantTx(tenantctx.With(ctx, tenantA), func(tx pgx.Tx, id uuid.UUID) error {
			return tx.QueryRow(ctx, "SELECT id FROM courses WHERE tenant_id=$1 AND slug='published-a'", id).Scan(&courseA)
		})
		if err != nil {
			t.Fatal(err)
		}
		for _, item := range []struct {
			resource, id, detail, body string
		}{
			{"courses", courseA.String(), "published-a", `{"topic":"  تنظیم   هیجان "}`},
			{"workshops", workshopA.String(), workshopA.String(), `{"topic":"  تنظیم   هیجان "}`},
		} {
			path := "/v1/admin/" + item.resource + "/" + item.id
			rec := request(http.MethodPut, hostA, path, item.body)
			if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"topic":"تنظیم هیجان"`) {
				t.Errorf("save %s topic: %d %s", item.resource, rec.Code, rec.Body.String())
				continue
			}
			if item.resource == "courses" && (!strings.Contains(rec.Body.String(), `"price_rials":12000000`) ||
				!strings.Contains(rec.Body.String(), `"instructor_name":"مدرس"`)) {
				t.Errorf("saving only the course topic changed its instructor or price: %s", rec.Body.String())
			}
			oversize := `{"topic":"` + strings.Repeat("آ", 81) + `"}`
			if rec = request(http.MethodPut, hostA, path, oversize); rec.Code != http.StatusBadRequest {
				t.Errorf("invalid topic should be rejected for %s: %d %s", item.resource, rec.Code, rec.Body.String())
			}
			list := "/v1/public/" + item.resource
			for _, route := range []string{list, list + "/" + item.detail} {
				rec = request(http.MethodGet, hostA, route, "")
				if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"topic":"تنظیم هیجان"`) {
					t.Errorf("own published topic from %s: %d %s", route, rec.Code, rec.Body.String())
				}
				rec = request(http.MethodGet, hostB, route, "")
				if strings.Contains(rec.Body.String(), "تنظیم هیجان") {
					t.Errorf("foreign topic returned from %s: %s", route, rec.Body.String())
				}
			}
		}
		var invalidations int
		if err := store.WithTenantTx(tenantctx.With(ctx, tenantA), func(tx pgx.Tx, id uuid.UUID) error {
			return tx.QueryRow(ctx, `SELECT count(*) FROM outbox_events
				WHERE tenant_id=$1 AND event_type='education.updated' AND payload->>'host'=$2`, id, hostA).Scan(&invalidations)
		}); err != nil || invalidations != 2 {
			t.Errorf("expected two catalog invalidations on %s: %d, %v", hostA, invalidations, err)
		}

		err = store.WithTenantTx(tenantctx.With(ctx, tenantA), func(tx pgx.Tx, id uuid.UUID) error {
			_, err := tx.Exec(ctx, `INSERT INTO workshop_registrations
				(tenant_id, workshop_id, guest_name, guest_phone, status, guest_lookup_hash)
				VALUES ($1,$2,'Guest','09121234567','confirmed','fixture-lookup-hash')`, id, workshopA)
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
		full := request(http.MethodGet, hostA, "/v1/public/workshops/"+workshopA.String(), "")
		if full.Code != http.StatusOK || !strings.Contains(full.Body.String(), `"remaining_capacity":0`) ||
			!strings.Contains(full.Body.String(), `"is_registration_open":false`) {
			t.Errorf("expected filled workshop: %d %s", full.Code, full.Body.String())
		}
	})
	t.Run("private course cover is not advertised", func(t *testing.T) {
		var invalid uuid.UUID
		err := store.WithTenantTx(tenantctx.With(ctx, tenantA), func(tx pgx.Tx, id uuid.UUID) error {
			if err := tx.QueryRow(ctx, `INSERT INTO media_assets
				(tenant_id, storage_key, original_name, mime_type, size_bytes, access_class, validation_status, uploaded_by)
				VALUES ($1,$2,'private.png','image/png',128,'course_private','verified',$3) RETURNING id`,
				id, id.String()+"/private.png", ownerA).Scan(&invalid); err != nil {
				return err
			}
			_, err := tx.Exec(ctx, `INSERT INTO courses (tenant_id,title,slug,instructor_name,price_rials,status,cover_media_id)
				VALUES ($1,'cover-rejected','cover-rejected','مدرس',0,'published',$2)`, id, invalid)
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
		for _, route := range []string{"/v1/public/courses", "/v1/public/courses/cover-rejected"} {
			res := request(http.MethodGet, hostA, route, "")
			if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), "cover-rejected") ||
				strings.Contains(res.Body.String(), invalid.String()) {
				t.Errorf("invalid course cover on %s: %d %s", route, res.Code, res.Body.String())
			}
		}
	})
	t.Run("profile content and portrait isolation", func(t *testing.T) {
		profileBody := func(portrait uuid.UUID) string {
			return `{"title":"` + "کلینیک تخصصی دکتر شمس (به‌روزشده)" + `","phone":"02177778888","home_profile":{` +
				`"introduction":"معرفی عمومی یکتا","specialties":["اضطراب"],"credentials":["مدرک عمومی"],"office_hours":"شنبه ۱۰ تا ۱۲","portrait_media_id":"` + portrait.String() + `"}}`
		}
		if res := request(http.MethodPut, hostA, "/v1/admin/site", profileBody(portraitB)); res.Code != http.StatusBadRequest {
			t.Errorf("foreign portrait must be rejected: %d %s", res.Code, res.Body.String())
		}
		if res := request(http.MethodPut, hostA, "/v1/admin/site", profileBody(portraitA)); res.Code != http.StatusOK {
			t.Fatalf("save own public home profile: %d %s", res.Code, res.Body.String())
		}
		legacyBody := `{"title":"کلینیک تخصصی دکتر شمس (به‌روزشده)","phone":"02177778888"}`
		if res := request(http.MethodPut, hostA, "/v1/admin/site", legacyBody); res.Code != http.StatusOK {
			t.Fatalf("legacy profile update: %d %s", res.Code, res.Body.String())
		}
		for _, host := range []string{hostA, hostB} {
			res := request(http.MethodGet, host, "/v1/public/site", "")
			var data struct {
				Profile json.RawMessage `json:"home_profile"`
			}
			if err := json.Unmarshal(res.Body.Bytes(), &data); err != nil {
				t.Fatal(err)
			}
			onA := host == hostA
			if res.Code != http.StatusOK || bytes.Contains(data.Profile, []byte("معرفی عمومی یکتا")) != onA ||
				bytes.Contains(data.Profile, []byte(portraitA.String())) != onA {
				t.Errorf("public home profile for %s: %d %s", host, res.Code, res.Body.String())
			}
		}
		path := "/v1/public/media/" + portraitA.String() + "/image"
		res := request(http.MethodGet, hostA, path, "")
		if res.Code != http.StatusFound || res.Header().Get("Location") == "" || !strings.Contains(res.Header().Get("Cache-Control"), "no-store") {
			t.Errorf("public image redirect: %d %v", res.Code, res.Header())
		}
		if res := request(http.MethodGet, hostB, path, ""); res.Code != http.StatusNotFound {
			t.Errorf("foreign public image: %d %s", res.Code, res.Body.String())
		}
	})
	checkBlogCatalog(t, store, tenantA, ownerA, hostA, hostB, token, portraitB, serve)

}
