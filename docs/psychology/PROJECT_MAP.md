# نقشهٔ پروژهٔ روان‌شناسی

این جدول محل شروع جست‌وجوی هر قابلیت را نشان می‌دهد. مسیرهای app و قرارداد در این جدول برنامه‌ریزی‌شده‌اند و هنگام ساخت در تسک همان مرحله به‌روز می‌شوند.

| قابلیت | مسیر برنامه‌ریزی‌شده | سند و فاز |
| --- | --- | --- |
| ثبت Router و Middleware Go | `apps/psychology-api/cmd/server/` | [پلتفرم](modules/platform.md)، `PSY-0` |
| Tenant، Host و نقش‌ها | `apps/psychology-api/internal/{tenant,domain,auth}/` | [پلتفرم](modules/platform.md)، `PSY-0` |
| Schema، migration و sqlc | `docs/psychology/database.md`، `apps/psychology-api/{migrations,sqlc.yaml}` | [مدل داده](DATA_MODEL.md) · [الگوی Migration](../patterns/02-migration-rls.md)، `PSY-0.2` |
| قرارداد و کلاینت HTTP | `packages/contracts/psychology/openapi.yaml` و `packages/psychology-api-client/` | [نقشهٔ راه](TASKS.md)، `PSY-0.3` |
| پوستهٔ سایت و تنظیمات | `apps/psychology-site/src/{pages,components,theme}/` و `apps/psychology-api/internal/settings/` | [پلتفرم](modules/platform.md)، `PSY-0` |
| ورود و پوستهٔ پنل | `apps/psychology-admin/src/pages/` | [پلتفرم](modules/platform.md)، `PSY-0` |
| کارگاه، ظرفیت و ثبت‌نام | `apps/psychology-api/internal/workshops/` و مسیر `workshops/` در `src/pages/` سایت و پنل | [کارگاه](modules/workshops.md)، `PSY-1` |
| پرداخت، Callback و پیگیری | `apps/psychology-api/internal/payments/` و مسیرهای `checkout/` سایت و `payments/` پنل | [پرداخت](modules/payments.md)، `PSY-1` و `PSY-3` |
| بلاگ و پیوند رسانه | `apps/psychology-api/internal/{blog,media}/` و مسیر `blog/` سایت و پنل | [بلاگ و رسانه](modules/blog-media.md)، `PSY-1` |
| نوبت حضوری و تقویم | `apps/psychology-api/internal/booking/` و مسیر `appointments/` در سایت و پنل | [نوبت](modules/booking.md)، `PSY-2` |
| دوره، پخش و دسترسی | `apps/psychology-api/internal/{courses,media}/` و مسیرهای `courses/` در سایت و پنل و `account/courses/` در سایت | [دوره](modules/courses.md)، `PSY-3` |
| تست ایزولاسیون و مرورگر | تست Go کنار ماژول‌ها و `apps/psychology-api/internal/testsupport/`، سناریوهای مرورگر در `apps/psychology-site/tests/` | [پذیرش](ACCEPTANCE.md)، همهٔ فازها |
| استقرار، بکاپ و کش | `infrastructure/psychology/` و `apps/psychology-api/cmd/worker/` و `internal/cache/` | [پلتفرم](modules/platform.md)، `PSY-0` و `PSY-4` |
| دیزاین سیستم و کامپوننت‌های مشترک UI | `packages/ui/` | [سیستم طراحی و قالب‌ها](modules/ui-templates.md)، `PSY-5` |
| موتور قالب‌های سایت عمومی (Multi-Template) | `apps/psychology-site/src/templates/` | [الگوی ۰۴](../patterns/04-ui-and-templates.md)، `PSY-5` |

الگوی داخل هر ماژول Go از `http.go` برای HTTP، `service.go` برای منطق و `queries.sql` برای sqlc شروع می‌شود. تصمیم دربارهٔ Routeهای REST با OpenAPI است. تنظیمات monorepo و بسته‌های UI، SEO و lint عمومی را در صورت سازگاری بازاستفاده کن. [الگوی ماژول](../patterns/01-tenant-scoped-module.md)
