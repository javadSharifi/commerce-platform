# وضعیت مسیر روان‌شناسی

- آخرین کار تکمیل‌شده — `PSY-0.7`، ساخت پوستهٔ RTL پنل و سایت، قرارداد قالب نسخه‌دار (`clinic-standard` نسخه 1.0.0)، اندپوینت‌های عمومی و پنل پروفایل و تنظیمات کلینیک با اعتبارسنجی رنگ و عنوان.
- تسک در حال انجام — هیچ‌کدام.
- تسک پیشنهادی بعدی — `PSY-0.8`، تنظیم دستور واحد `make verify-psychology` برای بیلد، vet، تست Go و lint، typecheck و تست بسته‌های TS و مستندسازی فاز PSY-0.
- دستور تأیید برنامه — `cd apps/psychology-api && go test ./...` و در ریشه `pnpm typecheck && pnpm build && pnpm test`. دستور کامل `make verify-psychology` در `PSY-0.8` نهایی می‌شود.

## تأییدهای محیط هدف

- حساب هدف زرین‌پال، باکت پارس‌پک، دامنه و تنظیمات آروان، شرایط عرضه و نگه‌داری داده در تسک‌های عرضهٔ هر نسخه بررسی می‌شوند. مسیر محلی با دادهٔ ساختگی و Adapter پرداخت تستی قابل اجراست. [نقشهٔ راه](TASKS.md)

## گزارش‌ها

### ۲۰۲۶-۱۱-۱۹ — اجرای تسک PSY-0.7 (پوسته RTL و تنظیمات کلینیک)

- ماژول `settings` در Go شامل سرویس، هَندلر HTTP و کوئری‌های SQL برای تنظیمات و هویت بصری کلینیک پیاده‌سازی شد.
- مسیر عمومی `GET /v1/public/site` با استفاده از Domain Resolver و تفکیک Host تنها اطلاعات انتشارپذیر را بازمی‌گرداند.
- مسیرهای پنل `GET /v1/admin/site` و `PUT /v1/admin/site` با کنترل نشست، عضویت Tenant، نقش‌های مجاز و اعتبارسنجی فرمت رنگ هگز پیاده‌سازی شدند.
- قرارداد پوسته نسخه‌دار `ThemeConfig` در `apps/psychology-site/src/theme/theme.ts` پیاده شد و قالب راست‌چین `BaseLayout.astro` با تزریق رنگ داینامیک `--primary` و صفحهٔ اصلی کلینیک ایجاد شدند.
- پوسته پنل راست‌چین `AdminLayout.astro` با سایدبار، تاپ‌بار و صفحهٔ تنظیمات و مشخصات کلینیک (`settings.astro`) در `apps/psychology-admin` ایجاد و بیلد شدند.

### ۲۰۲۶-۱۱-۱۹ — اجرای تسک PSY-0.6 (احراز هویت، عضویت و کنترل نقش)

- هشینگ رمز عبور با الگوریتم استاندارد `Argon2id` طبق تصمیم PSY-D02 پیاده‌سازی شد (`HashPassword` و `CheckPassword`).
- ماژول `auth.Service` برای ورود، صدور نشست سروری با توکن هش‌شده SHA-256، خروج و بررسی عضویت در کلینیک پیاده‌سازی شد.
- میدل‌ویرهای `Authenticate` (احراز هویت کوکی یا توکن)، `RequireTenantMembership` (تأیید عضویت در Tenant انتخابی با هدر `X-Tenant-ID`)، `RequireRole` (محدودسازی به نقش‌های مجاز) و `CSRFProtection` (مسدودسازی درخواست‌های cross-site) پیاده‌سازی شدند.
- اندپوینت‌های `/admin/auth/login`، `/admin/auth/logout` و `/admin/auth/me` پیاده‌سازی شدند.
- آزمون‌های جامع در `internal/auth/auth_test.go` برای نشست‌های مستقل، مسدودسازی دسترسی متقاطع کاربر کلینیک A به کلینیک B، کنترل دسترسی نقش‌ها و رد درخواست‌های CSRF با موفقیت پاس شدند.

### ۲۰۲۶-۱۱-۱۹ — اجرای تسک PSY-0.5 (دامنه رسیلور و WithTenantTx)

- پکیج `tenantctx` در `internal/platform/tenantctx` برای انتقال ایمن `tenant_id` در کانتکست پیاده‌سازی شد.
- تابع `WithTenantTx` در `internal/platform/db/store.go` جهت اجرای تراکنش‌های ایزوله مستاجر با ست کردن `SELECT set_config('app.tenant_id', ...)` پیاده‌سازی شد.
- کوئری‌های SQL دامنه تعریف شده و کدهای Go با `sqlc generate` در `internal/domain/domaindb` تولید شدند.
- ماژول `domain.Resolver` در `internal/domain/resolver.go` همراه با پاک‌سازی Host و آزمون سناریوهای دو Host معتبر، Host تأییدنشده و نامعتبر نوشته و پاس شد.

### ۲۰۲۶-۱۱-۱۹ — اجرای تسک PSY-0.4 (مهاجرت‌های هسته و RLS)

- فایل‌های migration `000001_create_core_schema.up.sql` و `000001_create_core_schema.down.sql` ایجاد شدند.
- نقش‌های کم‌اختیار `app_user` و `lookup_user` با `NOSUPERUSER NOBYPASSRLS` به صورت idempotent اضافه شدند.
- جدول‌های `tenants`، `domains`، `site_settings`، `panel_identities`، `memberships`، `panel_sessions` و `password_resets` پیاده‌سازی شدند.
- سیاست‌های `ENABLE ROW LEVEL SECURITY` و `FORCE ROW LEVEL SECURITY` همراه با policyهای جداسازی مستاجر بر اساس `app.tenant_id` روی جدول‌های tenant-scoped اعمال گردیدند.
- تست یکپارچگی فایل‌های migration و تطابق امنیتی RLS در `apps/psychology-api/migrations/migration_test.go` نوشته شد و با موفقیت پاس شد.

### ۲۰۲۶-۱۱-۱۹ — اجرای تسک PSY-0.3 (قرارداد OpenAPI و کلاینت تایپ‌شده)

- قرارداد سراسری OpenAPI 3.0.3 تحت پیشوند `/v1` در فایل `packages/contracts/psychology/openapi.yaml` تعریف شد.
- جداسازی حوزه‌ها انجام شد: Public (دریافت اطلاعات سایت بر مبنای Host)، Admin Auth (ورود، خروج و me)، Admin Site (تنظیمات کلینیک)، Account (حساب خریدار برای فاز ۳) و Callbacks (بازگشت از درگاه زرین‌پال).
- الگوهای استاندارد خطا (`ErrorResponse`)، صفحه‌بندی (`PaginationMeta`) و طرح احراز هویت با کوکی نشست (`SessionAuth`) مستند شدند.
- بسته کلاینت `@psychology/api-client` در `packages/psychology-api-client/` با تایپ‌های کامل TypeScript، کلاس خطای استاندارد `ApiClientError` و متدهای ارتباط با API پیاده‌سازی و در مونو‌ریپو متصل شد (`typecheck` و `build` سبز).

### ۲۰۲۶-۱۱-۱۹ — اجرای تسک PSY-0.2 (مستند پایگاه داده و اسکیما)

- سند `docs/psychology/database.md` بر اساس قراردادهای معماری نوشته شد.
- قواعد سراسری پایگاه داده ثبت شدند: شناسه‌های UUID v4، زمان‌های `timestamptz` بر مبنای UTC، مبالغ `bigint` بر حسب ریال، و حذف منطقی با ایندکس جزئی.
- قوانین سخت RLS، پالیسی‌های جداسازی مستاجر با `current_setting('app.tenant_id', true)` و کلیدهای خارجی ترکیبی (`Composite FK`) جهت ممانعت از ارجاع متقاطع تدوین شدند.
- ترتیب قفل‌ها، پیشگیری از Deadlock و اسکیماهای هسته برای فازهای PSY-0 تا PSY-4 مستند شدند.

### ۲۰۲۶-۱۱-۱۹ — اجرای تسک PSY-0.1 (اسکلت اجرایی)

- ساختار `pnpm workspace` با فایل‌های `package.json` و `pnpm-workspace.yaml` و `.gitignore` راه‌اندازی شد.
- ماژول Go در `apps/psychology-api/` با router چی (`chi/v5`) و تست اندپوینت `/healthz` ساخته و با موفقیت آزموده شد (`go test` سبز است).
- دو اپلیکیشن Astro در `apps/psychology-site/` (وب‌سایت عمومی) و `apps/psychology-admin/` (پنل مدیریت) با حالت SSR و آداپتور Node پیکربندی و بیلد شدند (`pnpm build` موفق).
- فایل `infrastructure/psychology/docker-compose.yml` برای اجرای دیتابیس محلی PostgreSQL 16 ایجاد شد.

### ۲۰۲۶-۱۱-۱۹ — نقشهٔ راه مستقل

کارگاه و بلاگ در نسخهٔ اول، نوبت در نسخهٔ دوم و فروش دورهٔ ضبط‌شده در نسخهٔ سوم تعریف شدند. تسک‌های پیاده‌سازی و راه‌اندازی باز هستند. [نقشهٔ راه](TASKS.md) · [آزمون‌های پذیرش](ACCEPTANCE.md)
