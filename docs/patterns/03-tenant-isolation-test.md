# الگوی ۰۳ — تست ایزولاسیون Tenant و تست‌های نگهبان

**کی استفاده کنم:** برای **هر منبع/جدول tenant-scoped جدید** یک تست ایزولاسیون بنویس. تست‌های نگهبان یک‌بار نوشته می‌شوند و برای همه جدول‌های آینده خودکار کار می‌کنند.
**مرتبط:** `01-tenant-scoped-module.md`، `02-migration-rls.md`، `ANTI_PATTERNS.md` (AP-01 تا AP-04، AP-19).

> **قانون:** این تست‌ها **هرگز `Skip` نمی‌شوند، ضعیف نمی‌شوند و حذف نمی‌شوند** تا سبز شوند. اگر fail شدند، یعنی ایزولاسیون شکسته است؛ علت را درست کن.

---

## ۰. پیش‌نیاز: دو اتصال به دیتابیس تست

| متغیر env | نقش | کاربرد |
| --- | --- | --- |
| `TEST_ADMIN_DATABASE_URL` | `migrator` | فقط برای ساخت Tenant (جدول `tenants` بدون RLS) |
| `TEST_APP_DATABASE_URL` | `app_user` | **همان نقش پروداکشن**؛ تمام تست‌های ایزولاسیون با این اجرا می‌شوند |

- تست با superuser بی‌معنی است، چون RLS را دور می‌زند (AP-02).
- قبل از اجرای تست‌ها، Migrationها روی دیتابیس تست اعمال شده باشد (`make migrate-up` روی DB تست؛ در `make test-db` قرار بده).
- نمونه مقادیر در `.env.test.example` (با رمز فرضی؛ فایل واقعی commit نشود).
- هر تست Tenantهای **تازه** می‌سازد و به خالی‌بودن دیتابیس تکیه نمی‌کند.

---

## ۱. هلپر تست — `internal/platform/testenv/testenv.go`

```go
package testenv

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Env struct {
	Admin *pgxpool.Pool // نقش migrator
	App   *pgxpool.Pool // نقش app_user (مثل پروداکشن)
}

func New(t *testing.T) *Env {
	t.Helper()
	adminURL := os.Getenv("TEST_ADMIN_DATABASE_URL")
	appURL := os.Getenv("TEST_APP_DATABASE_URL")
	if adminURL == "" || appURL == "" {
		// عمداً Fatal و نه Skip: تست ایزولاسیون نباید بی‌صدا رد شود.
		t.Fatal("TEST_ADMIN_DATABASE_URL and TEST_APP_DATABASE_URL must be set")
	}

	ctx := context.Background()
	admin, err := pgxpool.New(ctx, adminURL)
	if err != nil {
		t.Fatalf("connect admin: %v", err)
	}
	t.Cleanup(admin.Close)

	app, err := pgxpool.New(ctx, appURL)
	if err != nil {
		t.Fatalf("connect app: %v", err)
	}
	t.Cleanup(app.Close)

	return &Env{Admin: admin, App: app}
}

// NewTenant یک فروشگاه تازه می‌سازد و شناسه‌اش را برمی‌گرداند.
// ستون‌ها را با schema واقعی جدول tenants (P0.2.3) هماهنگ کن.
func (e *Env) NewTenant(t *testing.T) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := e.Admin.QueryRow(context.Background(),
		`INSERT INTO tenants (name) VALUES ($1) RETURNING id`,
		"test-"+uuid.NewString(),
	).Scan(&id)
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	return id
}
```

---

## ۲. تست ایزولاسیون هر منبع — `internal/category/isolation_test.go`

برای هر منبع جدید این فایل را کپی کن و نام‌ها را عوض کن. **همه‌ی این سناریوها باید باشند.**

```go
package category_test

import (
	"context"
	"errors"
	"testing"

	"commerce/api/internal/category"
	"commerce/api/internal/platform/db"
	"commerce/api/internal/platform/tenantctx"
	"commerce/api/internal/platform/testenv"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestCategory_TenantIsolation(t *testing.T) {
	env := testenv.New(t)
	store := db.NewStore(env.App)
	svc := category.NewService(store)

	tenantA, tenantB := env.NewTenant(t), env.NewTenant(t)
	ctxA := tenantctx.With(context.Background(), tenantA)
	ctxB := tenantctx.With(context.Background(), tenantB)

	// --- ۱) A یک دسته می‌سازد
	catA, err := svc.Create(ctxA, category.CreateInput{Name: "Phones", Slug: "phones"})
	if err != nil {
		t.Fatalf("A create: %v", err)
	}

	// --- ۲) B نمی‌تواند با slug بخواند
	if _, err := svc.GetBySlug(ctxB, "phones"); !errors.Is(err, category.ErrNotFound) {
		t.Fatalf("B must not read A's category: got err=%v", err)
	}

	// --- ۳) لیست B شامل داده A نیست
	items, err := svc.List(ctxB, 100, 0)
	if err != nil {
		t.Fatalf("B list: %v", err)
	}
	for _, it := range items {
		if it.ID == catA.ID {
			t.Fatal("B's list contains A's category")
		}
	}

	// --- ۴) یکتایی slug فقط داخل هر Tenant است
	if _, err := svc.Create(ctxB, category.CreateInput{Name: "Phones B", Slug: "phones"}); err != nil {
		t.Fatalf("B must be able to reuse slug 'phones': %v", err)
	}
	if _, err := svc.Create(ctxA, category.CreateInput{Name: "Dup", Slug: "phones"}); !errors.Is(err, category.ErrSlugTaken) {
		t.Fatalf("A duplicate slug must fail with ErrSlugTaken: got %v", err)
	}

	// --- ۵) حمله با SQL خام در context B: UPDATE و DELETE روی ردیف A باید بی‌اثر باشند
	var updated, deleted int64
	err = store.WithTenantTx(ctxB, func(tx pgx.Tx, _ uuid.UUID) error {
		tag, err := tx.Exec(context.Background(), `UPDATE categories SET name = 'hacked' WHERE id = $1`, catA.ID)
		if err != nil {
			return err
		}
		updated = tag.RowsAffected()
		tag, err = tx.Exec(context.Background(), `DELETE FROM categories WHERE id = $1`, catA.ID)
		if err != nil {
			return err
		}
		deleted = tag.RowsAffected()
		return nil
	})
	if err != nil {
		t.Fatalf("raw attack tx: %v", err)
	}
	if updated != 0 || deleted != 0 {
		t.Fatalf("B modified A's row: updated=%d deleted=%d", updated, deleted)
	}
	if got, err := svc.GetBySlug(ctxA, "phones"); err != nil || got.Name != "Phones" {
		t.Fatalf("A's category was altered: %+v err=%v", got, err)
	}

	// --- ۶) B نمی‌تواند ردیفی با tenant_id دیگری بنویسد (WITH CHECK)
	err = store.WithTenantTx(ctxB, func(tx pgx.Tx, _ uuid.UUID) error {
		_, err := tx.Exec(context.Background(),
			`INSERT INTO categories (tenant_id, name, slug) VALUES ($1, 'evil', 'evil')`, tenantA)
		return err
	})
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "42501" { // RLS WITH CHECK violation
		t.Fatalf("insert with foreign tenant_id must be rejected by RLS (42501): got %v", err)
	}

	// --- ۷) بدون tenant در context: خطا (نه داده)
	if _, err := svc.List(context.Background(), 10, 0); !errors.Is(err, db.ErrNoTenant) {
		t.Fatalf("no tenant in ctx must return ErrNoTenant: got %v", err)
	}

	// --- ۸) اتصال خام بدون ست‌کردن tenant: هیچ ردیفی دیده نمی‌شود (fail-closed)
	var n int
	if err := env.App.QueryRow(context.Background(), `SELECT count(*) FROM categories`).Scan(&n); err != nil {
		t.Fatalf("raw count: %v", err)
	}
	if n != 0 {
		t.Fatalf("without app.tenant_id no rows must be visible, got %d", n)
	}
}
```

---

## ۳. تست‌های نگهبان — `internal/platform/db/guard_test.go`

اینها یک‌بار نوشته می‌شوند و **هر جدول یا نقشِ اشتباهِ آینده را خودکار می‌گیرند.**

```go
package db_test

import (
	"context"
	"testing"

	"commerce/api/internal/platform/testenv"
)

// جدول‌هایی که tenant_id دارند ولی عمداً RLS ندارند.
// اضافه‌کردن به این فهرست فقط با تأیید صریح صاحب پروژه مجاز است.
var nonRLSAllowlist = map[string]bool{
	// "domains": true, // مثال؛ تصمیم در P1.3
}

// نقش برنامه نباید superuser باشد و نباید RLS را دور بزند (AP-02).
func TestGuard_AppRoleCannotBypassRLS(t *testing.T) {
	env := testenv.New(t)
	var super, bypass bool
	err := env.App.QueryRow(context.Background(),
		`SELECT rolsuper, rolbypassrls FROM pg_roles WHERE rolname = current_user`,
	).Scan(&super, &bypass)
	if err != nil {
		t.Fatalf("query role: %v", err)
	}
	if super || bypass {
		t.Fatalf("app role must be NOSUPERUSER and NOBYPASSRLS (super=%v bypass=%v)", super, bypass)
	}
}

// هر جدول دارای tenant_id باید RLS فعال + اجباری و حداقل یک policy داشته باشد (AP-04).
func TestGuard_AllTenantTablesHaveForcedRLS(t *testing.T) {
	env := testenv.New(t)
	rows, err := env.App.Query(context.Background(), `
		SELECT c.relname
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		JOIN pg_attribute a ON a.attrelid = c.oid AND a.attname = 'tenant_id' AND NOT a.attisdropped
		WHERE n.nspname = 'public'
		  AND c.relkind IN ('r', 'p')
		  AND (NOT c.relrowsecurity
		       OR NOT c.relforcerowsecurity
		       OR NOT EXISTS (SELECT 1 FROM pg_policy p WHERE p.polrelid = c.oid))
		ORDER BY c.relname`)
	if err != nil {
		t.Fatalf("query tables: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan: %v", err)
		}
		if !nonRLSAllowlist[name] {
			t.Errorf("table %q has tenant_id but RLS is not enabled+forced with a policy", name)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}
}

// نقش برنامه باید به جدول‌ها دسترسی پایه داشته باشد (تنظیم‌نشدن ALTER DEFAULT PRIVILEGES را می‌گیرد).
func TestGuard_AppRoleHasBasicPrivileges(t *testing.T) {
	env := testenv.New(t)
	rows, err := env.App.Query(context.Background(), `
		SELECT c.relname
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'public' AND c.relkind IN ('r', 'p')
		  AND NOT has_table_privilege(current_user, c.oid, 'SELECT')
		ORDER BY c.relname`)
	if err != nil {
		t.Fatalf("query privileges: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan: %v", err)
		}
		t.Errorf("app_user has no SELECT privilege on table %q (check default privileges / grants)", name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}
}
```

---

## ۴. «تست خودِ تست» (یک‌بار، دستی، بعد از نوشتن هر تست ایزولاسیون)

تستی که هیچ‌وقت fail نشود، چیزی را اثبات نمی‌کند (AP-19). بعد از نوشتن، یک‌بار **عمداً RLS را بشکن** و ببین تست fail می‌شود:

```sql
-- روی دیتابیس تست، با نقش migrator:
ALTER TABLE categories NO FORCE ROW LEVEL SECURITY;
ALTER TABLE categories DISABLE ROW LEVEL SECURITY;
```
- `go test ./internal/category/...` و تست نگهبان باید **fail** شوند.
- سپس دوباره `ENABLE` و `FORCE` کن (یا `make migrate-down && make migrate-up`) و مطمئن شو سبز شد.
- در گزارش تسک بنویس که این بررسی را انجام دادی.

---

## ۵. چک‌لیست تست برای هر منبع جدید

- [ ] فایل `isolation_test.go` برای منبع (هر هشت سناریوی بخش ۲ یا معادلشان)
- [ ] تست با `app_user` اجرا می‌شود، نه superuser
- [ ] Tenantها در تست تازه ساخته می‌شوند
- [ ] تست خودِ تست (بخش ۴) انجام و در گزارش ذکر شده
- [ ] تست‌های نگهبان سبزند (و جدول جدید را بدون استثنای غیرمجاز می‌پذیرند)
- [ ] هیچ `t.Skip`، هیچ assertion شل‌شده
- [ ] برای lookup اولیه و پردازش Outbox، علاوه بر تست API، محدودهٔ کم‌اختیار نقش‌های D-28 با PostgreSQL واقعی آزموده شده است. lookup فقط دامنهٔ تأییدشده یا عضویت هویت احرازشده را می‌بیند. دسترسی Worker محدود به عملیات صف می‌ماند.

## ۶. این‌ها را عوض نکن

- اجرای تست‌ها با `app_user`؛ `Fatal` به‌جای `Skip` وقتی env ست نیست.
- سناریوهای ۵ و ۶ (حمله با SQL خام) و ۸ (fail-closed بدون tenant).
- تست‌های نگهبان و فهرست استثنای خالی (بدون تأیید من چیزی به آن اضافه نکن).
