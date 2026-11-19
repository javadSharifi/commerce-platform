# نقشهٔ پروژهٔ فروشگاه‌ساز

برای پیدا کردن یک قابلیت، از ردیف همان قابلیت شروع کن. اولویت کار در [TASKS.md](../TASKS.md)، تصمیم‌ها در [DECISIONS.md](DECISIONS.md) و وضعیت اجرا در [PROGRESS.md](PROGRESS.md) ثبت می‌شوند.

برای مسیر کارگاه، نوبت و دوره به [نقشهٔ پروژهٔ روان‌شناسی](psychology/PROJECT_MAP.md) مراجعه کن.

## ساختار Monorepo

الان فایل‌های راهنما در ریشه و `docs/` موجودند. درخت و مسیرهای پیاده‌سازی برنامه‌ریزی‌شده‌اند و در تسک‌های مربوط ساخته می‌شوند.

```text
commerce-platform/
├── apps/
│   ├── api/                 # Go · REST API مشترک
│   │   ├── cmd/server/      # نقطهٔ شروع HTTP server
│   │   ├── internal/        # ماژول‌های Go
│   │   └── migrations/     # Migrationهای PostgreSQL و RLS
│   ├── storefront/          # Astro SSR · سایت هر فروشگاه
│   └── admin/               # Astro · پنل مشترک مدیریت
├── packages/
│   ├── contracts/           # OpenAPI و قراردادهای مشترک
│   ├── api-client/          # کلاینت TypeScript تولیدشده از OpenAPI
│   ├── theme-contract/      # قرارداد قالب‌ها
│   ├── ui/                  # کامپوننت‌های مشترک
│   ├── config/              # تنظیمات مشترک TypeScript و lint
│   └── seo/                 # ابزارهای SEO
├── infrastructure/
│   ├── docker/
│   ├── caddy/
│   ├── arvancloud/
│   └── monitoring/
├── scripts/
├── docs/                    # اسناد موجود
├── pnpm-workspace.yaml
└── package.json
```

## هر قابلیت کجاست؟

| قابلیت | نقطهٔ شروع پیاده‌سازی | سند موجود و تسک |
| --- | --- | --- |
| راه‌اندازی API و health check | `apps/api/cmd/server/main.go` | [TASKS.md](../TASKS.md)، `P1.1` |
| قرارداد HTTP و کلاینت TypeScript | `packages/contracts/openapi.yaml` و `packages/api-client/` | [وضعیت قراردادها](modules/README.md#وضعیت-قراردادها)، `P0.3` و `P3.1.3` |
| مسیرهای Public و Admin در API مشترک | ثبت Router در `apps/api/cmd/server/` و Handler ماژول مربوط در `apps/api/internal/` | [الگوی ماژول Go](patterns/01-tenant-scoped-module.md)، `P0.3.2` |
| Schema، Migration، Query و RLS | `docs/database.md`، `apps/api/migrations/` و `apps/api/sqlc.yaml` | [الگوی Migration](patterns/02-migration-rls.md) · [تست ایزولاسیون](patterns/03-tenant-isolation-test.md)، `P0.2` و `P1.2` |
| ساخت فروشگاه و محدودهٔ Tenant | `apps/api/internal/tenant/` | [Tenant](modules/tenant.md)، `P1.3` |
| دامنه، تشخیص Host، DNS و TLS | `apps/api/internal/domain/`، `apps/storefront/`، `apps/admin/` و `infrastructure/{arvancloud,caddy}/` | [Domain](modules/domain.md)، `P1.3`، `P4.3` و `P5.3` |
| ورود مدیر، نشست، عضویت و نقش | `apps/api/internal/auth/` و `apps/admin/` | [Auth](modules/auth.md)، `P1.4` و `P4.1` |
| محصول، تنوع، دسته‌بندی و جستجو | `apps/api/internal/{catalog,category}/`، `apps/storefront/` و `apps/admin/` | [Catalog](modules/catalog.md) · [Category](modules/category.md)، `P2.1`، `P3.3` و `P4.2` |
| صفحهٔ اصلی فروشگاه، نمایش کالا و RTL | `apps/storefront/` | [TASKS.md](../TASKS.md)، `P3.1.6` و `P3.3` |
| تصویر محصول، آپلود فیلم و ویس و S3 پارس‌پک | فرم محصول و مطلب در `apps/admin/`، پخش در `apps/storefront/`. مسیر بخش Go در `P2.2` مشخص می‌شود. | [Media](modules/media.md)، `P2.2`، `P3.3.11` و `P4.2.6` |
| بلاگ، پیش‌نویس و انتشار مطلب | ماژول Go در `apps/api/internal/`، ویرایشگر در `apps/admin/`، فهرست و صفحهٔ مطلب در `apps/storefront/` | [Blog](modules/blog.md)، `P2.8`، `P3.3.10` و `P4.2.15` |
| موجودی و جلوگیری از oversell | `apps/api/internal/inventory/` و `apps/admin/` | [Inventory](modules/inventory.md)، `P2.3` و `P4.2.5` |
| ثبت‌نام و حساب مشتری | `apps/api/internal/customer/`، `apps/storefront/` و `apps/admin/` | [Customer](modules/customer.md)، `P2.4` و `P3.3.7` |
| سبد مهمان و مشتری | رابط در `apps/storefront/`. مسیر بخش Go در `P2.4` مشخص می‌شود. | [Cart](modules/cart.md)، `P2.4` و `P3.3.5` |
| Checkout، سفارش، انقضا و لغو | `apps/api/internal/order/`، `apps/storefront/` و `apps/admin/` | [Order](modules/order.md)، `P2.5`، `P3.3.6` و `P4.2` |
| زرین‌پال، Callback و Verify | `apps/api/internal/payment/` و صفحهٔ نتیجه در `apps/storefront/` | [Payment](modules/payment.md)، `P2.6` و `P3.3.6` |
| آدرس و هزینهٔ ارسال | `apps/api/internal/shipping/`، Checkout در `apps/storefront/` و تنظیمات در `apps/admin/` | [Shipping](modules/shipping.md)، `P2.7` و `P4.2.12` |
| قالب، انتخاب Theme و نسخه‌بندی | `apps/api/internal/theme/`، قالب‌ها در `apps/storefront/` و `packages/theme-contract/` | [Theme](modules/theme.md)، `P3.2` و `P4.2.11` |
| تنظیمات فروشگاه، نام، لوگو و تماس | فرم در `apps/admin/`، نمایش در `apps/storefront/`. مسیر بخش Go در طراحی تنظیمات مشخص می‌شود. | [Settings](modules/settings.md)، `P0.3.6` و `P4.2.10` |
| حالت خرید فروشگاه و تماس برای قیمت | تنظیمات مالک در `apps/admin/`، وضعیت قیمت در `apps/api/internal/catalog/`، مرحلهٔ تماس در `apps/storefront/` و کنترل خرید آنلاین در سرویس‌های سفارش و پرداخت | [Settings](modules/settings.md) · [Catalog](modules/catalog.md) · [Cart](modules/cart.md)، `P2.0.1`، `P2.1.10`، `P3.3.9` و `P4.2.14` |
| داشبورد و اجزای مشترک پنل | `apps/admin/` و `packages/ui/` | [TASKS.md](../TASKS.md)، `P4.1.5` و `P4.2.1` |
| SEO، canonical، sitemap و robots | `packages/seo/` و `apps/storefront/` | [TASKS.md](../TASKS.md)، `P3.4` |
| کش آروان، Outbox، Worker و purge | هدرها در `apps/storefront/`، purge در `apps/api/internal/cache/` و تنظیمات در `infrastructure/arvancloud/`. نقطهٔ اجرای Worker در `P5.1` مشخص می‌شود. | [Cache](modules/cache.md)، `P3.5`، `P5.1` و `P5.2` |
| محیط توسعه، CI و دیپلوی | ریشهٔ Monorepo، `packages/config/`، `scripts/` و `infrastructure/` | [روش کار](WORKFLOW.md) · [TASKS.md](../TASKS.md)، `P0.1`، `P1.5` و `P5.4` |
| مانیتورینگ، پشتیبان‌گیری و بازیابی | `infrastructure/monitoring/` برای مانیتورینگ. نقطهٔ اجرای Backup و Restore در `P5.5` مشخص می‌شود. | [TASKS.md](../TASKS.md)، `P5.5` |

در ماژول‌های Go، `http.go` نقطهٔ ورود HTTP، `service.go` منطق ماژول و `queries.sql` نقطهٔ شروع Queryهاست. [الگوی ماژول Go](patterns/01-tenant-scoped-module.md)

وقتی نقطهٔ ورود یک قابلیت ساخته شد، مسیر واقعی را در همان ردیف با برچسب «موجود» ثبت کن. اگر مسیر جابه‌جا شد، همان ردیف را به‌روز کن.
