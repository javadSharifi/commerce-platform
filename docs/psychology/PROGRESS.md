# وضعیت مسیر روان‌شناسی

- آخرین کار تکمیل‌شده — `PSY-0.3`، ساخت قرارداد OpenAPI در `packages/contracts/psychology/openapi.yaml` و بسته کلاینت TypeScript تایپ‌شده در `packages/psychology-api-client/`.
- تسک در حال انجام — هیچ‌کدام.
- تسک پیشنهادی بعدی — `PSY-0.4`، ساخت migration نقش‌ها، `tenants`، دامنه‌ها، هویت پنل، عضویت و نشست با RLS و lookup محدود.
- دستور تأیید برنامه — `cd apps/psychology-api && go test ./...` و در ریشه `pnpm typecheck && pnpm build && pnpm test`. دستور کامل `make verify-psychology` در `PSY-0.8` تنظیم می‌شود.

## تأییدهای محیط هدف

- حساب هدف زرین‌پال، باکت پارس‌پک، دامنه و تنظیمات آروان، شرایط عرضه و نگه‌داری داده در تسک‌های عرضهٔ هر نسخه بررسی می‌شوند. مسیر محلی با دادهٔ ساختگی و Adapter پرداخت تستی قابل اجراست. [نقشهٔ راه](TASKS.md)

## گزارش‌ها

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
