# الگوی ۰۱ — ماژول Tenant-Scoped (Handler → Service → sqlc → `WithTenantTx`)

**کی استفاده کنم:** هر endpoint یا سرویسی که داده‌ی یک Tenant (محصول، دسته‌بندی، سفارش، مشتری، ...) را می‌خواند یا می‌نویسد.
**مثال این الگو:** منبع `category` (ساده‌ترین منبع). برای منبع دیگر، نام‌ها را عوض کن، **ساختار را نه.**
**مرتبط:** `02-migration-rls.md` (جدول)، `03-tenant-isolation-test.md` (تست)، `ANTI_PATTERNS.md` (AP-01، AP-03، AP-13، AP-18).

---

## ۱. ساختار فایل‌ها

```
apps/psychology-api/
├── sqlc.yaml
├── migrations/                          # الگوی ۰۲
└── internal/
    ├── platform/
    │   ├── tenantctx/tenantctx.go       # خواندن/نوشتن tenant در context
    │   ├── db/store.go                  # Store.WithTenantTx  ← تنها در دسترسی به داده tenant-scoped
    │   └── httpx/httpx.go               # JSON، Error، Decode
    └── category/
        ├── queries.sql                  # Queryهای sqlc
        ├── categorydb/                  # تولیدشده توسط sqlc (دستی ویرایش نکن)
        ├── service.go
        └── http.go
```

---

## ۲. `internal/platform/tenantctx/tenantctx.go`

```go
package tenantctx

import (
	"context"

	"github.com/google/uuid"
)

type key struct{}

// With فقط توسط middleware تشخیص tenant صدا زده می‌شود:
//   - Storefront: Domain Resolver (از Host تأییدشده)
//   - Admin: middleware عضویت (از Membership احرازشده)
// در هیچ handler یا service دیگری صدا نزن.
func With(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, key{}, id)
}

// ID شناسه tenant را برمی‌گرداند. ok=false یعنی tenant تشخیص داده نشده.
func ID(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(key{}).(uuid.UUID)
	return id, ok && id != uuid.Nil
}
```

---

## ۳. `internal/platform/db/store.go` ← مهم‌ترین فایل

```go
package db

import (
	"context"
	"errors"
	"fmt"

	"commerce/api/internal/platform/tenantctx"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNoTenant = errors.New("db: no tenant in context")

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// WithTenantTx یک تراکنش باز می‌کند، tenant را برای RLS ست می‌کند و fn را اجرا می‌کند.
// اگر fn خطا بدهد تراکنش Rollback می‌شود، وگرنه Commit.
//
// قواعد:
//   - tenantID فقط از context گرفته می‌شود (هرگز از ورودی کاربر).
//   - set_config(..., true) یعنی «LOCAL»: فقط تا پایان همین تراکنش (امن برای connection pool).
//   - هر دسترسی به جدول tenant-scoped فقط داخل این تابع.
func (s *Store) WithTenantTx(ctx context.Context, fn func(tx pgx.Tx, tenantID uuid.UUID) error) error {
	tenantID, ok := tenantctx.ID(ctx)
	if !ok {
		return ErrNoTenant
	}
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", tenantID.String()); err != nil {
			return fmt.Errorf("set tenant: %w", err)
		}
		return fn(tx, tenantID)
	})
}

// Direct فقط برای دسترسی‌های غیر tenant-scoped مجاز در schema مصوب است.
// دسترسی اولیه به domains و memberships از مسیر lookup محدود با RLS فعال انجام می‌شود.
func (s *Store) Direct() *pgxpool.Pool { return s.pool }
```

---

## ۴. `internal/platform/httpx/httpx.go`

```go
package httpx

import (
	"encoding/json"
	"net/http"
)

type errorBody struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	// خطای نوشتن روی connection کلاینت قابل‌اقدام نیست؛ تنها جایی که نادیده‌گرفتن خطا مجاز است.
	_ = json.NewEncoder(w).Encode(v)
}

// Error فرمت یکسان خطا (طبق قرارداد OpenAPI پروژه). message را برای کاربر امن و بدون جزئیات داخلی بنویس.
func Error(w http.ResponseWriter, status int, code, message string) {
	var b errorBody
	b.Error.Code = code
	b.Error.Message = message
	JSON(w, status, b)
}

// Decode بدنه JSON را با سقف حجم و رد کردن فیلد ناشناخته می‌خواند.
func Decode(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // ۱ مگابایت
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}
```

---

## ۵. `sqlc.yaml` (هر ماژول یک ورودی)

```yaml
version: "2"
sql:
  - engine: "postgresql"
    schema: "migrations"                       # sqlc فایل‌های .down.sql را نادیده می‌گیرد
    queries: "internal/category/queries.sql"
    gen:
      go:
        package: "categorydb"
        out: "internal/category/categorydb"
        sql_package: "pgx/v5"
        overrides:
          - db_type: "uuid"
            go_type: "github.com/google/uuid.UUID"
```

بعد از هر تغییر در `queries.sql` یا Migration: `sqlc generate` را اجرا کن و **فایل‌های تولیدشده را دستی ویرایش نکن.**

---

## ۶. `internal/category/queries.sql`

```sql
-- name: CreateCategory :one
INSERT INTO categories (tenant_id, name, slug)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetCategoryBySlug :one
SELECT * FROM categories
WHERE tenant_id = $1 AND slug = $2 AND deleted_at IS NULL;

-- name: ListCategories :many
SELECT * FROM categories
WHERE tenant_id = $1 AND deleted_at IS NULL
ORDER BY name
LIMIT $2 OFFSET $3;
```

**قاعده:** هر Query علاوه بر RLS، **فیلتر صریح `tenant_id = $1`** هم دارد (دفاع دو لایه + استفاده درست از ایندکس). RLS شبکه‌ی ایمنی است، نه جایگزین فیلتر.

---

## ۷. `internal/category/service.go`

```go
package category

import (
	"context"
	"errors"

	"commerce/api/internal/category/categorydb"
	"commerce/api/internal/platform/db"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrNotFound  = errors.New("category: not found")
	ErrSlugTaken = errors.New("category: slug already exists")
)

type Category struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
	Slug string    `json:"slug"`
}

type CreateInput struct {
	Name string
	Slug string
}

type Service struct {
	store *db.Store
}

func NewService(store *db.Store) *Service { return &Service{store: store} }

func (s *Service) Create(ctx context.Context, in CreateInput) (Category, error) {
	var out Category
	err := s.store.WithTenantTx(ctx, func(tx pgx.Tx, tenantID uuid.UUID) error {
		row, err := categorydb.New(tx).CreateCategory(ctx, categorydb.CreateCategoryParams{
			TenantID: tenantID,
			Name:     in.Name,
			Slug:     in.Slug,
		})
		if err != nil {
			return mapErr(err)
		}
		out = toCategory(row)
		return nil
	})
	return out, err
}

func (s *Service) GetBySlug(ctx context.Context, slug string) (Category, error) {
	var out Category
	err := s.store.WithTenantTx(ctx, func(tx pgx.Tx, tenantID uuid.UUID) error {
		row, err := categorydb.New(tx).GetCategoryBySlug(ctx, categorydb.GetCategoryBySlugParams{
			TenantID: tenantID,
			Slug:     slug,
		})
		if err != nil {
			return mapErr(err)
		}
		out = toCategory(row)
		return nil
	})
	return out, err
}

func (s *Service) List(ctx context.Context, limit, offset int32) ([]Category, error) {
	out := []Category{} // نه nil، تا JSON به‌صورت [] برگردد
	err := s.store.WithTenantTx(ctx, func(tx pgx.Tx, tenantID uuid.UUID) error {
		rows, err := categorydb.New(tx).ListCategories(ctx, categorydb.ListCategoriesParams{
			TenantID: tenantID,
			Limit:    limit,
			Offset:   offset,
		})
		if err != nil {
			return err
		}
		for _, r := range rows {
			out = append(out, toCategory(r))
		}
		return nil
	})
	return out, err
}

func toCategory(r categorydb.Category) Category {
	return Category{ID: r.ID, Name: r.Name, Slug: r.Slug}
}

// mapErr خطای دیتابیس را به خطای دامنه ترجمه می‌کند. خطای ناشناخته را با context برمی‌گرداند.
func mapErr(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation
		return ErrSlugTaken
	}
	return err
}
```

---

## ۸. `internal/category/http.go`

```go
package category

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"commerce/api/internal/platform/httpx"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
)

var validate = validator.New()

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) MountPublic(r chi.Router) {
	r.Get("/categories", h.list)
	r.Get("/categories/{slug}", h.get)
}

func (h *Handler) MountAdmin(r chi.Router) {
	r.Post("/categories", h.create)
}

type createRequest struct {
	Name string `json:"name" validate:"required,min=1,max=200"`
	Slug string `json:"slug" validate:"required,min=1,max=200"`
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := httpx.Decode(w, r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_json", "invalid request body")
		return
	}
	if err := validate.Struct(req); err != nil {
		httpx.Error(w, http.StatusUnprocessableEntity, "validation_failed", "name and slug are required")
		return
	}

	// توجه: tenant هیچ‌جا از r خوانده نمی‌شود؛ Service آن را از context می‌گیرد.
	c, err := h.svc.Create(r.Context(), CreateInput{Name: req.Name, Slug: req.Slug})
	if err != nil {
		h.fail(w, r, "create category", err)
		return
	}
	httpx.JSON(w, http.StatusCreated, c)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	c, err := h.svc.GetBySlug(r.Context(), chi.URLParam(r, "slug"))
	if err != nil {
		h.fail(w, r, "get category", err)
		return
	}
	httpx.JSON(w, http.StatusOK, c)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	limit := intParam(r, "limit", 20, 1, 100)
	offset := intParam(r, "offset", 0, 0, 1_000_000)
	items, err := h.svc.List(r.Context(), int32(limit), int32(offset))
	if err != nil {
		h.fail(w, r, "list categories", err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

// fail خطای دامنه را به status مناسب ترجمه می‌کند؛ خطای ناشناخته را لاگ و ۵۰۰ می‌دهد
// (بدون افشای جزئیات داخلی به کلاینت).
func (h *Handler) fail(w http.ResponseWriter, r *http.Request, op string, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		httpx.Error(w, http.StatusNotFound, "not_found", "category not found")
	case errors.Is(err, ErrSlugTaken):
		httpx.Error(w, http.StatusConflict, "slug_taken", "slug already exists")
	default:
		slog.ErrorContext(r.Context(), op, "err", err)
		httpx.Error(w, http.StatusInternalServerError, "internal", "internal error")
	}
}

func intParam(r *http.Request, name string, def, min, max int) int {
	v, err := strconv.Atoi(r.URL.Query().Get(name))
	if err != nil || v < min {
		return def
	}
	if v > max {
		return max
	}
	return v
}
```

---

## ۹. اتصال در `cmd/server/main.go` (خلاصه)

`publicRoutes` و `adminRoutes` گروه‌هایی هستند که قرارداد OpenAPI پروژه تعیین می‌کند. نام middlewareهای زیر نقش هر بررسی را در نمونه نشان می‌دهد.

```go
store := db.NewStore(pool)
handler := category.NewHandler(category.NewService(store))

publicRoutes.Group(func(r chi.Router) {
	r.Use(resolveTenantFromVerifiedHost)
	handler.MountPublic(r)
})

adminRoutes.Group(func(r chi.Router) {
	r.Use(authenticateAdmin)
	r.Use(resolveTenantFromAuthenticatedMembership)
	r.Use(requireCategoryWritePermission)
	handler.MountAdmin(r)
})
```

- **Public** — Host تأییدشده Tenant را تعیین می‌کند. در این نمونه گروه Public فقط عملیات خواندن عمومی دسته‌ها را دارد.
- **Admin** — ابتدا هویت کاربر احراز می‌شود. سپس عضویت او در مطب انتخاب‌شده بررسی و Tenant معتبر در context قرار می‌گیرد. در پایان، مجوز همان عملیات برای همان مطب بررسی می‌شود. نمونهٔ Admin بالا فقط ساخت دسته‌بندی را نشان می‌دهد. برای هر عملیات مدیریتی بعدی مجوز همان عملیات را بررسی کن.

---

## ۱۰. چک‌لیست ساخت یک منبع جدید با این الگو

- [ ] Migration جدول (الگوی ۰۲): `tenant_id` + RLS + Grant
- [ ] `queries.sql`: هر Query فیلتر `tenant_id = $1` دارد؛ `sqlc generate` اجرا شده
- [ ] Service: **همه** دسترسی‌ها داخل `WithTenantTx`؛ `tenantID` از callback است نه از ورودی
- [ ] خطاها: با `mapErr` ترجمه می‌شوند؛ هیچ خطایی نادیده گرفته نمی‌شود
- [ ] Handler: فقط decode/validate/status؛ tenant را از `r` نمی‌خواند
- [ ] خواندن عمومی دسته‌ها فقط در گروه Public از Host تأییدشده Tenant می‌گیرد.
- [ ] عملیات مدیریتی فقط در گروه Admin و پس از احراز هویت، عضویت و مجوز همان عملیات ثبت شده است.
- [ ] تست HTTP نشان می‌دهد خواندن عمومی در مطب معتبر کار می‌کند و ساخت دسته‌بندی در گروه Public ثبت نشده است.
- [ ] تست HTTP نشان می‌دهد ساخت دسته‌بندی تنها با عضویت و مجوز لازم موفق می‌شود. نبود هویت، نبود عضویت در مطب انتخاب‌شده یا نبود مجوز، تغییری در داده ایجاد نمی‌کند.
- [ ] OpenAPI (`packages/contracts`) به‌روز شده
- [ ] تست ایزولاسیون برای منبع (الگوی ۰۳)
- [ ] `go build ./... && go vet ./... && go test ./...` سبز است

## ۱۱. این‌ها را عوض نکن

- `Store.WithTenantTx` و `set_config(..., true)`.
- گرفتن tenant فقط از `tenantctx`.
- فیلتر صریح `tenant_id` در Queryها.
- الگوی `mapErr` و عدم افشای خطای داخلی به کلاینت.
