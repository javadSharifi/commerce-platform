# docs/patterns/ — الگوهای طلایی (Golden Patterns)

> این پوشه **نمونه‌ی کاملِ درست** برای کارهای تکراری است.
> قانون: وقتی کاری شبیه یکی از این الگوها را می‌کنی، **اول الگو را بخوان، بعد کپی کن و فقط نام‌ها را عوض کن.** ساختار را عوض نکن.
> اگر فکر می‌کنی الگو اشتباه یا ناقص است، **کد را خودت تغییر نده**؛ از من بپرس.

## فهرست الگوها

| فایل | کی بخوانم | فاز |
| --- | --- | --- |
| `01-tenant-scoped-module.md` | ساخت هر endpoint/سرویس که به داده tenant-scoped دست می‌زند (Handler → Service → sqlc → `WithTenantTx`) | P1، P2 |
| `02-migration-rls.md` | ساخت هر جدول جدید یا تغییر جدول (Migration + `tenant_id` + RLS + Grant) | P1، P2 |
| `03-tenant-isolation-test.md` | نوشتن تست ایزولاسیون و «تست‌های نگهبان» (RLS روی همه جدول‌ها، نقش دیتابیس) | P1، P2 |

*الگوهای بعدی در فاز مربوط نوشته می‌شوند. Domain Resolver، Outbox و Worker، هدر کش Astro و URL purge در آروان، کامپوننت قالب و `PaymentProvider`.*
**اگر الگویی برای کارت وجود ندارد، شبیه‌سازی نکن؛ بپرس.**

## کتابخانه‌های پایه (برای Go از قبل تأییدشده)

فقط این‌ها بدون پرسیدن مجازند؛ هر چیز دیگر طبق `AGENTS.md` و `ANTI_PATTERNS.md (AP-20)` تأیید می‌خواهد:

| کتابخانه | کاربرد |
| --- | --- |
| `net/http`، `log/slog`، `context`، `errors`، `testing` (stdlib) | پایه |
| `github.com/go-chi/chi/v5` | Router |
| `github.com/jackc/pgx/v5` (`pgxpool`، `pgconn`) | دیتابیس |
| `sqlc` (ابزار تولید کد) | Queryها |
| `github.com/golang-migrate/migrate/v4` | Migration |
| `github.com/go-playground/validator/v10` | اعتبارسنجی ورودی |
| `github.com/google/uuid` | نوع UUID (طبق تصمیم P0.2.1؛ اگر ID از نوع دیگری انتخاب شد، الگوها را با من هماهنگ کن) |

> برای تست‌ها از خود `testing` استفاده کن؛ کتابخانه assertion اضافه نکن.

## قراردادهای مشترک

- **Module path** در مثال‌ها `commerce/api` است؛ با نام واقعی `go.mod` عوض کن.
- **ساختار ماژول:** `internal/<module>/{service.go, http.go, queries.sql, <module>db/}`؛ پوشه `<module>db/` را sqlc تولید می‌کند و **دستی ویرایش نمی‌شود**.
- **لایه‌ها:** `http.go` فقط HTTP (decode، validate، status code)؛ `service.go` منطق و تراکنش؛ فقط Service با دیتابیس حرف می‌زند.
- **Tenant:** فقط از `context` (که middleware پر کرده) و فقط از طریق `Store.WithTenantTx`.
- **خطاها:** خطای دامنه با `errors.New` در ماژول تعریف می‌شود (`ErrNotFound`، `ErrSlugTaken`) و `http.go` آن‌ها را به status code ترجمه می‌کند.
- **نام‌گذاری Migration:** `NNNNNN_<action>_<table>.up.sql` و `.down.sql`.
- **زمان:** `timestamptz` همیشه؛ مقدار زمان در UTC.
- **ID** — UUID v4 با `gen_random_uuid()` طبق D-21.
