# الگوی ۰۲ — Migration با `tenant_id` + RLS + Grant

**کی استفاده کنم:** هر بار که جدول جدیدی می‌سازی، یا جدولی را تغییر می‌دهی.
**مرتبط:** `01-tenant-scoped-module.md`، `03-tenant-isolation-test.md`، `ANTI_PATTERNS.md` (AP-02، AP-04، AP-15، AP-16).

---

## ۰. پیش‌نیاز یک‌بار-مصرف: نقش‌های دیتابیس (زیرساخت، نه Migration)

برنامه با نقش کم‌اختیار اجرا می‌شود. lookup و Worker مطابق تصمیم‌های پروژه نقش جدا و محدود دارند.

| نقش | کار | RLS |
| --- | --- | --- |
| `migrator` | مالک جدول‌ها؛ فقط Migration با آن اجرا می‌شود | `NOBYPASSRLS` (با `FORCE RLS` تابع policy است) |
| `app_user` | نقش runtime API | `NOSUPERUSER NOBYPASSRLS` |

نمونه (محیط توسعه؛ فایل `infrastructure/docker/initdb/01-roles.sql`؛ **رمزها از env و در repo ثابت نیستند**):

```sql
CREATE ROLE migrator LOGIN NOSUPERUSER NOBYPASSRLS;
CREATE ROLE app_user LOGIN NOSUPERUSER NOBYPASSRLS;
-- رمز هر نقش با ALTER ROLE ... PASSWORD از env ست می‌شود (در اسکریپت init)

GRANT CONNECT ON DATABASE commerce TO app_user, migrator;
GRANT USAGE  ON SCHEMA public TO app_user;
GRANT CREATE ON SCHEMA public TO migrator;      -- از PostgreSQL 15 به بعد برای public لازم است

-- هر جدول/sequence که migrator بسازد، خودکار برای app_user دسترسی پایه بگیرد
ALTER DEFAULT PRIVILEGES FOR ROLE migrator IN SCHEMA public
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO app_user;
ALTER DEFAULT PRIVILEGES FOR ROLE migrator IN SCHEMA public
    GRANT USAGE, SELECT ON SEQUENCES TO app_user;
```

با این تنظیم، در Migrationها **لازم نیست** برای هر جدول `GRANT` بنویسی. (اگر `ALTER DEFAULT PRIVILEGES` تنظیم نشده باشد، تست نگهبان در الگوی ۰۳ خطا می‌دهد؛ آن را دستی دور نزن.)

---

## ۱. ساخت فایل Migration

```bash
migrate create -ext sql -dir apps/psychology-api/migrations -seq create_categories
# → 000003_create_categories.up.sql  و  000003_create_categories.down.sql
```

- شماره را دستی عوض نکن. Migrationِ merge‌شده را **هرگز ویرایش نکن** (AP-16)؛ تغییر = فایل جدید.
- هر Migration هم `up` دارد هم `down`.

---

## ۲. الگوی جدول tenant-scoped — `000003_create_categories.up.sql`

```sql
CREATE TABLE categories (
    id         uuid        PRIMARY KEY DEFAULT gen_random_uuid(),   -- نوع ID مطابق طرح‌وارهٔ پروژه
    tenant_id  uuid        NOT NULL REFERENCES tenants(id),
    name       text        NOT NULL,
    slug       text        NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);

-- یکتایی slug داخل هر Tenant (نه سراسری) و فقط برای ردیف‌های حذف‌نشده،
-- تا بعد از soft delete بشود دوباره از همان slug استفاده کرد.
CREATE UNIQUE INDEX categories_tenant_slug_uq
    ON categories (tenant_id, slug)
    WHERE deleted_at IS NULL;

-- RLS: فعال + اجباری (مالک جدول هم تابع policy باشد)
ALTER TABLE categories ENABLE ROW LEVEL SECURITY;
ALTER TABLE categories FORCE  ROW LEVEL SECURITY;

-- اگر app.tenant_id ست نشده باشد: NULL ← هیچ سطری دیده یا نوشته نمی‌شود (امن).
CREATE POLICY tenant_isolation ON categories
    USING      (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);
```

**`000003_create_categories.down.sql`:**

```sql
DROP TABLE categories;   -- policy و ایندکس‌ها همراه جدول حذف می‌شوند
```

### چرا این جزئیات؟

| جزئیات | دلیل |
| --- | --- |
| `tenant_id NOT NULL` + FK | هیچ ردیفی بدون مالک وجود ندارد |
| `current_setting(..., true)` | بدون ست‌شدن tenant، خطا نمی‌دهد و `NULL` می‌دهد |
| `NULLIF(..., '')` | بعد از پایان تراکنش، مقدار ممکن است `''` شود؛ cast خالی به uuid خطا می‌دهد |
| `FORCE ROW LEVEL SECURITY` | مالک جدول هم تابع RLS باشد |
| `USING` **و** `WITH CHECK` | `USING` خواندن/تغییر را محدود می‌کند؛ `WITH CHECK` جلوی INSERT/UPDATE با `tenant_id` دیگر را می‌گیرد |
| ایندکس یکتا شامل `tenant_id` | یکتایی در هر Tenant + ایندکس جستجوی tenant-aware (AP-15) |
| `deleted_at` + ایندکس partial | soft delete بدون قفل‌شدن slug |

---

## ۳. جدول فرزند: جلوگیری از اشاره‌ی بین‌Tenantی (Composite FK)

**مشکل:** `order_items` ممکن است به `order_id` Tenant دیگر اشاره کند. کنترل‌های Referential Integrity دیتابیس RLS را **دور می‌زنند**، پس خود RLS جلوی این را نمی‌گیرد.
**راه‌حل:** جدول والد یک `UNIQUE (tenant_id, id)` دارد و فرزند با کلید ترکیبی به آن اشاره می‌کند.

```sql
-- روی جدول والد (در Migration خود orders)
ALTER TABLE orders ADD CONSTRAINT orders_tenant_id_id_uq UNIQUE (tenant_id, id);

-- جدول فرزند: tenant_id خودش را دارد (حتی اگر از والد قابل‌استخراج است) و RLS مستقل
CREATE TABLE order_items (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id  uuid NOT NULL REFERENCES tenants(id),
    order_id   uuid NOT NULL,
    -- ... ستون‌های Snapshot (AP-08)
    FOREIGN KEY (tenant_id, order_id) REFERENCES orders (tenant_id, id)
);
-- + ENABLE/FORCE RLS و policy مثل بخش ۲
```

قاعده: **هر جدول tenant-scoped ستون `tenant_id` و policy خودش را دارد**، حتی جدول فرزند.

---

## ۴. تغییر جدول موجود

- ستون جدید روی جدول پر: اول `NULL`-پذیر یا با `DEFAULT` امن، بعد backfill، بعد (اگر لازم بود) `NOT NULL`، در Migrationهای جدا.
- هرگز `tenant_id` را از ستون حذف نکن و policy را برندار و ضعیف نکن.
- تغییر مخرب (حذف ستون، تغییر نوع، حذف جدول): **قبل از نوشتن از من بپرس** و به چند مرحله بشکن.
- ایندکس روی جدول بزرگ (`CREATE INDEX CONCURRENTLY`): داخل تراکنش اجرا نمی‌شود؛ از من بپرس تا روش اجرا هماهنگ شود.

---

## ۵. جدول‌های غیر tenant-scoped (استثنا، با احتیاط)

- دسترسی غیر tenant-scoped به جدول‌های `tenants` و `users` محدود به ماژول‌های مسئول آن‌ها و مسیر مجاز در schema است.
- RLS روی `domains` و `memberships` طبق تصمیم‌های پروژه فعال می‌ماند. lookup اولیه از عملیات پارامتری و نقش‌های محدود مطابق تصمیم‌های پروژه استفاده می‌کند.
- **هر جدولی که `tenant_id` دارد ولی RLS ندارد** باید در فهرست استثنای تست نگهبان (الگوی ۰۳) باشد، و اضافه‌کردن به آن فهرست **فقط با تأیید من** مجاز است.
- `outbox_events` RLS اجباری دارد. Worker نقش جداگانه و مجوز محدود به عملیات پردازش همان صف دارد. policyهای آن مطابق تصمیم‌های پروژه آزموده می‌شوند.

---

## ۶. تأیید بعد از نوشتن Migration

```bash
make migrate-up      # اعمال
make migrate-down    # برگرداندن یک مرحله (باید بدون خطا باشد)
make migrate-up      # دوباره اعمال
sqlc generate        # اگر Query مربوطه تغییر کرد
go test ./...        # شامل تست‌های نگهبان الگوی ۰۳
```

## ۷. چک‌لیست هر Migration جدید

- [ ] فایل `up` و `down` هر دو هست و `down` تست شده
- [ ] جدول tenant-scoped: `tenant_id NOT NULL REFERENCES tenants(id)`
- [ ] `ENABLE` **و** `FORCE` ROW LEVEL SECURITY
- [ ] policy با `USING` **و** `WITH CHECK` و عبارت `NULLIF(current_setting('app.tenant_id', true), '')::uuid`
- [ ] یکتایی‌ها با `tenant_id` شروع می‌شوند؛ ایندکس‌های جستجو هم
- [ ] جدول فرزند: Composite FK `(tenant_id, parent_id)`
- [ ] پول `bigint`، زمان `timestamptz`
- [ ] Migration قبلی ویرایش نشده
- [ ] تست ایزولاسیون برای جدول جدید نوشته شد (الگوی ۰۳)

## ۸. این‌ها را عوض نکن

- `FORCE` RLS، ترکیب `USING` + `WITH CHECK`، و `NULLIF(current_setting(..., true), '')`.
- جدا بودن `migrator` از `app_user`.
