# راهنمای پایگاه داده و Schema مسیر روان‌شناسی

> این سند مرجع حقیقت ساختار پایگاه داده، قیدها، امنیت RLS و ترتیب قفل‌ها برای پلتفرم روان‌شناسی است.
> تمامی Migrationها و کوئری‌های `sqlc` باید به این سند و [الگوی Migration و RLS](../../patterns/02-migration-rls.md) وفادار باشند.

---

## ۱. قراردادهای سراسری (Global Conventions)

| موضوع | قرارداد الزامی | دلیل و کاربرد |
| --- | --- | --- |
| **شناسه‌ها (IDs)** | `UUID v4` با `DEFAULT gen_random_uuid()` | یکتایی بدون وابستگی به شماره‌انداز متمرکز و جلوگیری از حدس‌زدن IDها |
| **زمان‌ها** | `timestamptz` | ذخیره زمان به صورت دقیق همراه با منطقه زمانی (همیشه در سطح کد به صورت UTC ذخیره و پردازش می‌شود) |
| **مبالغ مالی** | `bigint` (ریال) | جلوگیری قطعی از خطای ممیز شناور (float). نمایش در رابط کاربری به تومان است. |
| **نام‌گذاری** | `snake_case` انگلیسی و جمع برای جداول | مثال: `tenants`، `memberships`، `site_settings` |
| **حذف منطقی** | `deleted_at timestamptz` | در منابع مهم همراه با ایندکس‌های یکتای جزئی (`WHERE deleted_at IS NULL`) |

---

## ۲. قوانین امنیت داده و RLS (Row Level Security)

1. **تفکیک کامل Tenantها:**
   تمام جدول‌های داده‌ای کلینیک‌ها شامل ستون `tenant_id uuid NOT NULL REFERENCES tenants(id)` هستند.
2. **اجبار RLS:**
   تمام جدول‌های tenant-scoped دارای دو دستور زیر هستند:
   ```sql
   ALTER TABLE <table_name> ENABLE ROW LEVEL SECURITY;
   ALTER TABLE <table_name> FORCE ROW LEVEL SECURITY;
   ```
   دستور `FORCE` تضمین می‌کند که حتی مالک جدول (یا نقش‌های بدون Superuser) نیز مقید به policy هستند.
3. **الگوی Policy استاندارد:**
   ```sql
   CREATE POLICY tenant_isolation ON <table_name>
       USING      (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
       WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);
   ```
4. **کلیدهای خارجی ترکیبی (Composite FK):**
   برای جلوگیری از حملات ارجاع متقاطع بین دو مستاجر (Cross-Tenant Reference Attack)، هر جدول فرزند علاوه بر `tenant_id` مستقل خود، با کلید ترکیبی `(tenant_id, parent_id)` به والد ارجاع می‌دهد:
   ```sql
   -- روی والد:
   ALTER TABLE parents ADD CONSTRAINT parents_tenant_id_id_uq UNIQUE (tenant_id, id);
   -- روی فرزند:
   FOREIGN KEY (tenant_id, parent_id) REFERENCES parents (tenant_id, id) ON DELETE CASCADE
   ```

---

## ۳. ترتیب قفل‌ها و پیشگیری از Deadlock

1. **ترتیب قفل منابع:** در هر تراکنشی که نیاز به قفل چند ردیف دارد (مانند ثبت‌نام کارگاه یا رزرو نوبت):
   - همیشه ابتدا منبع والد (مثل کارگاه یا سانس نوبت‌دهی) قفل می‌شود و سپس ردیف فرزند ایجاد یا به‌روز می‌شود.
2. **مرتب‌سازی در عملیات دسته‌ای:** اگر چند ردیف هم‌زمان با `SELECT ... FOR UPDATE` قفل می‌شوند، حتماً بر اساس `id ASC` مرتب شوند.
3. **خروج تماس‌های شبکه از تراکنش:** هرگونه فراخوانی وب‌سرویس خارجی (درگاه زرین‌پال، پیامک، S3 پارس‌پک) **اکیداً خارج از تراکنش دیتابیس** انجام می‌شود.

---

## ۴. ساختار جداول هسته (Core Schema - فاز PSY-0)

### ۴.۱ جدول `tenants` (کلینیک‌ها / مطب‌ها)
```sql
CREATE TABLE tenants (
    id         uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    name       text        NOT NULL,
    slug       text        NOT NULL UNIQUE,
    status     text        NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'suspended', 'archived')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
```

### ۴.۲ جدول `domains` (دامنه‌ها و زیردامنه‌ها)
هر کلینیک یک یا چند دامنه (زیردامنه پایلوت یا دامنه اختصاصی) دارد.
```sql
CREATE TABLE domains (
    id          uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   uuid        NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    host        text        NOT NULL UNIQUE,
    is_primary  boolean     NOT NULL DEFAULT false,
    is_verified boolean     NOT NULL DEFAULT false,
    verified_at timestamptz,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX domains_tenant_id_idx ON domains (tenant_id);

ALTER TABLE domains ENABLE ROW LEVEL SECURITY;
ALTER TABLE domains FORCE  ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON domains
    USING      (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);
```

### ۴.۳ جدول `site_settings` (تنظیمات سایت و هویت بصری)
```sql
CREATE TABLE site_settings (
    tenant_id     uuid        PRIMARY KEY REFERENCES tenants(id) ON DELETE CASCADE,
    title         text        NOT NULL DEFAULT '',
    description   text        NOT NULL DEFAULT '',
    phone         text        NOT NULL DEFAULT '',
    address       text        NOT NULL DEFAULT '',
    primary_color text        NOT NULL DEFAULT '#0284c7',
    logo_media_id uuid,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE site_settings ENABLE ROW LEVEL SECURITY;
ALTER TABLE site_settings FORCE  ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON site_settings
    USING      (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);
```

### ۴.۴ جدول `panel_identities` (کاربران پنل مدیریت کلینیک)
هویت کلی اعضای کادر درمان و پنل مدیریت (مستقل از اینکه در کدام کلینیک‌ها عضو هستند).
```sql
CREATE TABLE panel_identities (
    id            uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    email         text        NOT NULL UNIQUE,
    password_hash text        NOT NULL,
    full_name     text        NOT NULL,
    status        text        NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'inactive')),
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);
```

### ۴.۵ جدول `memberships` (عضویت و نقش در کلینیک)
تعیین نقش هر کاربر در هر کلینیک (`owner`، `content_manager`، `receptionist`، `therapist`).
```sql
CREATE TABLE memberships (
    id         uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id  uuid        NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id    uuid        NOT NULL REFERENCES panel_identities(id) ON DELETE CASCADE,
    role       text        NOT NULL CHECK (role IN ('owner', 'content_manager', 'receptionist', 'therapist')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT memberships_tenant_user_uq UNIQUE (tenant_id, user_id)
);

ALTER TABLE memberships ENABLE ROW LEVEL SECURITY;
ALTER TABLE memberships FORCE  ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON memberships
    USING      (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);
```

### ۴.۶ جدول `panel_sessions` (نشست‌های سمت سرور)
```sql
CREATE TABLE panel_sessions (
    id         uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid        NOT NULL REFERENCES panel_identities(id) ON DELETE CASCADE,
    token_hash text        NOT NULL UNIQUE,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX panel_sessions_token_hash_idx ON panel_sessions (token_hash);
CREATE INDEX panel_sessions_expires_at_idx ON panel_sessions (expires_at);
```

### ۴.۷ جدول `password_resets` (کدهای بازیابی رمز عبور)
```sql
CREATE TABLE password_resets (
    id         uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid        NOT NULL REFERENCES panel_identities(id) ON DELETE CASCADE,
    code_hash  text        NOT NULL,
    expires_at timestamptz NOT NULL,
    used_at    timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX password_resets_user_id_idx ON password_resets (user_id);
```

---

## ۵. نقشهٔ راه تکمیل افزایشی جداول در فازهای بعدی

| فاز | جداول جدید | شرح و قیدهای کلیدی |
| --- | --- | --- |
| **`PSY-1` (کارگاه و بلاگ)** | `workshops`<br>`workshop_registrations`<br>`purchases`<br>`payment_attempts`<br>`blog_posts`<br>`media_assets` | • قفل کارگاه روی ظرفیت صندلی‌ها<br>• ثبت‌نام مهمان با شماره تماس<br>• اسنپ‌شات ریالی در خرید و پرداخت<br>• کلیدهای خارجی ترکیبی `(tenant_id, workshop_id)` |
| **`PSY-2` (نوبت‌دهی)** | `therapists`<br>`therapist_schedules`<br>`appointment_slots`<br>`appointments` | • تولید سانس‌های غیرهم‌پوشان<br>• قفل اتمیک سانس نوبت جهت جلوگیری از رزرو هم‌زمان<br>• ثبت تاریخچه تغییر وضعیت نوبت |
| **`PSY-3` (دوره‌ها)** | `buyer_accounts`<br>`buyer_sessions`<br>`courses`<br>`course_lessons`<br>`course_entitlements` | • حساب مجزای خریدار مقید به Tenant<br>• دسترسی فعال به دوره با پیوند به خرید موفق<br>• عدم دسترسی به لینک‌های ویدیو بدون خرید |
| **`PSY-4` (بهره‌برداری)** | `outbox_events` (در صورت نیاز به CDN purge) | • رویدادهای پاک‌سازی کش در همان تراکنش تغییر داده |
