# ماژول سیستم طراحی مشترک و موتور چندقالبی (`ui-templates`)

این سند مشخصات فنی فاز `PSY-5` برای یکپارچه‌سازی رابط کاربری (`packages/ui`) و پشتیبانی از چند قالب قابل انتخاب توسط هر Tenant است. [تصمیم‌ها](../DECISIONS.md#psy-d09--سیستم-طراحی-کامپوننتهای-مشترک-و-معماری-چندقالبی) · [الگو](../../patterns/04-ui-and-templates.md)

---

## ۱. تغییرات پایگاه داده و Backend (`apps/psychology-api`)

### پیش‌نیاز مسیر واقعی (`PSY-5.0`)

- در `cmd/server/main.go`، pool با نقش runtime، `domain.Resolver`، `auth.Service` و `settings.Handler` را با مسیرهای عمومی و احرازشدهٔ تنظیمات به سرور واقعی وصل کن.
- Tenant مسیر عمومی از Host تأییدشده و Tenant مسیر پنل از نشست و عضویت احرازشده مشخص می‌شود. خواندن و نوشتنِ دادهٔ تنظیمات با Queryهای پارامتری sqlc و در `Store.WithTenantTx` انجام می‌شوند تا `FORCE ROW LEVEL SECURITY` جدول `site_settings` با نقش runtime اعمال شود.
- آزمون را از ورودی واقعی سرور با PostgreSQL و دو Host اجرا کن. خواندن عمومی هر مطب، رفتار Host ناشناخته، رد درخواست پنل بدون مجوز و جدایی خواندن و نوشتن A و B را بررسی کن.

### مایگریشن `000008_add_site_template_settings`
افزودن ستون‌های زیر به جدول `site_settings` (بدون تغییر مایگریشن‌های قبلی و با حفظ RLS فعلی):
- `template_id text NOT NULL DEFAULT 'minimal'` — از Postgres ENUM استفاده **نشود** تا افزودن قالب‌های آینده نیاز به Migration جدید نداشته باشد.
- `template_version text NOT NULL DEFAULT '1.0.0'` — نسخهٔ فعال در زمان ذخیره از کاتالوگ ثبت می‌شود. رندر با `template_id` پیاده‌سازی فعال را انتخاب می‌کند. تغییر ناسازگار شناسهٔ جدید می‌گیرد.
- `theme_radius text NOT NULL DEFAULT 'md' CHECK (theme_radius IN ('sm', 'md', 'lg'))`
- `theme_scale text NOT NULL DEFAULT 'md' CHECK (theme_scale IN ('sm', 'md', 'lg'))`

### اعتبارسنجی در `internal/settings`
- `template_id`: فقط حروف کوچک انگلیسی، عدد و خط تیره (`^[a-z0-9-]{2,40}$`).
- `template_version`: فرمت نسخه (`^[0-9]+\.[0-9]+\.[0-9]+$`). مقدار ممیزی در Backend از نسخهٔ فعال همان شناسه در کاتالوگ به دست می‌آید.
- `primary_color`: مطابق قاعده قبلی (`^#[0-9a-fA-F]{6}$`).
- `theme_radius` و `theme_scale`: فقط یکی از `'sm' | 'md' | 'lg'`.
- خروجی `GET /v1/public/site` و `GET /v1/admin/site` و ورودی `PUT /v1/admin/site` در `packages/contracts/psychology/openapi.yaml` و `packages/psychology-api-client` به‌روزرسانی می‌شوند.
- هنگام `PUT /v1/admin/site`، رویداد Outbox مربوط به پاک‌سازی مسیرهای متأثر Host تأییدشدهٔ همان Tenant در همان تراکنش ذخیرهٔ تنظیمات ثبت می‌شود. پاسخ پنل `private, no-store` است. با کش HTML فعال، ظاهر عمومی پس از پردازش موفق رویداد و پاک‌سازی کش همان Host به‌روز می‌شود.

---

## ۲. پکیج مشترک `packages/ui` (`@psychology/ui`)

- شامل Tailwind CSS v4 + DaisyUI v5 برای لایه استایل و توکن‌های تم.
- شامل کامپوننت‌های پایه (Button, Card, Input, Textarea, Badge, Alert, EmptyState) و کامپوننت‌های تعاملی Headless بر پایه `react-aria-components` + `@internationalized/date` (Modal, Select, Tabs, PersianDatePicker).
- استفاده هم‌زمان در `apps/psychology-site` و `apps/psychology-admin` و آماده برای ماژول‌های آتی فروشگاه (`catalog`, `cart`, `order`).
- فایل `packages/ui/src/template-catalog.ts` متادیتای `id`، `version`، `labelFa`، `descriptionFa` و `daisyTheme` را برای پنل و سایت فراهم می‌کند. `registry.ts` سایت کامپوننت‌ها را به شناسه‌های کاتالوگ وصل می‌کند. تست نگهبان، برابری دقیق مجموعهٔ شناسه‌ها را در دو جهت بررسی می‌کند.
- اتصال هر اپ با رندر یک کامپوننت مشترک تأیید می‌شود. هر دو Dockerfile بستهٔ `packages/ui` را در مرحلهٔ `builder` کپی و بیلد می‌کنند و دستور `typecheck` هر دو اپ `astro check && tsc --noEmit` را اجرا می‌کند.

---

## ۳. رفتار SSR و جلوگیری از FOUC در `psychology-site`

1. مسیر SSR پروفایل عمومی Tenant جاری را از `/v1/public/site` برای همان Host می‌خواند و تنظیمات معتبر را به `BaseLayout.astro` می‌دهد. وقتی پروفایل معتبر است و قالب در رجیستری پیدا نمی‌شود، پوستهٔ `minimal` انتخاب می‌شود. خطای تشخیص Host یا خواندن پروفایل طبق رفتار API رسیدگی می‌شود.
2. روی تگ `<html lang="fa" dir="rtl" data-theme={template.meta.daisyTheme} data-radius={radius} data-scale={scale}>` قرار می‌گیرد.
3. در صورت تغییر `primary_color` توسط روان‌شناس، فقط متغیر CSS مربوط به رنگ برند به صورت امن (پس از چک Regex) در استایل inline همان `<html>` تزریق می‌شود.
4. خانهٔ `/` ظاهر منتشرشده را رندر می‌کند و پارامترهای ظاهری پیش‌نمایش را اعمال نمی‌کند.

---

## ۴. پیش‌نمایش قبل از ذخیره

مسیر `/preview` در `psychology-site` همان رندر صفحهٔ خانه را با محتوای عمومی Host جاری استفاده می‌کند. پاسخ `Cache-Control: private, no-store` دارد و HTML با `<meta name="robots" content="noindex, nofollow">` رندر می‌شود. مسیر از کش مشترک و sitemap کنار گذاشته می‌شود.

- فقط `template`، `primary`، `radius` و `scale` از Query برای ظاهر پذیرفته می‌شوند. شناسه از رجیستری و رنگ و اندازه‌ها با همان قواعد تم اعتبارسنجی می‌شوند. قالب ناشناخته به `minimal` برمی‌گردد و ورودی ظاهری نامعتبر با مقدار معتبر منتشرشده یا پیش‌فرض جایگزین می‌شود. Tenant فقط از Host جاری تعیین می‌شود.
- پنل برای مقایسهٔ قالب‌ها با مقادیر فعلی فرم، `/preview` را روی دامنهٔ عمومی تأییدشدهٔ مطب باز می‌کند. پیش‌نمایش فقط خواندنی است و انتشار به اقدام ذخیره در پنل وابسته است.

## ۵. سناریوهای تست الزامی

- **Backend & RLS:** تغییر `template_id` توسط Tenant A هیچ اثری روی خروجی `GET /v1/public/site` برای Tenant B ندارد. ورودی‌های نامعتبر (`template_id` با کاراکتر غیرمجاز یا `theme_radius: "xl"`) کد `400` برمی‌گردانند.
- **Fallback ایمن:** اگر از API مقدار `template_id: "non-existent"` برسد، `resolveTemplate` بدون کرش روی `minimal` برمی‌گردد.
- **کاتالوگ و نسخه:** شناسه‌های دو فهرست دقیقاً برابرند. ذخیره نسخهٔ فعال را ثبت می‌کند و رکوردی با نسخهٔ قدیمی همچنان پیاده‌سازی فعال همان شناسه را رندر می‌کند.
- **ذخیره، کش و پیش‌نمایش:** مسیر اولیهٔ پنل تا خانه و `/preview` را با دو Host و مراحل [آزمون پذیرش PSY-A12](../ACCEPTANCE.md#psy-a12--جزئیات-آزمون-فاز-۵) پیش از ریفکتور سایر صفحات اجرا کن.
- **تقویم شمسی:** انتخاب تاریخ شمسی در `PersianDatePicker` خروجی استاندارد تاریخ (میلادی ISO برای ارسال به API و نمایش شمسی فارسی برای کاربر) تولید می‌کند.
- **بیلد و تایپ:** `make verify-psychology` و `pnpm lint` بدون خطا سبز می‌شوند.
