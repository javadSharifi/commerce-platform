# docs/ANTI_PATTERNS.md — کارهای آسانِ غلط ← کارهای درست

> **چطور از این فایل استفاده کنی:**
> قبل از نوشتن کد، اگر کاری که می‌خواهی بکنی شبیه ستون ❌ یکی از موارد پایین است، **بایست** و ستون ✅ را انجام بده.
> هر مورد شناسه دارد (`AP-01`، ...). در گزارش‌ها و کامیت‌ها می‌توانی به آن ارجاع بدهی.
> اگر وضعیتی را می‌بینی که این‌جا نیست و «راه آسان» با «راه درست» فرق دارد، **بپرس** و بعد از تأیید من همین‌جا اضافه‌اش کن (بخش آخر).
> مثال‌ها قالب هستند: نام جدول/ستون/نوع ID را با تصمیم نهایی P0.2 هماهنگ کن.

## فهرست سریع

| شناسه | موضوع | شدت |
| --- | --- | --- |
| AP-01 | `tenant_id` از ورودی کاربر | 🔴 بحرانی |
| AP-02 | اتصال دیتابیس با نقش مالک/superuser (RLS دور زده می‌شود) | 🔴 بحرانی |
| AP-03 | `SET` بدون `LOCAL` / خارج تراکنش (نشت بین Tenantها) | 🔴 بحرانی |
| AP-04 | جدول جدید بدون `tenant_id` و RLS | 🔴 بحرانی |
| AP-05 | اعتماد به قیمت/مبلغ ارسالی کلاینت | 🔴 بحرانی |
| AP-06 | پول با float | 🔴 بحرانی |
| AP-07 | کم‌کردن موجودی با «بخوان، بعد بنویس» (oversell) | 🔴 بحرانی |
| AP-08 | سفارش بدون Snapshot | 🔴 بحرانی |
| AP-09 | تأیید پرداخت غیر Idempotent | 🔴 بحرانی |
| AP-10 | Cache کردن صفحات شخصی (سبد، حساب، checkout) | 🔴 بحرانی |
| AP-11 | پاک‌کردن کش مستقیم از handler (به‌جای Outbox) | 🟠 مهم |
| AP-12 | اعتماد به Host/`X-Forwarded-Host` یا دامنه تأییدنشده | 🔴 بحرانی |
| AP-13 | ساخت SQL با الحاق رشته | 🔴 بحرانی |
| AP-14 | HTML کاربر با `set:html` بدون پاک‌سازی (XSS) | 🔴 بحرانی |
| AP-15 | `slug`/SKU یکتای سراسری به‌جای یکتا در هر Tenant | 🟠 مهم |
| AP-16 | ویرایش Migration قدیمی | 🟠 مهم |
| AP-17 | شکستن مرز ماژول‌ها (دسترسی مستقیم به جدول‌های ماژول دیگر) | 🟠 مهم |
| AP-18 | قورت‌دادن خطا | 🟠 مهم |
| AP-19 | تست جعلی / ضعیف‌کردن تست برای سبز شدن | 🔴 بحرانی |
| AP-20 | اضافه‌کردن dependency/سرویس «برای راحتی» | 🟠 مهم |
| AP-21 | Secret در کد یا Git | 🔴 بحرانی |
| AP-22 | نوشتن دستی تایپ‌های API در Frontend | 🟡 متوسط |

---

## AP-01 — `tenant_id` از ورودی کاربر 🔴

**وسوسه:** «کلاینت `tenant_id` را می‌فرستد، ساده‌تر است.»
**چرا غلط است:** هر کسی می‌تواند عدد را عوض کند و به داده فروشگاه دیگر برسد.

```go
// ❌
tenantID := r.URL.Query().Get("tenant_id")        // یا از body / header دلخواه

// ✅ Middleware قبلاً از Host (یا Membership در پنل) استخراج و در context گذاشته
t, ok := tenant.FromContext(r.Context())
if !ok { http.Error(w, "tenant not resolved", http.StatusInternalServerError); return }
```

**قاعده:** Storefront ← از دامنه تأییدشده. Admin ← از عضویت احرازشده کاربر (و اگر چند فروشگاه دارد، بررسی کن عضو آن است).

---

## AP-02 — اتصال با نقش مالک/superuser 🔴

**وسوسه:** «با همان کاربر `postgres` وصل شو که همه‌چیز کار کند.»
**چرا غلط است:** Superuser و مالک جدول، RLS را **دور می‌زنند**. تست‌ها سبز می‌شوند ولی ایزولاسیون واقعی نیست.

```sql
-- ✅ نقش جدا برای برنامه
CREATE ROLE app_user LOGIN PASSWORD '...' NOSUPERUSER NOBYPASSRLS;
-- Migration با نقش مالک (جدا) اجرا می‌شود، برنامه با app_user وصل می‌شود.

-- ✅ روی هر جدول tenant-scoped
ALTER TABLE products ENABLE ROW LEVEL SECURITY;
ALTER TABLE products FORCE ROW LEVEL SECURITY;   -- مالک هم تابع RLS شود
```

**چک:** تست ایزولاسیون باید با اتصال `app_user` اجرا شود، نه با superuser.

---

## AP-03 — `SET` بدون `LOCAL` / خارج تراکنش 🔴

**وسوسه:** `SET app.tenant_id = '...'` روی connection.
**چرا غلط است:** Connection در pool به درخواست‌های بعدی (Tenant دیگر) می‌رسد و مقدار قبلی می‌ماند ← **نشت داده**.
**نکته:** `SET LOCAL` پارامتر قبول نمی‌کند؛ از `set_config(..., true)` استفاده کن (معادل LOCAL و پارامتری).

```go
// ❌
conn.Exec(ctx, "SET app.tenant_id = '"+tenantID+"'")   // هم نشت، هم SQL injection

// ✅ همیشه داخل تراکنش؛ tenant از context می‌آید (نه از پارامتر)
func (s *Store) WithTenantTx(ctx context.Context, fn func(tx pgx.Tx, tenantID uuid.UUID) error) error {
    tenantID, ok := tenantctx.ID(ctx)
    if !ok { return ErrNoTenant }
    return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
        if _, err := tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", tenantID.String()); err != nil {
            return err
        }
        return fn(tx, tenantID)
    })
}
```

**قاعده:** هر دسترسی به داده tenant-scoped فقط از طریق `WithTenantTx`. مستقیم از pool نخوان.
**نسخه کامل و رسمی:** `docs/patterns/01-tenant-scoped-module.md` (اگر این مثال با آن فرق داشت، آن الگو معتبر است).

---

## AP-04 — جدول جدید بدون `tenant_id` و RLS 🔴

**وسوسه:** «این جدول کوچک است، فعلاً tenant نمی‌خواهد.»

```sql
-- ✅ هر جدول tenant-scoped در همان Migration:
CREATE TABLE products (
    id         uuid PRIMARY KEY,                       -- نوع ID طبق P0.2.1
    tenant_id  uuid NOT NULL REFERENCES tenants(id),
    -- ...
);
CREATE INDEX products_tenant_idx ON products (tenant_id);

ALTER TABLE products ENABLE ROW LEVEL SECURITY;
ALTER TABLE products FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON products
    USING      (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);
```

`NULLIF(..., '')` و `true` باعث می‌شوند اگر tenant ست نشده باشد، **هیچ سطری** برنگردد (امن)، نه خطا و نه همه‌چیز.
**چک‌لیست هر Migration جدید:** `tenant_id NOT NULL` · ایندکس · ENABLE + FORCE RLS · policy با USING و WITH CHECK · تست ایزولاسیون برای آن جدول.

---

## AP-05 — اعتماد به قیمت/مبلغ ارسالی کلاینت 🔴

```go
// ❌
total := req.Total                       // کلاینت فرستاده
price := req.Items[i].Price

// ✅ فقط شناسه و تعداد از کلاینت؛ قیمت از دیتابیس
variants := catalog.GetVariantsForCheckout(ctx, ids)   // قیمت فعلی از DB
total := computeTotal(variants, req.Items, shipping)   // محاسبه در Backend
```

همین قاعده برای موجودی، هزینه ارسال، تخفیف و مبلغ پرداخت. صفحه Cache‌شده هرگز منبع قیمت پرداخت نیست.

---

## AP-06 — پول با float 🔴

```go
// ❌
price float64
// ❌ ستون: numeric با ممیز بدون تصمیم، real، double precision، money

// ✅ عدد صحیح در کوچک‌ترین واحد (طبق تصمیم P0.2.2)
price int64            // Go
-- ✅ SQL
price bigint NOT NULL CHECK (price >= 0)
```

محاسبه درصد (تخفیف/مالیات) را با عدد صحیح و قاعده گرد کردن مشخص انجام بده و تست بنویس.

---

## AP-07 — oversell: «بخوان، بعد بنویس» 🔴

```go
// ❌ دو درخواست هم‌زمان هر دو می‌بینند موجودی ۱ است
stock := getStock(id)
if stock >= qty { setStock(id, stock-qty) }
```

```sql
-- ✅ یک دستور اتمیک
-- name: DecrementStock :execrows
UPDATE product_variants
SET stock = stock - $2
WHERE id = $1 AND stock >= $2;
```
```go
n, err := q.DecrementStock(ctx, db.DecrementStockParams{ID: id, Qty: qty})
if err != nil { return err }
if n == 0 { return ErrInsufficientStock }   // بدون تأثیر = موجودی کافی نبود
```

علاوه بر آن: `CHECK (stock >= 0)` روی ستون، و کاهش موجودی داخل **همان تراکنش** ساخت سفارش.

---

## AP-08 — سفارش بدون Snapshot 🔴

```sql
-- ❌ order_items فقط variant_id دارد و قیمت/نام را با JOIN از محصول می‌خوانیم
-- (با تغییر قیمت، سفارش قدیمی عوض می‌شود)

-- ✅ مقادیر زمان خرید در خود سطر ذخیره شود
CREATE TABLE order_items (
    id            uuid PRIMARY KEY,
    tenant_id     uuid NOT NULL,
    order_id      uuid NOT NULL,
    variant_id    uuid,              -- فقط برای ارجاع؛ ممکن است بعداً حذف شود
    product_title text   NOT NULL,   -- Snapshot
    sku           text   NOT NULL,   -- Snapshot
    unit_price    bigint NOT NULL,   -- Snapshot
    quantity      int    NOT NULL CHECK (quantity > 0)
);
```

مبلغ‌های سفارش (جمع، ارسال، تخفیف) هم در `orders` ذخیره می‌شود، نه اینکه هر بار محاسبه شود.

---

## AP-09 — تأیید پرداخت غیر Idempotent 🔴

**وسوسه:** در Callback درگاه، سفارش را «paid» کن و کار تمام.
**چرا غلط است:** کاربر صفحه را refresh می‌کند یا درگاه دو بار Callback می‌زند ← دو بار اثر (موجودی، ایمیل، ...).

```sql
-- ✅ فقط از pending می‌شود paid شد
-- name: MarkOrderPaid :execrows
UPDATE orders SET status = 'paid', paid_at = now()
WHERE id = $1 AND status = 'pending';

-- و یکتایی تراکنش درگاه
ALTER TABLE payments ADD CONSTRAINT payments_provider_ref_uq UNIQUE (provider, provider_ref);
```
اگر `n == 0` بود، یعنی قبلاً پردازش شده؛ **بدون خطا و بدون تکرار اثرها** پاسخ موفق بده.
همیشه Verify را با سرور درگاه انجام بده، نه با پارامترهای Callback.

---

## AP-10 — Cache کردن صفحات شخصی 🔴

```ts
// ❌ سبد/حساب/checkout با هدر کش عمومی (یا بدون هدر در حالی که CDN همه‌چیز را Cache می‌کند)

// ✅ صفحات شخصی
Astro.response.headers.set('Cache-Control', 'private, no-store');

// ✅ فقط برای پاسخ عمومی canonical که برای کش مشترک مجاز است
Astro.response.headers.set('Cache-Control', 'public, s-maxage=300');
```

قاعده‌ها:
- پاسخی که به cookie/session/کاربر وابسته است **هرگز** عمومی Cache نشود.
- Cache key شامل Host + مسیر (و زبان/ارز اگر وجود دارد).
- صفحات جستجو، فیلتر و pagination با query string از کش مشترک عبور می‌کنند.
- Worker URLهای متأثر را فقط برای Host تأییدشدهٔ همان Tenant در آروان پاک می‌کند. رفتار CDN در P5 طبق D-26 آزموده می‌شود.

---

## AP-11 — پاک‌کردن کش مستقیم از handler 🟠

```go
// ❌ بعد از commit، مستقیم CDN را صدا بزن
tx.Commit(ctx)
cdn.PurgeURLs(urls)      // خرابی در این نقطه، کش کهنه را بدون رویداد قابل retry باقی می‌گذارد

// ✅ رویداد را در همان تراکنشِ تغییر داده بنویس؛ Worker بعداً پردازش و retry می‌کند
err := store.WithTenantTx(ctx, func(tx pgx.Tx, tenantID uuid.UUID) error {
    if err := catalogdb.New(tx).UpdateVariantPrice(ctx, params); err != nil { return err }
    return outboxdb.New(tx).InsertOutboxEvent(ctx, outboxdb.InsertOutboxEventParams{
        TenantID: tenantID,
        Type:     "product.updated",
        Payload:  payloadJSON,          // دادهٔ حداقلی برای URLهای متأثر همان Tenant
    })
})
// هر دو نوشتن در یک تراکنش‌اند: یا هر دو انجام می‌شوند یا هیچ‌کدام.
```

Worker: رویدادها را با `SELECT ... FOR UPDATE SKIP LOCKED` برمی‌دارد، retry با backoff، و رویداد ناموفق را dead-letter می‌کند.

---

## AP-12 — اعتماد به Host/`X-Forwarded-Host` یا دامنه تأییدنشده 🔴

```go
// ❌
host := r.Header.Get("X-Forwarded-Host")      // قابل جعل توسط کلاینت
t := findTenantByHostname(host)               // بدون بررسی تأیید

// ✅
// - Host از مسیر مورد اعتماد آروان و Caddy و طبق مرز اعتماد مصوب پذیرفته شود
// - lookup فقط روی دامنه‌های تأییدشده
SELECT tenant_id FROM domains WHERE hostname = $1 AND status = 'verified';
```

- دامنه تأییدنشده به هیچ Tenant وصل نمی‌شود.
- یک hostname فقط به یک Tenant تعلق دارد (`UNIQUE (hostname)`).
- فعال‌سازی CDN و صدور TLS لبه و origin به دامنهٔ تأییدشده محدودند. مسیر و آزمون‌ها در D-26 آمده‌اند.

---

## AP-13 — SQL با الحاق رشته 🔴

```go
// ❌
q := "SELECT * FROM products WHERE slug = '" + slug + "'"
q := fmt.Sprintf("... ORDER BY %s", sortParam)

// ✅ sqlc با پارامتر
-- name: GetProductBySlug :one
SELECT * FROM products WHERE slug = $1;
```

برای `ORDER BY` پویا: مقدار کاربر را با **allowlist ثابت** (map از نام مجاز به ستون) بپذیر، هرگز مستقیم وارد SQL نکن.

---

## AP-14 — `set:html` / `innerHTML` با محتوای کاربر 🔴

```astro
---
// ❌ توضیحات محصول (که صاحب فروشگاه نوشته) مستقیم
---
<div set:html={product.description} />

<!-- ✅ -->
<!-- ۱) ترجیحاً متن ساده: <p>{product.description}</p> -->
<!-- ۲) اگر HTML لازم است: هنگام ذخیره، در Backend با sanitizer مبتنی بر allowlist
        پاک‌سازی شود و نسخه پاک‌سازی‌شده ذخیره/رندر شود. -->
```

افزودن کتابخانه sanitizer = dependency جدید؛ اول از من تأیید بگیر (AP-20). مورد مشابه: `eval`، `new Function`، script دلخواه در قالب ممنوع است.

---

## AP-15 — یکتایی سراسری به‌جای یکتایی در هر Tenant 🟠

```sql
-- ❌ دو فروشگاه نمی‌توانند هر دو محصولی با slug "iphone" داشته باشند
slug text UNIQUE

-- ✅
UNIQUE (tenant_id, slug)
UNIQUE (tenant_id, sku)
```
و ایندکس‌های جستجو هم با `tenant_id` شروع شوند: `(tenant_id, slug)`.

---

## AP-16 — ویرایش Migration قدیمی 🟠

- Migrationی که یک‌بار اجرا/merge شده **هرگز ویرایش نشود**. تغییر = Migration جدید (شماره بعدی).
- هر Migration هم `up` دارد هم `down`.
- تغییر مخرب (حذف ستون، تغییر نوع) را به چند مرحله بشکن و قبل از انجام از من بپرس.

---

## AP-17 — شکستن مرز ماژول‌ها 🟠

```go
// ❌ ماژول order مستقیم جدول‌های catalog را query/JOIN می‌کند یا sqlc آن را import می‌کند
import "app/internal/catalog/db"

// ✅ از طریق interface عمومی ماژول
type VariantReader interface {
    GetVariantsForCheckout(ctx context.Context, ids []uuid.UUID) ([]CheckoutVariant, error)
}
// ماژول order فقط به این interface وابسته است.
```

هدف: بعداً بشود ماژول را جدا کرد. چرخه import بین ماژول‌ها ممنوع است.

---

## AP-18 — قورت‌دادن خطا 🟠

```go
// ❌
result, _ := q.GetOrder(ctx, id)
if err != nil { }                       // خالی
log.Println(err); return nil            // خطا را «موفق» وانمود کردن

// ✅
if err != nil {
    return fmt.Errorf("get order %s: %w", id, err)   // با context به بالا برگردان
}
```

خطای کاربر (۴xx) را از خطای سرور (۵xx) جدا کن. جزئیات داخلی (SQL، stack) را به کلاینت نفرست؛ در لاگ بنویس.

---

## AP-19 — تست جعلی یا ضعیف‌کردن تست 🔴

```go
// ❌
assert.True(t, true)
t.Skip("flaky")                          // برای سبز شدن
assert.NotNil(t, result)                 // به‌جای سنجیدن رفتار واقعی
// ❌ شل‌کردن assertion تا تست رد شود

// ✅ تست باید بدون پیاده‌سازی درست، fail شود
```

نمونه الزامی تست ایزولاسیون (با اتصال `app_user`، نه superuser):
1. دو Tenant بساز (A و B).
2. در Tenant A یک محصول/سفارش بساز.
3. با context Tenant B همان را بخوان و تغییر بده.
4. **انتظار:** صفر سطر برای خواندن، و رد شدن/بی‌اثر بودن برای تغییر.

اگر تستی fail می‌شود، **علت** را درست کن. اگر فکر می‌کنی خود تست غلط است، دلیلش را بنویس و از من بپرس.

---

## AP-20 — dependency یا سرویس «برای راحتی» 🟠

**ممنوع بدون تأیید:** Redis، Kubernetes، message queue، ORM (جایگزین sqlc)، کتابخانه UI یا state سنگین، فریم‌ورک اضافه، هر package که با ۲۰ خط کد ساده جایگزین می‌شود.

قبل از پیشنهاد dependency بنویس:
1. چه مشکلی را حل می‌کند؟
2. چرا کد ساده/کتابخانه استاندارد کافی نیست؟
3. اندازه و نگهداری‌شدن آن؟
4. اثر روی مصرف RAM و تعداد سرویس‌های همیشه‌روشن؟

---

## AP-21 — Secret در کد یا Git 🔴

```go
// ❌
const paymentKey = "abc123"
// ❌ commit کردن .env یا کلید درگاه در تست‌ها/README

// ✅ فقط از env؛ فایل .env در .gitignore؛ فقط .env.example با مقدار فرضی
key := os.Getenv("PAYMENT_API_KEY")
```

کلیدهای درگاهِ هر فروشگاه که در دیتابیس ذخیره می‌شوند باید **رمزنگاری‌شده** باشند و در لاگ/پاسخ API ظاهر نشوند.

---

## AP-22 — تایپ‌های API را دستی نوشتن 🟡

```ts
// ❌ interface Product { ... }   // کپی دستی در Storefront و Admin، دیر یا زود ناهماهنگ می‌شود

// ✅ تایپ‌ها و کلاینت از OpenAPI در packages/api-client تولید شوند
import { client } from '@platform/api-client';
```

اگر endpoint عوض شد، اول OpenAPI (`packages/contracts`) را عوض کن، بعد کلاینت را دوباره تولید کن. از `any` برای دور زدن خطای تایپ استفاده نکن.

---

## اضافه‌کردن مورد جدید به این فایل

وقتی راه آسانِ غلطی پیدا کردی که این‌جا نیست:
1. **کار را ادامه نده**؛ از من بپرس.
2. بعد از تأیید من، با همین قالب اضافه کن: `AP-XX — عنوان` · **وسوسه** · **چرا غلط است** · ❌ و ✅ با کد · **قاعده**.
3. شناسه را به «فهرست سریع» بالای فایل اضافه کن.
