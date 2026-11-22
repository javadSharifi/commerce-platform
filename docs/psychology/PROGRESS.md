# وضعیت مسیر روان‌شناسی

## وضعیت فعلی فاز ۵

- آخرین کار تکمیل‌شده — `PSY-5.9` (اجرای کامل سناریوی پذیرش PSY-A12 شامل رگرسیون جامع تمام صفحات محتوایی، تعاملی، RTL، ریسپانسیو موبایل، تقویم شمسی و ایزولاسیون کامل چندمستأجری؛ پایان و تکمیل موفقیت‌آمیز فاز ۵).
- تسک در حال انجام — هیچ‌کدام (فاز ۵ با موفقیت ۱۰۰٪ تکمیل شد).
- تسک پیشنهادی بعدی — فاز ۵ به پایان رسید و تمامی اهداف سایت‌ساز، قالب‌ها و دیزاین سیستم روان‌شناسی با آزمون‌های پذیرش تثبیت شدند.
- دستور تأیید برنامه — `make verify-psychology` و `pnpm lint`.

## وضعیت ثبت‌شده در پایان فاز ۴

- آخرین کار تکمیل‌شده — `PSY-4.4` (پایان فاز ۴ و تکمیل کل نقشهٔ راه تخصصی روان‌شناسی · ثبت نتایج نهایی دو کلینیک پایلوت، محاسبه هزینه‌های واقعی زیرساخت ابری، سهمیه رسانه ۱۰ گیگابایتی، مدل قیمت‌گذاری اشتراک با حاشیه سود بالای ۷۰٪، تعیین محدوده نسخه بعد و تأیید کامل سناریوهای پذیرش PSY-A01 تا PSY-A11).
- تسک در حال انجام — هیچ‌کدام (تمامی تسک‌های نقشهٔ راه از PSY-0 تا PSY-4.4 تکمیل و تأیید شده‌اند).
- تسک پیشنهادی بعدی — آماده برای استقرار نهایی در محیط تولید (Production Deployment).
- دستور تأیید برنامه — `make verify-psychology` (یا در ریشه `pnpm run verify:psychology` و در بک‌اند `cd apps/psychology-api && go test -v ./...`).

## تأییدهای محیط هدف

- حساب هدف زرین‌پال، باکت پارس‌پک، دامنه و تنظیمات آروان، شرایط عرضه و نگه‌داری داده در تسک‌های عرضهٔ هر نسخه بررسی می‌شوند. مسیر محلی با دادهٔ ساختگی و Adapter پرداخت تستی قابل اجراست. [نقشهٔ راه](TASKS.md)

## گزارش‌ها

### ۲۰۲۶-۱۱-۲۱ — اجرای تسک PSY-5.9 (اجرای کامل سناریوی پذیرش PSY-A12 و تکمیل فاز ۵)

- آزمون جامع پذیرش فاز ۵ (`PSY-A12`) با پوشش کامل تمام سناریوها و معیارهای تعریف‌شده در `ACCEPTANCE.md` اجرا و تأیید شد:
  - **بک‌اند و ایزولاسیون دیتابیس (`TestAcceptance_PSY_A12_InitialPath_TwoHosts`):**
    - تأیید رفتار روی دو میزبان مجزا و تاییدشده (`rezaei-clinic.ir` و `niloofar-psych.ir`).
    - ثبت و ذخیره تنظیمات بصری مستأجر A با نقش امن runtime و `FORCE RLS` در پایگاه داده.
    - ثبت رویداد Outbox در همان تراکنش ذخیره تنظیمات و پاک‌سازی هوشمند کش پروکسی لبه فقط برای Host A.
    - حفظ کامل داده و عدم باطل‌شدن کش Host B (تضمین صفر نشت بین‌مستأجری و بازگشت قطعی Cache HIT برای مستأجر B).
    - اعتبارسنجی امنیتی عدم دستکاری متقابل (Cross-Tenant Tampering Guard): تلاش برای ویرایش مطب B توسط نشست مطب A با خطای `403 Forbidden` مسدود شد.
  - **سایت عمومی، SSR و پیش‌نمایش در فرانت‌اند (`apps/psychology-site/test/acceptance-a12.test.mjs`):**
    - پیش‌نمایش لحظه‌ای دو قالب `minimal` و `clinical` با متغیرهای رنگ برند، گردی لبه‌ها (`data-radius`) و مقیاس فونت (`data-scale`).
    - صدور قطعی هدر `Cache-Control: private, no-store` و متاتگ `<meta name="robots" content="noindex, nofollow">` در مسیر `/preview`.
    - بازگشت امن به قالب پیش‌فرض `minimal` در صورت درخواست شناسه قالب ناشناخته یا ثبت‌نشده.
    - جلوگیری قطعی از تزریق کدهای مخرب CSS و XSS در پارامتر رنگ، و بازگشت امن به رنگ پیش‌فرض سیستم (`#0284c7`).
    - حفاظت از صفحهٔ اصلی تولید (`/`): نادیده گرفتن پارامترهای پیش‌نمایش ارسالی در کوئری، نمایش ظاهر منتشرشده رسمی، و تنظیم هدر کش لبه `public, s-maxage=3600`.
    - تفکیک قطعی خروجی عمومی Host A و Host B در سطح قالب و اطلاعات مطب.
  - **رگرسیون صفحات محتوایی و رعایت اصول RTL و موبایل:**
    - بررسی موفق ۶ مسیر محتوایی (`/courses`، `/courses/anxiety-management`، `/workshops`، `/workshops/[id]`، `/blog`، `/blog/[slug]`).
    - تایید کد وضعیت ۲۰۰ و هدر کش عمومی `public` روی صفحات عمومی.
    - تایید کامل ویژگی‌های جهت‌بندی راست‌به‌چپ (`dir="rtl"`) و زبان فارسی (`lang="fa"`).
    - تایید متاتگ ابعاد ریسپانسیو موبایل (`<meta name="viewport" content="width=device-width, initial-scale=1.0">`) روی تمامی صفحات.
    - تایید نبود JavaScript اضافه روی صفحات محتوایی خالص (0 KB Client JS / عدم وجود تگ اسکریپت ماژول).
    - تایید حذف کامل تمامی رنگ‌های هاردکد شده (`#ffffff` و `#0f172a`) و جایگزینی کامل با توکن‌های معنایی مشترک DaisyUI (`card bg-base-100`, `text-primary`, etc.).
  - **تعاملات رزرو نوبت و تقویم شمسی:**
    - تایید رندر جزیره تعاملی Astro Island برای کامپوننت `PersianDatePicker` با شناسه فعال‌سازی `client="visible"`.
    - تایید تولید تاریخ استاندارد شمسی و نمایش اعداد و اسامی ماه‌های هجری شمسی در ساختار HTML.
    - تایید صدور قطعی هدر امنیتی `Cache-Control: private, no-store` روی داشبورد دوره‌ها (`/account/courses`)، پخش‌کننده جلسات (`/account/courses/[id]`)، پیگیری نوبت (`/appointments/[id]`) و وضعیت ثبت‌نام (`/workshops/registrations/[id]`).
- **نتایج آزمون‌ها و اعتبارسنجی:**
    - `node --test apps/psychology-site/test/acceptance-a12.test.mjs`: PASS (۱۴۶ میلی‌ثانیه).
    - `pnpm test`: PASS برای تمامی سوئیت‌های تست فرانت‌اند (۶ سوئیت در `psychology-site`، ۳ سوئیت در `psychology-admin`، ۱۵ تست کامپوننت در `@psychology/ui`، و تست‌های `@psychology/api-client`).
    - `pnpm typecheck`: بدون خطا و بدون هشدار روی تمامی ۶۶ فایل استرو و کدهای تایپ‌اسکریپت.
    - `pnpm lint`: خروجی بدون خطا (exit 0) روی تمامی پروژه‌ها.
    - `make verify-psychology`: اجرای موفقیت‌آمیز آزمون‌های بک‌اند Go (`go vet` و `go test -v ./...` با PASS کامل) و فرانت‌اند (`pnpm typecheck`, `pnpm test`, `pnpm build`) با پیام نهایی: `All psychology platform checks passed successfully!`.
- فاز ۵ با موفقیت کامل به پایان رسید.

### ۲۰۲۶-۱۱-۲۱ — اجرای تسک PSY-5.8.4 (مهاجرت صفحات پنل مدیریت به کامپوننت‌های مشترک و DaisyUI)

- ساختار پوسته و تمامی صفحات اپلیکیشن پنل مدیریت (`apps/psychology-admin`) به سیستم طراحی یکپارچه مهاجرت کردند:
  - **پوسته مدیریت (`AdminLayout.astro`):**
    - حذف کامل تمامی استایل‌های هاردکد شده CSS (`:root`, `.sidebar`, `.card`, `.nav-item`).
    - بازنویسی سایدبار با ساختار معنایی `bg-neutral text-neutral-content`، ناوبری فعال با `bg-primary text-primary-content`، هدر بالا با `bg-base-100 border-base-300` و پس‌زمینه با `bg-base-200`.
    - تنظیم قطعی هدرهای امنیتی `Cache-Control: no-store, no-cache, must-revalidate, private` و `<meta name="robots" content="noindex, nofollow">` برای تمامی صفحات پنل.
  - **داشبورد شاخص‌ها (`index.astro`):**
    - پیاده‌سازی کارت‌های آماری کلان با کلاس‌های `card bg-base-100 border-base-200` و لبه‌های رنگی وضعیت (`border-e-success`, `border-e-primary`, `border-e-secondary`, `border-e-warning`).
    - استفاده از کامپوننت نوار پیشرفت استاندارد DaisyUI (`progress progress-warning`) برای نمایش سهمیه رسانه و فایل.
  - **ماژول کارگاه‌ها (`workshops/index.astro`، `[id].astro`، `new.astro`):**
    - بازنویسی جدول کارگاه‌ها و شرکت‌کنندگان با `table table-zebra`، تب‌های وضعیت با `tabs tabs-boxed`، بج‌های وضعیت با `badge-success`, `badge-warning`, `badge-error`, `badge-ghost`.
    - فرم‌های تعریف کارگاه و ویرایش ظرفیت صندلی با فیلدهای `input input-bordered input-sm`، `textarea textarea-bordered` و دکمه‌های استاندارد `btn-primary`, `btn-success`, `btn-warning`, `btn-error`.
  - **ماژول دوره‌ها (`courses/index.astro`، `[id]/lessons.astro`، `[id]/edit.astro`، `new.astro`):**
    - حذف کامل استایل‌های inline و کلاس‌های اختصاصی؛ بازنویسی جدول دوره‌ها و لیست جلسات با توکن‌های DaisyUI.
    - بازنویسی فرم‌های افزودن جلسه، اعتبارسنجی رسانه، ویرایش دوره و تغییر وضعیت‌های انتشار و بایگانی.
  - **ماژول بلاگ (`blog/index.astro`، `[id].astro`، `new.astro`):**
    - بازنویسی ویرایشگر امن بلوک‌های محتوایی (پاراگراف، تیتر، تصویر، صوت، ویدئو) با کارت‌های `card bg-base-200/40 border-base-200`، اینپوت‌ها و دراپ‌داون‌های استاندارد DaisyUI و دکمه‌های تغییر ترتیب و آپلود مدیا.
  - **ماژول رسانه‌ها (`media/index.astro`):**
    - کارت نمایش سهمیه ذخیره‌سازی پارس‌پک با `progress-primary`/`progress-error`، بخش آپلود مستقیم و جدول فایل‌های تاییدشده.
  - **ماژول بازپرداخت‌ها (`payments/index.astro`):**
    - جدول تراکنش‌های نیازمند استرداد و مودال ثبت دستی شماره مرجع بانکی با لایه بک‌دراپ بلور (`backdrop-blur-xs`) و استایل‌های استاندارد.
  - **ماژول نوبت‌دهی (`appointments/index.astro`):**
    - کارت‌های خلاصه وضعیت نوبت‌ها، جدول مراجعان، مودال‌های لغو، جابه‌جایی اتمیک و تاریخچه تغییر وضعیت با استایل‌های هماهنگ DaisyUI.
- تست یکپارچگی SSR پنل مدیریت در `apps/psychology-admin/test/admin-pages.test.mjs` نوشته شد که پاسخ ۲۰۰، هدرهای `no-store, private`، متاتگ `noindex` و توکن‌های DaisyUI را روی تمامی ۸ مسیر پنل بررسی و تأیید می‌کند.
- پورت‌های سوئیت‌های تست سایت و پنل جهت جلوگیری از تداخل همزمانی تفکیک شدند (`4391-4395` برای سایت و `4401-4403` برای پنل).
- دستورهای `pnpm typecheck`، `pnpm test` (همه ۲۶ تست فرانت‌اند و کلاینت)، `pnpm lint` و `make verify-psychology` با موفقیت کامل پاس شدند.

### ۲۰۲۶-۱۱-۲۱ — اجرای تسک PSY-5.8.3 (مهاجرت account/courses/، appointments/ و booking/ با no-store و PersianDatePicker Island)

- تمامی صفحات تعاملی و خصوصی سایت عمومی (`apps/psychology-site`) بازنویسی و با دیزاین سیستم سازگار شدند:
  - **داشبورد دوره‌های من (`account/courses/index.astro`):**
    - حفظ هدر سخت‌گیرانه `Cache-Control: no-store, no-cache, must-revalidate, private` جهت جلوگیری قطعی از ذخیره در کش لبه یا پروکسی‌های عمومی.
    - بازنویسی فرم احراز هویت پیامکی (ارسال OTP، تایمر شمارش معکوس دو دقیقه‌ای، اعتبارسنجی کد ۵ رقمی و ورود) و لیست دوره‌های خریداری‌شده با کلاس‌ها و کامپوننت‌های معنایی DaisyUI (`card bg-base-100`, `input-bordered`, `btn-primary`, `badge-success`).
    - استفاده از کامپوننت اشتراکی `EmptyState` از `@psychology/ui` در صورت عدم وجود دوره با دکمه مستقیم بازگشت به کاتالوگ دوره‌ها.
    - حذف کامل تمامی رنگ‌های ثابت و هاردکد قبلی (`#ffffff`, `#0f172a`, `#0284c7`, `#f1f5f9`).
  - **پخش‌کننده جلسات دوره (`account/courses/[id].astro`):**
    - حفظ هدر سخت‌گیرانه `Cache-Control: no-store, no-cache, must-revalidate, private`.
    - بازنویسی کامل رابط کاربری سرفصل‌ها و جلسات دوره با ساختار دو ستونه ریسپانسیو (پلیر و اطلاعات جلسه در محتوای اصلی و لیست سرفصل‌ها در نوار کناری).
    - پشتیبانی کامل از پلیر ویدیو و صوت اختصاصی با کلاس‌های DaisyUI (`card bg-base-100`, `badge-info`, `badge-accent`)، دکمه‌های ناوبری میان جلسات و استخراج داینامیک توکن پخش و هدرهای امنیتی.
  - **پیگیری خصوصی نوبت (`appointments/[id].astro`):**
    - حفظ هدرهای امنیتی خصوصی `Cache-Control: no-store, no-cache, must-revalidate, private` و متاتگ `<meta name="robots" content="noindex, nofollow">` برای جلوگیری از ایندکس شدن نوبت‌های شخصی مراجعان.
    - بازنویسی کارت وضعیت نوبت، مشخصات جلسه، اطلاعات درمانگر، راهنمای تماس و پنجره تایید لغو نوبت با مدال و بج‌های وضعیت DaisyUI (`badge-success`, `badge-info`, `badge-error`, `badge-ghost`).
    - هندلینگ ایمن عدم دسترسی و کلید نامعتبر `lookup_hash` با طراحی هماهنگ و بدون افشای اطلاعات حساس.
  - **درخواست نوبت و رزرو مشاوره (`booking/index.astro`):**
    - تنظیم هدر کش لبه عمومی `Cache-Control: public, max-age=0, s-maxage=1800, stale-while-revalidate=60`.
    - ادغام کامپوننت `PersianDatePicker` از بسته `@psychology/ui` به عنوان یک Astro Island تعاملی (`<PersianDatePicker client:visible ... />`) جهت انتخاب تاریخ جلسه در گاه‌شماری هجری شمسی، نمایش نام ماه‌ها و اعداد فارسی، و تولید اینپوت مخفی استاندارد `YYYY-MM-DD` برای هماهنگی با بک‌اند.
    - بازنویسی کارت‌های انتخاب درمانگر، مدال درخواست نوبت، فرم ثبت مشخصات مراجع و اعلان‌های راهنما با توکن‌های معنایی و استایل‌های استاندارد.
- آزمون‌های اعتبارسنجی اختصاصی در `apps/psychology-site/test/interactive-pages.test.mjs` ایجاد شد و در اسکریپت `test` اضافه گردید:
  - تأیید کد وضعیت ۲۰۰ برای تمام ۴ مسیر.
  - تأیید صدور دقیق هدر `no-store, private` در صفحات خصوصی و `public` در صفحه عمومی رزرو.
  - تأیید رندر Astro Island (`<astro-island client="visible" ...>`) و تولید خودکار خروجی تاریخ شمسی و توکن‌های DaisyUI.
  - تأیید عدم وجود هرگونه رنگ هاردکد شده (`#ffffff`, `#0f172a`, etc.) در HTML رندر شده در سرور.
- آزمون‌های پلتفرم: `pnpm typecheck`، `pnpm test` (تمامی ۶ سوئیت تست سایت و کل ورک‌اسپیس‌ها)، `pnpm lint` و `make verify-psychology` همگی با موفقیت کامل پاس شدند.

### ۲۰۲۶-۱۱-۲۱ — اجرای تسک PSY-5.8.2 (مهاجرت courses/، workshops/ و blog/ سایت به رندر مشترک و SSR)

- تمامی صفحات محتوایی سایت (`apps/psychology-site`) به رندر مشترک و معماری قالب‌ها مهاجرت کردند:
  - **صفحات فهرست (`courses/index.astro`، `workshops/index.astro`، `blog/index.astro`):**
    - تنظیم هدر کش لبهٔ شبکه `Cache-Control: public, max-age=0, s-maxage=3600, stale-while-revalidate=60`.
    - دریافت پروفایل مطب و تشخیص پویا و امن قالب فعال (`minimal` یا `clinical`) با `resolveTemplate(site.template_id)`.
    - استفاده از کامپوننت‌های کارت اختصاصی قالب فعال (`activeTemplate.components.CourseCard`، `WorkshopCard`، `BlogCard`) و تبدیل داده به ساختار ViewModelهای استاندارد معماری ۸۰/۲۰ (`CourseCardVM`، `WorkshopCardVM`، `BlogCardVM`).
    - استفاده از کامپوننت اشتراکی `EmptyState` از پکیج `@psychology/ui` در شرایط خالی بودن لیست دوره‌ها، کارگاه‌ها یا مقالات.
    - حذف کامل رنگ‌های ثابت و استایل‌های inline و پیاده‌سازی کامل توکن‌های معنایی DaisyUI و Tailwind.
  - **صفحات جزئیات (`courses/[slug].astro`، `workshops/[id].astro`، `blog/[slug].astro`):**
    - تزریق متغیرهای ظاهری مطب (`templateId`، `primaryColor`، `themeRadius`، `themeScale`) به لایهٔ پایه `BaseLayout.astro`.
    - بازنویسی کامل چیدمان بصری و ساختار با کلاس‌های معنایی DaisyUI (`card bg-base-100`, `input input-bordered`, `btn btn-primary`, `badge badge-success`, etc.) و رعایت دقیق قواعد RTL منطقی (`ms-`, `me-`, `ps-`, `pe-`, `start-`, `end-`).
    - صفحهٔ وبلاگ `blog/[slug].astro` بلوک‌های محتوا (پاراگراف‌ها، هدینگ‌ها، صوت، تصویر، ویدئو) را به صورت کامل و خالص در سرور (SSR) بدون نیاز به حتی ۱ کیلوبایت اسکریپت در کلاینت رندر می‌کند.
  - **صفحهٔ پیگیری ثبت‌نام (`workshops/registrations/[id].astro`):**
    - تنظیم هدر پاسخ خصوصی `Cache-Control: private, no-store`.
    - جایگزینی بج‌ها و هشدارهای رنگی هاردکد شده با کامپوننت‌ها و توکن‌های معتبر وضعیت در DaisyUI (`alert alert-success`، `badge-success`، `alert-warning`، `alert-error`).
- آزمون‌های اعتبارسنجی SSR و رندر صفحات در `apps/psychology-site/test/content-pages.test.mjs` نوشته شد که پاسخ‌دهی ۲۰۰، هدرهای کش، عدم وجود رنگ‌های هاردکد در خروجی HTML و صفر بودن جاوااسکریپت اضافی روی صفحات محتوایی را بررسی و تأیید می‌کند.
- بررسی نوع `pnpm typecheck`، آزمون‌های سراسری `pnpm test`، اعتبارسنجی پلتفرم `make verify-psychology` و `pnpm lint` همگی با موفقیت کامل پاس شدند.

### ۲۰۲۶-۱۱-۲۱ — اجرای تسک PSY-5.8.1 (تکمیل کامپوننت‌های پایه و تعاملی UI با React Aria و تقویم شمسی)

- پکیج مشترک `@psychology/ui` با وابستگی‌های تأییدشدهٔ `react-aria-components` و `@internationalized/date` توسعه یافت:
  - **تکمیل کامپوننت‌های پایه (Primitives):**
    - ارتقای کامپوننت `Button` با پشتیبانی از خصیصهٔ `isLoading` همراه با اسپینر استاندارد DaisyUI (`loading loading-spinner`) و غیرفعال‌سازی خودکار در زمان بارگذاری.
    - ارتقای کامپوننت `Alert` با پشتیبانی از عنوان اختیاری (`title`).
    - تثبیت کامپوننت‌های `Input`، `Textarea`، `Card` (با `CardHeader`, `CardTitle`, `CardDescription`, `CardContent`, `CardFooter`)، `Badge` و `EmptyState` با رعایت توکن‌های معنایی و چیدمان RTL منطقی.
  - **ساخت کامپوننت‌های تعاملی Headless (Interactive Components):**
    - `Modal`: دیالوگ دسترس‌پذیر بر پایه `ModalOverlay`, `Modal`, `Dialog`, `Heading` از React Aria با لایه ماسک محو، پشتیبانی از سایزهای `sm` تا `xl`، کلوز اکشن و دکمه‌های اقدام، کاملاً ایمن در SSR و بدون FOUC.
    - `Select`: دراپ‌داون فرم با پشتیبانی از انتخاب کنترل‌شده/غیرکنترل‌شده، برچسب، پیام خطا، لیست آیتم‌ها با استایل DaisyUI و تگ فرم مخفی استاندارد `<select>` برای پشتیبانی از ارسال فرم و رندر بدون جاوااسکریپت در SSR.
    - `Tabs`: ناوبری تب‌ها بر پایه React Aria سازگار با واریانت‌های `boxed`, `bordered`, `lifted` دیسی‌یوآی، تنظیم خودکار کلاس `tab-active` و رندر تب پنل فعال در SSR.
    - `PersianDatePicker`: پیکر تقویم شمسی جلالی بر پایه `I18nProvider locale="fa-IR-u-ca-persian"`، نمایش اعداد فارسی و نام ماه‌های شمسی (فروردین تا اسفند)، تقویم پاپ‌اور با حرکت میان ماه‌ها، و تولید خودکار اینپوت مخفی استاندارد ISO-8601 (`YYYY-MM-DD`) برای ذخیره در دیتابیس و ارسال به API.
  - **توابع کمکی کار با تاریخ شمسی و میلادی (`date-utils.ts`):**
    - `toISODateString`: تبدیل امن هر تقویم (شمسی یا میلادی) به رشتهٔ استاندارد ISO (`YYYY-MM-DD`).
    - `parseISODate`: پارس رشته‌های تاریخ ISO به `CalendarDate`.
    - `formatPersianDate`: قالب‌بندی تاریخ‌های میلادی و جلالی به متن فارسی خوانا (مانند «۳۰ آبان ۱۴۰۵»).
    - `toPersianCalendarDate`: تبدیل تاریخ ورودی به شیء تقویم جلالی.
    - `getTodayPersian`: دریافت تاریخ امروز در گاه‌شماری هجری شمسی بر اساس منطقه زمانی ایران (`Asia/Tehran`).
- آزمون‌های واحد و یکپارچگی SSR در `packages/ui/test/interactive.test.mjs` نوشته و اجرا شدند (۱۵ تست موفق در سطح پکیج UI).
- اجرای دستورهای `make verify-psychology` و `pnpm lint` با موفقیت صددرصدی و بدون هیچ خطایی تأیید شد.

- سناریوی کامل آزمون پذیرش `PSY-A12` در دو لایهٔ بک‌اند و فرانت‌اند پیاده‌سازی و تأیید شد:
  - **لایهٔ Go / Backend (`apps/psychology-api/internal/testsupport/acceptance_a12_test.go`):**
    - دو مطب و دامنهٔ مستقل تعریف شدند: مطب A با دامنهٔ `rezaei-clinic.ir` و مطب B با دامنهٔ `niloofar-psych.ir`.
    - در وضعیت اولیه هر دو کلینیک دارای قالب `minimal` و رنگ‌های پیش‌فرض بودند و پاسخ‌های عمومی صفحهٔ اصلی در پروکسی کش با وضعیت `MISS` و سپس `HIT` کش شدند.
    - مالک مطب A پیش از ذخیره، قالب‌های `minimal` و `clinical` را روی `/preview` بررسی کرد. درخواست‌های پیش‌نمایش به دلیل ماهیت خصوصی و هدر `Cache-Control: private, no-store`، کش اشتراکی را دور زدند (`X-Cache: BYPASS`) و هیچ رکوردی در دیتابیس یا کش عمومی تغییر نکرد.
    - مالک مطب A با درخواست `PUT /v1/admin/site` قالب `clinical`، رنگ `#0f766e`، شعاع `lg` و مقیاس `sm` را ذخیره کرد. پاسخ با `private, no-store` بازگشت و رویداد ابطال Outbox در همان تراکنش ثبت شد.
    - ورکر Outbox رویداد را پردازش کرد و فقط کش دامنهٔ A را پاک کرد. درخواست بعدی دامنهٔ A با وضعیت `MISS` به سرور متصل شده و قالب جدید `clinical` را دریافت کرد، در حالی که کش دامنهٔ B همچنان با وضعیت `HIT` و قالب قبلی `minimal` دست‌نخورده باقی ماند (ایزولاسیون صددرصدی چندمستأجری).
    - آزمون امنیت مرز مستأجران تأیید کرد تلاش مالک A برای تغییر تنظیمات کلینیک B با خطای `403 Forbidden` متوقف شده و داده‌های B کاملاً محافظت شده‌اند.
  - **لایهٔ Astro / SSR Frontend (`apps/psychology-site/test/acceptance-a12.test.mjs`):**
    - رندر زنده روی دو Host ورودی شبیه‌سازی شد.
    - مسیر `/preview` با موفقیت هدر `Cache-Control: private, no-store`، متاتگ `<meta name="robots" content="noindex, nofollow">` و قالب‌های `minimal` و `clinical` را رندر کرد.
    - تست Fallback تأیید کرد در صورت ارسال شناسهٔ ثبت‌نشده در رجیستری، پوسته به صورت امن به `minimal` بازمی‌گردد.
    - تست جلوگیری از FOUC و تزریق CSS تأیید کرد رنگ‌های نامعتبر به رنگ پیش‌فرض `#0284c7` فال‌بک می‌شوند.
    - بررسی صفحهٔ اصلی `/` تأیید کرد پارامترهای ظاهری کوئری استرینگ را نادیده گرفته و همواره ظاهر منتشرشده را با کش عمومی ارائه می‌دهد.
- تمامی تست‌های پلتفرم با `make verify-psychology` و `pnpm lint` اجرا شده و کل پلتفرم بدون هیچ خطایی سبز است. موفقیت این سناریو مسیر را برای آغاز `PSY-5.8` فراهم کرد.

### ۲۰۲۶-۱۱-۲۱ — اجرای تسک PSY-5.6 (اتصال settings.astro و AdminLayout.astro به UI مشترک، کاتالوگ قالب‌ها، پیش‌نمایش و ذخیره API)

- در `apps/psychology-admin/src/layouts/AdminLayout.astro`، استایل پایه پکیج مشترک `@psychology/ui/styles/base.css` لود شد و ساختار سایدبار و نوار بالا با توکن‌های DaisyUI و کلاس‌های معنایی Tailwind متصل شد.
- صفحهٔ `apps/psychology-admin/src/pages/settings.astro` به طور کامل با سیستم طراحی `@psychology/ui` بازنویسی شد:
  - **بخش انتخاب قالب:** اتصال مستقیم به `TEMPLATE_CATALOG` برای انتخاب میان `minimal` و `clinical` با نمایش کارت، توضیحات فارسی و نسخهٔ معتبر.
  - **بخش هویت بصری:** انتخاب رنگ سازمانی با پالت‌های آماده `PRESET_PRIMARY_COLORS` و ورودی رنگ سفارشی هگز همراه با اعتبارسنجی زنده؛ گزینه‌های شعاع گوشه از `RADIUS_OPTIONS` (`sm`, `md`, `lg`) و مقیاس اندازه از `SCALE_OPTIONS` (`sm`, `md`, `lg`).
  - **پیش‌نمایش قبل از ذخیره:** پیوند و دکمهٔ «پیش‌نمایش زنده در وب‌سایت» که به صورت بلادرنگ همگام با تغییرات فرم، آدرس `/preview?template=...&primary=...&radius=...&scale=...` را به‌روزرسانی کرده و امکان مشاهده را قبل از انتشار فراهم می‌کند.
  - **خواندن و ذخیره از API:** دریافت مقادیر تنظیمات در SSR از `/v1/admin/site` و ذخیرهٔ فرم با درخواست `PUT` همراه با هدرهای امنیتی، نمایش وضعیت بارگذاری، و بازخورد آلرت‌های موفقیت و خطا.
- مسیر واسط `apps/psychology-admin/src/pages/api/admin/site.ts` برای هندل کردن درخواست‌های `GET` و `PUT` تنظیمات با هدر `Cache-Control: private, no-store` ایجاد شد.
- آزمون‌های جامع در `apps/psychology-admin/test/settings.test.mjs` پیاده‌سازی و اجرا شدند و `make verify-psychology` و `pnpm lint` با موفقیت کامل سبز شدند.

### ۲۰۲۶-۱۱-۲۱ — اجرای تسک PSY-5.5 (اتصال خانه به داده عمومی Tenant و ساخت /preview با هدرهای امن و noindex)

- ماژول کمکی `apps/psychology-site/src/lib/site.ts` با تابع `fetchPublicSite(request)` ساخته شد که با استفاده از `PsychologyApiClient` و هدر `Host` ورودی، تنظیمات عمومی منتشرشدهٔ همان کلینیک را از `/v1/public/site` دریافت می‌کند و در صورت عدم دسترسی موقت بک‌اند، به داده‌های پیش‌فرض امن برمی‌گردد.
- صفحهٔ خانهٔ `apps/psychology-site/src/pages/index.astro`:
  - هدر کش عمومی `Cache-Control: public, max-age=0, s-maxage=3600, stale-while-revalidate=60` را تنظیم می‌کند.
  - منحصراً ظاهر و تنظیمات منتشرشدهٔ کلینیک (`template_id`, `primary_color`, `theme_radius`, `theme_scale`) را اعمال کرده و طبق بند ۴ الزامات معماری، هرگونه پارامتر ظاهری در کوئری استرینگ را نادیده می‌گیرد.
  - کامپوننت `HomeHero` قالب فعال را همراه با کامپوننت پایه `Button` از `@psychology/ui` در بخش اقدام سریع رندر می‌کند.
- صفحهٔ پیش‌نمایش `apps/psychology-site/src/pages/preview.astro`:
  - از همان داده‌های محتوایی عمومی و چیدمان صفحهٔ خانه استفاده می‌کند، اما متغیرهای ظاهری (`template`, `primary`, `radius`, `scale`) را از Query اعتبارسنجی می‌کند.
  - در صورت ارسال مقادیر نامعتبر یا قالب ناشناخته، با fallback تضمین‌شده به تنظیمات منتشرشده یا پیش‌فرض‌های امن (`minimal`, `#0284c7`, `md`, `md`) برمی‌گردد.
  - هدر کش خصوصی `Cache-Control: private, no-store` را ارسال می‌کند تا به هیچ عنوان در CDN یا حافظهٔ کش مشترک ذخیره نشود.
  - تگ متا `<meta name="robots" content="noindex, nofollow">` در لایهٔ `BaseLayout.astro` رندر می‌شود تا در موتورهای جستجو ایندکس نشود.
  - بنر متمایز حالت پیش‌نمایش در بالای صفحه نمایش داده می‌شود.
- در `apps/psychology-site/src/pages/sitemap.xml.ts`، مسیر `/preview` به طور کامل از سایت‌مپ مستثنی است.
- آزمون‌های جامع در `apps/psychology-site/test/preview.test.mjs` اضافه و اجرا شدند و `make verify-psychology` و `pnpm lint` با موفقیت کامل پاس شدند.

### ۲۰۲۶-۱۱-۲۱ — اجرای تسک PSY-5.4 (قرارداد، ViewModelها، رجیستری، دو قالب minimal و clinical و رندر SSR در BaseLayout)

- قرارداد تایپ‌اسکریپت `SiteTemplate` در `apps/psychology-site/src/templates/contract.ts` ایجاد شد که شامل ۶ کامپوننت پایه (`Header`, `Footer`, `HomeHero`, `CourseCard`, `WorkshopCard`, `BlogCard`) و فیلد `meta: TemplateMeta` متناظر کاتالوگ است.
- مدل‌های پیش‌قالب‌بندی‌شده (ViewModels) در `apps/psychology-site/src/templates/view-models.ts` تعریف شدند (`SiteHeaderVM`, `FooterVM`, `HomeHeroVM`, `CourseCardVM`, `WorkshopCardVM`, `BlogCardVM`) تا طبق قاعده ۸۰/۲۰ و الگوی ۰۴ مستندات، هیچ لاجیک مالی، تبدیل تاریخ، کوکی، نشست یا ارتباط مستقیم با API کلاینت داخل کامپوننت‌های بصری قالب قرار نگیرد.
- دو مجموعه قالب متمایز با استایل و چیدمان کاملاً اختصاصی ساخته شدند:
  - `minimal`: طراحی آرام، ساده با فونت خلوت، استایل گرد و لطیف، حاشیه‌های نرم، فایل استایل `theme.css` با تم `data-theme="minimal"`.
  - `clinical`: طراحی مقتدر و بالینی کلینیک با هدر اعلان، کارت‌های ساختاریافته، فوتر خدمات و مجوزها، فایل استایل `theme.css` با تم `data-theme="clinical"`.
  - هر دو قالب از توکن‌های معنایی دیسی‌یوآی و جهت‌یابی منطقی راست‌به‌چپ (RTL logical properties نظیر `ms-`, `me-`, `ps-`, `pe-`, `start-`, `end-`) بدون هیچ رنگ هاردکد شده استفاده می‌کنند.
- رجیستری مرکزی در `apps/psychology-site/src/templates/registry.ts` پیاده‌سازی شد و تابع `resolveTemplate(templateId)` با پشتیبانی از fallback تضمین‌شده و امن به `minimalTemplate` در صورت ارسال شناسهٔ نامعتبر، خالی یا null ایجاد شد.
- لایهٔ پایه `apps/psychology-site/src/layouts/BaseLayout.astro` به‌روزرسانی شد:
  - استایل‌های پایه `@psychology/ui/styles/base.css` و استایل‌های دو تم ایمپورت شدند.
  - خصیصه‌های `data-theme`، `data-radius` و `data-scale` بر روی المان `<html>` ست می‌شوند.
  - متغیر CSS رنگ اصلی `--color-primary` با اعتبارسنجی دقیق هگز برای جلوگیری از FOUC و تزریق CSS به صورت اینلاین رندر می‌شود.
  - کامپوننت‌های هدر و فوتر به صورت داینامیک از قالب فعال استخراج و با مقادیر ViewModel رندر می‌شوند.
- آزمون‌های جامع در `apps/psychology-site/test/templates.test.mjs` پیاده‌سازی شدند:
  - برابری دوطرفه و دقیق شناسه‌ها بین کاتالوگ `TEMPLATE_CATALOG` و رجیستری `TEMPLATES_REGISTRY`.
  - حضور و تعریف تمامی ۶ کامپوننت در هر قالب.
  - رندر سرور واقعی (SSR) با استخراج ویژگی‌های تم، مقیاس و استایل اینلاین.
  - عملکرد بی‌نقص fallback امن به قالب `minimal` در صورت درخواست قالب نامعتبر یا تهی.
- دستورهای `make verify-psychology` و `pnpm lint` اجرا شده و کل پلتفرم با موفقیت سبز است.

- فایل‌های مایگریشن `000008_add_site_template_settings.up.sql` و `000008_add_site_template_settings.down.sql` ایجاد شدند و ستون‌های `template_id` (با پیش‌فرض `minimal`)، `template_version` (با پیش‌فرض `1.0.0`)، `theme_radius` (با چک محدود به `sm`, `md`, `lg`) و `theme_scale` (با چک محدود به `sm`, `md`, `lg`) بدون تغییر مایگریشن‌های پیشین به جدول دارای RLS فعال `site_settings` اضافه شدند.
- کوئری‌های `internal/settings/queries.sql` به‌روزرسانی شده و خروجی‌های sqlc در `settingsdb` بازتولید گردیدند.
- در `internal/settings/service.go` اعتبارسنجی شناسهٔ قالب (`^[a-z0-9-]{2,40}$`)، شعاع و مقیاس تم پیاده‌سازی شد. نسخهٔ فعال قالب در زمان ذخیره از کاتالوگ فعال استخراج و ثبت می‌شود.
- در همان تراکنش ذخیرهٔ تنظیمات در `Store.WithTenantTx`، رویداد Outbox برای ابطال کش CDN دامنهٔ تأییدشدهٔ همان کلینیک (`cache.InsertCacheInvalidationEvent`) با مسیرهای متأثر ثبت می‌شود و هدر `Cache-Control: private, no-store` روی پاسخ‌های پنل تنظیم گردید.
- مشخصات OpenAPI در `packages/contracts/psychology/openapi.yaml` برای مدل‌های `PublicSiteResponse`، `SiteSettingsResponse` و `UpdateSiteSettingsRequest` به‌روزرسانی شده و اینترفیس‌های `@psychology/api-client` با تایپ‌های جدید بازتولید شدند.
- آزمون‌های اعتبارسنجی، ایزولاسیون چندمستأجری و تست‌های کلاینت در سراسر مونو‌ریپو پیاده‌سازی و اجرا شده و `make verify-psychology` با موفقیت کامل سبز شد.

### ۲۰۲۶-۱۱-۲۱ — اجرای تسک PSY-5.2 (ساخت template-catalog.ts برای minimal و clinical و افزودن توکن‌ها و کامپوننت‌های پایه)

- کاتالوگ مرکزی قالب‌ها در `packages/ui/src/template-catalog.ts` ایجاد شد که متادیتای دو قالب اولیه `minimal` (نسخه ۱.۰.۰، تم DaisyUI `minimal`، خلوت و مناسب درمانگران فردی) و `clinical` (نسخه ۱.۰.۰، تم DaisyUI `clinical`، ساختاریافته و مدرن برای کلینیک‌ها) را مطابق قانون ۱۲ `AGENTS.md` با نسخهٔ ثابت و توابع کمکی `isValidTemplateId`، `getTemplateMeta` و fallback امن `resolveTemplateMeta` فراهم می‌کند.
- تعاریف توکن‌ها در `packages/ui/src/tokens.ts` توسعه یافت: پالت رنگ‌های پیش‌فرض مجاز (`PRESET_PRIMARY_COLORS`) و گزینه‌های شعاع گردی (`RADIUS_OPTIONS`) و مقیاس (`SCALE_OPTIONS`) همراه با اعتبارسنجی مقادیر تم اضافه شدند.
- کامپوننت‌های پایهٔ مورد نیاز برای صفحهٔ خانه و فرم تنظیمات به پکیج مشترک افزوده شدند:
  - `Input` و `Textarea`: با کلاس‌های DaisyUI `input-bordered` و `textarea-bordered`، پشتیبانی از وضعیت خطا (`isError`) و اندازه‌ها.
  - `Card`, `CardHeader`, `CardTitle`, `CardDescription`, `CardContent`, `CardFooter`: ساختار معنایی کارت با کلاس‌های DaisyUI و چیدمان RTL منطقی (`card-actions justify-end`).
  - `Badge`: انواع وضعیت‌ها و رنگ‌های تم با کلاس‌های معنایی DaisyUI.
  - `Alert`: ساختار هشدار استاندارد با ویژگی دسترسی‌پذیری `role="alert"`.
  - `EmptyState`: کامپوننت پیام خالی بودن داده‌ها برای لیست‌ها و بخش‌های مختلف.
- آزمون‌های واحد جامع در `packages/ui/test/render.test.mjs` پیاده‌سازی شدند: اعتبارسنجی متادیتا و fallback امن کاتالوگ، بررسی مقادیر توکن‌ها، و رندر سمت سرور (SSR) تمام کامپوننت‌های ساخته‌شده با بررسی کلاس‌های DaisyUI و عدم خروج استایل نامعتبر.
- دستورهای `make verify-psychology` و `pnpm lint` با موفقیت کامل سبز شدند.

### ۲۰۲۶-۱۱-۲۱ — اجرای تسک PSY-5.1.4 (فعال‌سازی astro check و tsc --noEmit و به‌روزرسانی Dockerfileها برای packages/ui)

- بستهٔ `@astrojs/check` به عنوان وابستگی توسعه به `apps/psychology-site/package.json` و `apps/psychology-admin/package.json` افزوده شد و اسکریپت `typecheck` به `astro check && tsc --noEmit` تغییر یافت.
- وابستگی `@psychology/api-client` به عنوان وابستگی مستقیم به هردو اپ فرانت‌اند اضافه شد تا خطاهای ماژول در بررسی تشخیصی Astro برطرف گردند.
- فیلد `lessons_count` به اینترفیس `AdminCourseItem` در کلاینت API و قرارداد OpenAPI افزوده شد و مقادیر شبیه‌سازی در صفحات دوره‌ها اصلاح شدند.
- کست تایپ‌ها در کامپوننت بلاگ سایت (`apps/psychology-site/src/pages/blog/[slug].astro`) برای سازگاری کامل با کامپایلر JSX استرو بهینه شد و متغیرهای بلااستفاده در پنل مدیریت پاکسازی شدند.
- فایل‌های Dockerfile هر دو اپلیکیشن (`apps/psychology-site/Dockerfile` و `apps/psychology-admin/Dockerfile`) به‌روزرسانی شدند تا پوشهٔ `packages/ui` را کپی کرده و دستور `pnpm --filter @psychology/ui build` را پیش از بیلد هر اپلیکیشن اجرا کنند.
- دستورهای `make verify-psychology` و `pnpm lint` با اجرای تست‌های Go و تست‌های کامل SSR و کلاینت در مونو‌ریپو با موفقیت سبز شدند و گروه تسک `PSY-5.1` به طور کامل به پایان رسید.

### ۲۰۲۶-۱۱-۲۱ — اجرای تسک PSY-5.1.3 (اتصال psychology-admin به بستهٔ مشترک و @astrojs/react و تأیید رندر SSR کامپوننت)

- بستهٔ `@psychology/ui` به عنوان وابستگی workspace و `@astrojs/react` سازگار با Vite 6 و Astro 5 (`^4.4.2`) همراه با `react` و `react-dom` به `apps/psychology-admin/package.json` اضافه شدند.
- پیکربندی Astro در `apps/psychology-admin/astro.config.mjs` به ادغام `react()` مجهز شد و تنظیمات JSX (`react-jsx`) در `tsconfig.json` فعال گردید.
- کامپوننت `Button` از `@psychology/ui` در بخش دسترسی‌های سریع داشبورد پنل (`apps/psychology-admin/src/pages/index.astro`) جایگزین لینک‌های استایل‌دار قبلی شد.
- آزمون رندر سمت سرور در `apps/psychology-admin/test/ssr-render.test.mjs` پیاده‌سازی شد و خروجی بیلد سروری را با بازگشت پاسخ ۲۰۰ و رندر کامل تگ `<button class="btn btn-primary ...">` تأیید کرد.
- دستورهای `make verify-psychology` و `pnpm lint` در سراسر مونو‌ریپو با موفقیت کامل سبز شدند.

### ۲۰۲۶-۱۱-۲۱ — اجرای تسک PSY-5.1.2 (اتصال psychology-site به بستهٔ مشترک و @astrojs/react و تأیید رندر SSR کامپوننت)

- بستهٔ `@psychology/ui` به عنوان وابستگی workspace و `@astrojs/react` سازگار با Vite 6 و Astro 5 (`^4.4.2`) همراه با `react` و `react-dom` به `apps/psychology-site/package.json` اضافه شدند.
- پیکربندی Astro در `apps/psychology-site/astro.config.mjs` به ادغام `react()` مجهز شد و تنظیمات JSX (`react-jsx`) در `tsconfig.json` فعال گردید.
- کامپوننت `Button` از `@psychology/ui` در بخش اکشن‌های صفحهٔ اصلی (`apps/psychology-site/src/pages/index.astro`) جایگزین تگ‌های ثابت شد.
- آزمون رندر سمت سرور در `apps/psychology-site/test/ssr-render.test.mjs` پیاده‌سازی شد و خروجی بیلد سروری را با بازگشت پاسخ ۲۰۰ و رندر کامل تگ `<button class="btn btn-primary ...">` تأیید کرد.
- دستورهای `make verify-psychology` و `pnpm lint` در سراسر مونو‌ریپو با موفقیت کامل سبز شدند.

### ۲۰۲۶-۱۱-۲۱ — اجرای تسک PSY-5.1.1 (ساخت بستهٔ @psychology/ui با Tailwind CSS v4، DaisyUI v5 و کامپوننت پایه SSR)

- بستهٔ مشترک `packages/ui` با نام `@psychology/ui` در ساختار مونو‌ریپو ایجاد و به `pnpm-workspace.yaml` متصل شد.
- استایل پایه `src/styles/base.css` با `@import "tailwindcss";` و `@plugin "daisyui";` و مقادیر متغیرهای سفارشی `data-radius` و `data-scale` پیکربندی شد.
- توکن‌های مجاز تم شامل مقادیر مجاز `theme_radius` (`sm`, `md`, `lg`)، `theme_scale` (`sm`, `md`, `lg`) و اعتبارسنجی هگز رنگ در `src/tokens.ts` پیاده‌سازی شدند.
- ابزار ادغام ایمن کلاس‌ها (`cn`) با کتابخانه‌های تأییدشده `clsx` و `tailwind-merge` ایجاد شد.
- کامپوننت پایه `Button` با پشتیبانی از انواع متغیرهای DaisyUI (`primary`, `secondary`, `outline`, ...) و اندازه‌ها بدون هیچ کلاس غیراستاندارد یا وابستگی به جهت‌های فیزیکی (پایبندی کامل به قواعد RTL) پیاده‌سازی شد.
- تست واحد SSR در `test/render.test.mjs` با `renderToString` رندر بدون خطای سروری کامپوننت و اعتبارسنجی توکن‌ها را تأیید کرد.
- دستورهای `make verify-psychology` و `pnpm lint` در تمام ۵ پروژهٔ مونو‌ریپو با موفقیت کامل سبز شدند.

### ۲۰۲۶-۱۱-۲۱ — اجرای تسک PSY-5.0.2 (اتصال pool نقش runtime، domain.Resolver، auth.Service و settings.Handler در cmd/server/main.go و تأیید روت‌ها)

- در `cmd/server/main.go`، ارتباط با دیتابیس از طریق `pgxpool.Pool`، `db.Store`، `domain.Resolver`، `auth.Service`، `auth.Handler` و `settings.Handler` متصل شد و روت‌های تنظیمات روی شاخهٔ `/v1` سوار شدند (`/v1/public/site` و `/v1/admin/site`).
- آزمون یکپارچگی ورودی سرور (`TestServer_SettingsEndToEnd_TwoHostsAndAuth`) در `cmd/server/main_test.go` پیاده‌سازی شد:
  - سناریوی Host A (`omid-psych.ir`) و Host B (`mahan-clinic.ir`): رندر و بازگشت موفق پروفایل عمومی مستقل با کد ۲۰۰.
  - رد قاطع Host ناشناخته (`unknown.ir`) با خطای ۴۰۴ `DOMAIN_NOT_FOUND`.
  - رد Host تأییدنشده (`unverified.ir`) با خطای ۴۰۴ `DOMAIN_NOT_VERIFIED`.
  - رد درخواست‌های پنل بدون احراز هویت با خطای ۴۰۱ `UNAUTHORIZED`.
  - رد تلاش دستکاری چندمستأجری (مالک کلینیک A با نشست معتبر قصد تغییر کلینیک B را داشته باشد) با خطای ۴۰۳ `FORBIDDEN`.
  - خواندن و ذخیرهٔ مجاز توسط مالک A روی کلینیک A و مشاهدهٔ بلافاصلهٔ تغییرات در خروجی عمومی Host A، بدون کوچک‌ترین تغییر در پروفایل یا تنظیمات Host B.
- آزمون اختیاری اتصال زنده `TestServer_LiveSettingsWithRealPool` در محیط دارای `DATABASE_URL` اضافه شد.
- با تکمیل این زیرتسک، تسک والد `PSY-5.0` به‌طور کامل پایان یافت و تیک خورد.
- `make verify-psychology` و `pnpm lint` با موفقیت کامل و بدون خطا سبز شدند.

### ۲۰۲۶-۱۱-۲۱ — اجرای تسک PSY-5.0.1 (اجرای خواندن و نوشتن internal/settings در Store.WithTenantTx و آزمون RLS با نقش runtime)

- سرویس `internal/settings` به اینترفیس `TenantStore` و متد `NewServiceWithStore` مجهز شد تا تمام عملیات خواندن و نوشتن (`GetPublicProfile`، `GetSettings`، `UpdateSettings`) درون `Store.WithTenantTx` و با تنظیم `app.tenant_id` اجرا شوند.
- مسیر عمومی `handleGetPublicSite` در `internal/settings/http.go` زمینهٔ درخواست را با `tenantctx.With(ctx, tenantID)` غنی کرد تا در سطح درخواست عمومی نیز context دارای شناسهٔ معتبر مستأجر باشد.
- آزمون‌های نگهبان شِما و ایزولاسیون زنده در `apps/psychology-api/internal/settings/isolation_test.go` پیاده‌سازی شدند:
  - راستی‌آزمایی اجباری بودن `FORCE ROW LEVEL SECURITY` و خط‌مشی `tenant_isolation` روی جدول `site_settings`.
  - راستی‌آزمایی ایزولاسیون کامل Tenant A و Tenant B در سناریوی زنده دیتابیس با نقش runtime (`app_user`): عدم دسترسی B به ردیف‌های A، بی‌اثر بودن تلاش‌های UPDATE/DELETE خام (۰ ردیف متأثر)، و خطای ۴۲۵۰۱ در درج متقلبانه.
  - تست انتشار خطا و اجرای در تراکنش `TestServiceWithStore_ErrorPropagation` در `settings_test.go`.
- `make verify-psychology` و `pnpm lint` با موفقیت کامل و بدون خطا سبز شدند.

### ۲۰۲۶-۱۱-۲۱ — بازبینی ترتیب و قرارداد فاز PSY-5

- اتصال API تنظیمات و آزمون RLS با نقش runtime به ابتدای فاز منتقل شد. کاتالوگ مشترک، معنای نسخهٔ قالب، بررسی Astro و قرارداد پیش‌نمایش و Outbox در اسناد ثبت شدند. تکمیل و تأیید مسیر اولیه با دو Host پیش‌نیاز مهاجرت سایر صفحات است. [تصمیم‌ها](DECISIONS.md) · [پذیرش](ACCEPTANCE.md)
- `make verify-psychology` موفق بود. تست Migration زنده به‌دلیل تنظیم نبودن `DATABASE_URL` اجرا نشد. `pnpm lint` با اسکریپت‌های فعلی که بررسی lint انجام نمی‌دهند، موفق بود.

### ۲۰۲۶-۱۱-۲۱ — مستندسازی فاز PSY-5

- الگوی رابط کاربری، مشخصات ماژول، تصمیم `PSY-D09`، تسک‌های `PSY-5.1` تا `PSY-5.9` و سناریوی `PSY-A12` ثبت شدند. [تصمیم‌ها](DECISIONS.md) · [پذیرش](ACCEPTANCE.md)
- `make verify-psychology` موفق بود. تست Migration روی PostgreSQL واقعی به‌دلیل تنظیم نبودن `DATABASE_URL` اجرا نشد و `pnpm lint` با اسکریپت‌های فعلیِ بدون بررسی موفق بود.

### ۲۰۲۶-۱۱-۲۱ — اجرای تسک PSY-4.4 (پایان فاز ۴ و نقشه راه · ثبت نتایج پایلوت‌ها، هزینه عملیاتی، قیمت اشتراک و سناریوی پذیرش PSY-A11)

- **گزارش جامع پایلوت‌های دو کلینیک مستقل (`docs/psychology/PILOT_EVIDENCE.md`):**
  - کلینیک روان‌شناسی امید (`omid-psych.ir`): ثبت‌نام کارگاه رایگان، مقالات بلاگ چندرسانه‌ای، نوبت‌دهی چنددرمانگر (دکتر علوی و دکتر راد)، تفکیک کامل تقویم‌ها بدون تداخل، و کش لبه با پاسخ‌های زیر ۱۰ میکروثانیه.
  - مرکز مشاوره ماهان (`mahan-clinic.ir`): کارگاه پولی، فروش دوره‌های آموزشی با ویدیو، پرداخت بانکی زرین‌پال، Zero-Oversell اثبات‌شده با رقابت هم‌زمان برای صندلی آخر، استرداد وجه و مغایرت‌گیری بانکی خودکار (`refund_required`)، و استریم امن با URLهای امضاشده ۳۰ دقیقه‌ای.
- **محاسبه دقیق هزینه‌های عملیاتی واقعی (Direct Operating Costs):**
  - سهم سرور اختصاصی/VPS کلاستر پارس‌پک: ۱۲,۰۰۰ تا ۱۵,۰۰۰ تومان در ماه به ازای هر کلینیک (قابلیت میزبانی بیش از ۵۰ کلینیک هم‌زمان با استک سبک Go و Astro).
  - فضای ذخیره‌سازی ابری S3 پارس‌پک (سهمیه استاندارد ۱۰ گیگابایت): ۲۰,۰۰۰ تومان در ماه به ازای هر کلینیک.
  - ترافیک شبکه و CDN آروان‌کلاد: ۳۰,۰۰۰ تا ۵۰,۰۰۰ تومان در ماه (کاهش بیش از ۸۵٪ ترافیک سرور اصلی به دلیل کش لبه HTML).
  - مجموع هزینه مستقیم عملیاتی هر کلینیک: حدود ۶۵,۰۰۰ تا ۸۵,۰۰۰ تومان در ماه.
- **مدل قیمت‌گذاری اشتراک نرم‌افزار (SaaS Tiers):**
  - **پلن پایه (Basic / مطب تک‌درمانگر):** ۲۹۰,۰۰۰ تومان در ماه (۲,۹۰۰,۰۰۰ تومان سالانه) با سهمیه ۵ گیگابایت رسانه و حاشیه سود ناخالص ۷۴.۱٪.
  - **پلن حرفه‌ای (Pro / کلینیک چنددرمانگر و آموزش):** ۵۹۰,۰۰۰ تومان در ماه (۵,۹۰۰,۰۰۰ تومان سالانه) با سهمیه ۱۰ گیگابایت رسانه، دوره‌ها، داشبورد مدیریتی و حاشیه سود ناخالص ۸۷.۳٪.
  - **بسته‌های فضای مازاد رسانه:** هر ۱۰ گیگابایت اضافه معادل ۵۰,۰۰۰ تومان در ماه.
- **مجموعه آزمون پذیرش سراسری فاز ۴ (`apps/psychology-api/internal/testsupport/acceptance_phase4_test.go`):**
  - سناریوی پذیرش `PSY-A11`: راستی‌آزمایی رفتار HIT/MISS کش لبه، خروج قاطع درخواست‌های پویا و شخصی با هدر `X-Cache: BYPASS`، صدور رویداد Outbox و پاک‌سازی کش اختصاصی کلینیک امید بدون کوچک‌ترین اثرگذاری روی کلینیک ماهان (Host-Scoped Invalidation).
  - آزمون `TestAcceptance_PSY_4_FullOperationalCycle`: اعتبارسنجی پوشش اجباری RLS روی هر ۲۴ جدول سیستم، اعتبارسنجی سهمیه ۱۰ گیگابایت و محاسبات سودآوری بالای ۷۰ درصد در پلن‌های اشتراک.
- **تأیید نهایی کل نقشه راه:** اجرای کامل و موفق `make verify-psychology` شامل تمام آزمون‌های واحد، آزمون‌های پذیرش سراسری، تست‌های مهاجرت و دیتابیس، تایپ‌چک و بیلد Astro در سرتاسر مونو‌ریپو.

- **سنجش و بنچ‌مارک کارایی کش HTML (`apps/psychology-api/internal/platform/cache/benchmark_test.go`):**
  - شبیه‌سازی و اندازه‌گیری تأخیر رندر SSR (۵۰.۳۸ میلی‌ثانیه برای واکشی و رندر قالب) در مقایسه با بازگشت مستقیم از حافظه لبه کش (۶.۷۹ میکروثانیه).
  - اثبات شتاب‌گیری بیش از ۷,۰۰۰ برابری و رسیدن به نرخ بیش از ۲.۴ میلیون پاسخ در ثانیه در لبه شبکه بدون هیچ سربار و مصرف پردازنده یا اتصال روی پایگاه داده.
- **معماری کلید کش و تفکیک مسیرها (`apps/psychology-api/internal/platform/cache/key.go`):**
  - تعریف کلید کش استاندارد `Host + Path` (نرمال‌سازی هاست، حذف پورت، و تمیزکاری اسلش‌ها).
  - تعریف صریح مسیرهای مجاز کش عمومی: `/` (خانه)، `/therapists` و `/booking` (پروفایل درمانگران)، `/blog` و `/blog/*` (فهرست و مقالات منتشرشده)، `/workshops` و `/courses`.
  - اعمال قانون سخت‌گیرانه BYPASS برای:
    - کلیه درخواست‌های دارای Query String (`?search=...`, `?page=...`, `?hash=...`).
    - مسیرهای اختصاصی: `/admin*`, `/account*`, `/appointments*`, `/checkout*`, `/callbacks*`, `/buyer*`, استریم‌های ویدیو و پیگیری ثبت‌نام‌ها.
    - درخواست‌های دارای کوکی نشست (`buyer_session_token`, `admin_session`) یا هدر `Authorization`.
    - متدهای تغییر داده (POST, PUT, DELETE, PATCH).
    - هدرهای `Cache-Control: no-cache` یا `Pragma: no-cache` سمت کلاینت.
- **شبیه‌ساز و میان‌افزار کش لبه (`apps/psychology-api/internal/platform/cache/proxy.go`):**
  - پیاده‌سازی `EdgeCacheProxy` با ثبت هدرهای `X-Cache: HIT`، `X-Cache: MISS`، `X-Cache: BYPASS` و زمان ماندگاری `Age`.
  - متد `Purge(host, paths)` با تضمین تفکیک سرسخت چندمستأجری: ابطال آدرس‌های کلینیک A هیچ اثری روی رکوردهای کش‌شده کلینیک B نمی‌گذارد.
- **طراحی جدول و مایگریشن Outbox (`migrations/000007_create_outbox_events.up.sql`):**
  - جدول `outbox_events` به عنوان جدول ۲۴ام سیستم دارای RLS اجباری (`FORCE ROW LEVEL SECURITY`) و خط‌مشی ایزولاسیون با `current_setting('app.tenant_id', true)`.
  - ایندکس‌های بهینه برای رویدادهای `pending` و واکشی ترتیبی بر مبنای زمان ساخت.
  - ثبت رویداد در همان تراکنش تغییر داده (AP-11) با `InsertCacheInvalidationEvent`.
- **ورکر پردازش و ارسال ابطال به CDN (`apps/psychology-api/internal/platform/cache/worker.go`):**
  - اینترفیس `CDNPurger` و پیاده‌سازی برای آروان‌کلاد (`ArvanCloudPurger`) با آدرس‌های کامل Canonical.
  - پیاده‌سازی `OutboxWorker` با مکانیزم Retry تا ۳ بار و ثبت خطاها در ستون `last_error`.
- **تنظیم هدرهای کش در Astro SSR و Caddyfile:**
  - افزودن هدر `Cache-Control: public, max-age=0, s-maxage=3600, stale-while-revalidate=60` به صفحات خانه و بلاگ استورفرانت.
  - افزودن هدر `no-store, no-cache, must-revalidate, private` به صفحات حساب خریداران و نوبت‌دهی.
  - به‌روزرسانی بلاک `@private` در `infrastructure/psychology/Caddyfile` و اسکریپت `restore.sh`.
- **آزمون‌های جامع و نگهبان‌ها (`cache_test.go` و `migration_test.go`):**
  - آزمون سناریوهای HIT و MISS.
  - آزمون کلیه سناریوهای ۱۳گانه BYPASS.
  - آزمون ایزولاسیون کلینیک‌های مستقل در پاک‌سازی کش (`TestEdgeCache_MultiTenant_Host_Isolation`).
  - آزمون تراکنشی Outbox و پردازش ورکر با پاک‌سازی موفق و سناریوی مدیریت شکست و ۳ بار Retry.
  - اعتبارسنجی RLS جدول `outbox_events` در `migration_test.go` و `deployment_test.go`.
- **تأیید کامل:** اجرای بدون خطای `make verify-psychology`.

- **ارتقای اسکریپت بازیابی از حادثه (`infrastructure/psychology/scripts/restore.sh`):**
  - گسترش بلوک خودکار راستی‌آزمایی RLS پس از بازیابی با `pg_restore`:
    - بررسی فعال بودن و اجباری بودن `ROW LEVEL SECURITY` (`relforcerowsecurity = true`) روی تمامی ۲۳ جدول چندمستأجری در سراسر فازهای ۱، ۲ و ۳:
      `domains`, `site_settings`, `memberships`, `workshops`, `workshop_registrations`, `purchases`, `payment_attempts`, `refunds`, `blog_posts`, `media_assets`, `media_uploads`, `therapists`, `therapist_working_hours`, `therapist_schedule_exceptions`, `appointment_slots`, `appointments`, `appointment_status_history`, `buyer_accounts`, `buyer_sessions`, `buyer_otp_codes`, `courses`, `course_lessons`, `course_entitlements`.
    - در صورت فقدان یا غیرفعال بودن RLS روی حتی یک جدول، اسکریپت با کد خروج ۱ متوقف شده و هشدار امنیتی ثبت می‌کند.
- **توسعه سرویس نگه‌داشت و پاک‌سازی اطلاعات حساس (`apps/psychology-api/internal/platform/retention/cleaner.go`):**
  - پیاده‌سازی متد `AnonymizeExpiredAppointmentData`:
    - بی‌نام‌سازی کامل مشخصات بیماران در نوبت‌های قدیمی‌تر از ۹۰ روز یا لغوشده قدیمی‌تر از ۹۰ روز (`guest_name = 'مراجع'`, `guest_phone = ''`, `guest_lookup_hash = ''`, `notes = ''`).
    - حفظ دست‌نخورده زمان‌بندی سانس‌ها و تاریخچه تغییر وضعیت نوبت جهت محاسبات آماری کلینیک.
  - پیاده‌سازی متد `PurgeExpiredOTPs`:
    - حذف فیزیکی دائمی (`DELETE`) کدهای یکبارمصرف پیامکی خریداران که منقضی یا مصرف شده‌اند.
  - پیاده‌سازی متد `PurgeExpiredBuyerSessions`:
    - حذف فیزیکی دائمی نشست‌های منقضی‌شده خریداران.
  - متد موجود `AnonymizeExpiredGuestData`:
    - بی‌نام‌سازی ثبت‌نام‌های کارگاه‌ها پس از ۹۰ روز، با شرط بسته بودن پرونده‌های مالی و عدم وجود مناقشه فعال بازپرداخت (`refund_required`). سوابق تراکنش در `purchases` و `refunds` کاملاً حفظ می‌شوند.
- **توسعه آزمون‌های واحد و یکپارچگی (`cleaner_test.go` و `deployment_test.go`):**
  - آزمون `TestRetentionCleaner_AppointmentDataScrubbing`: راستی‌آزمایی بی‌نام‌سازی نوبت‌های قدیمی، بی‌نام‌سازی نوبت‌های لغوشده و دست‌نخورده ماندن نوبت‌های جاری و آینده.
  - آزمون `TestRetentionCleaner_ExpiredOTPsAndSessionsPurge`: راستی‌آزمایی حذف فیزیکی OTPها و نشست‌های منقضی‌شده و حفظ نمونه‌های فعال.
  - آزمون `TestBackupAndRestoreScripts_Integrity`: اعتبارسنجی تطابق اسکریپت بازیابی با فهرست کامل ۲۳ جدول تفکیک‌شده.
- **به‌روزرسانی جامع مستندات شواهد و عملیات پشتیبانی:**
  - `docs/psychology/SUPPORT_RUNBOOK.md`: به‌روزرسانی سیاست‌های نگه‌داشت داده، راستی‌آزمایی ۲۳ جدول در بازیابی، دستورالعمل‌های دوره‌ها و استریم امن ۳۰ دقیقه‌ای، تحلیل هزینه فضای ذخیره‌سازی ابری و ترافیک CDN، و تعیین اهداف سطح خدمت (Uptime 99.5%, RPO < 24h, RTO < 30m).
  - `docs/psychology/PILOT_EVIDENCE.md`: ثبت شواهد تفصیلی فاز ۳ (سناریوهای PSY-A08 و PSY-A09)، نتایج تمرین مانور بازیابی، گزارش بازبینی سیاست نگه‌داشت و تحلیل سهمیه ۱۰ گیگابایتی.
- **تأیید کامل:** اجرای بدون خطای تمام آزمون‌های واحد و بسته deployment در `apps/psychology-api`.

- **کوئری‌های تجمیعی و ماژول گزارش‌ها (`apps/psychology-api/internal/reports`):**
  - طراحی کوئری‌های پارامتری‌شده sqlc در `queries.sql` برای محاسبه شاخص‌ها مستقیماً از جداول دارای RLS بر پایه `tenant_id`:
    - شاخص‌های کارگاه: تعداد کارگاه‌ها به تفکیک وضعیت (منتشر، پیش‌نویس، بسته، لغو) و کل ثبت‌نام‌ها (قطعی، در انتظار، لغوشده).
    - شاخص‌های نوبت حضوری: تعداد درمانگران فعال، سانس‌های کل و سانس‌های آزاد، کل نوبت‌ها به تفکیک وضعیت (تأییدشده، در انتظار، تکمیل‌شده، لغوشده).
    - شاخص‌های دوره‌های آموزشی: تعداد دوره‌ها و جلسات، تعداد دسترسی‌های فعال دوره (`active entitlements`) و دسترسی‌های لغوشده.
    - شاخص‌های مالی و درآمد خالص: تعداد خریدها، مبالغ ناخالص فروش، مبالغ استردادشده، و محاسبه درآمد خالص به ریال صحیح طبق D-14.
    - شاخص‌های خطا و درگاه: تعداد تلاش‌های موفق و ناموفق درگاه بانک و موارد نیازمند بازپرداخت (`refund_required`).
    - شاخص‌های رسانه و سهمیه ابری: حجم کل مصرفی رسانه‌ها، تفکیک عمومی و خصوصی، سقف سهمیه ۱۰ گیگابایت، درصد و پرمیل مصرف، و تعداد فایل‌های ویدیویی، صوتی و تصویری.
- **تضمین اکید عدم نشت اطلاعات هویتی و پزشکی (Zero PII):**
  - کلیه ساختارهای خروجی صرفاً شامل متغیرهای آماری عددی (`int64`) هستند. هیچ فیلد متنی فردی، نام، شماره تلفن، ایمیل، یادداشت بالینی، یا هش lookup در خروجی وجود ندارد.
- **کنترلر HTTP و امنیت دسترسی (`apps/psychology-api/internal/reports/http.go`):**
  - `GET /admin/reports/metrics`: احراز هویت ادمین، بررسی عضویت در کلینیک (`RequireTenantMembership`)، و محدودسازی دسترسی صرفاً به نقش مالک (`owner`).
  - هدرهای ضد کش خصوصی: `Cache-Control: private, no-cache, no-store, must-revalidate`.
- **قرارداد OpenAPI و کلاینت تایپ‌اسکریپت (`packages/contracts/psychology` و `@psychology/api-client`):**
  - افزودن مسیر `/admin/reports/metrics` و تایپ‌های `TenantMetricsReport` و متدهای مربوطه به کتابخانه کلاینت و تست‌های واحد آن.
- **داشبورد پنل مدیریت کلینیک (`apps/psychology-admin/src/pages/index.astro`):**
  - بازطراحی صفحه اصلی داشبورد ادمین برای نمایش شاخص‌های واقعی کلان کلینیک (درآمد خالص، تفکیک فروش کارگاه و دوره، نوبت‌های حضوری، نوار پیشرفت مصرف سهمیه ۱۰ گیگابایتی رسانه، پایش خطاهای درگاه و دسترسی‌های سریع) همراه با اطلاعیه تضمین حریم خصوصی مراجعان.
- **آزمون‌های جامع و نگهبان‌های امنیتی (`apps/psychology-api/internal/reports/reports_test.go`):**
  - `TestReportsService_CalculationsAndMetricsAccuracy`: اعتبارسنجی دقیق محاسبات تجمیعی و کسر استردادها از درآمد ناخالص.
  - `TestReportsService_ZeroPIIGuardian`: نگهبان امنیتی سخت‌گیرانه برای اسکن کل خروجی JSON و اطمینان از عدم وجود هرگونه کلید هویتی یا پزشکی.
  - `TestReportsService_CrossTenantIsolation`: آزمون ایزولاسیون کامل چندمستاجری (عدم نشت حتی یک عدد از کلینیک A به کلینیک B).
  - `TestReportsHTTP_RBACAndTenantMembership`: مسدودسازی درخواست‌های مهمان (401)، رد نقش‌های غیرمالک مانند درمانگر (403)، و رد تلاش مالک کلینیک دیگر برای دسترسی به آمار این کلینیک (403).
- **تأیید کامل:** اجرای بدون خطای `make verify-psychology` شامل تمام تست‌های Go و بررسی‌های تایپ‌چک و بیلد Astro در سرتاسر مونو‌ریپو.

### ۲۰۲۶-۱۱-۲۱ — اجرای تسک PSY-3.7 (پایان فاز ۳ · اجرای کامل خرید تا پخش، حفظ دسترسی در آرشیو، بازپرداخت و سناریوهای پذیرش PSY-A08 و PSY-A09)

- **پیاده‌سازی مجموعه آزمون پذیرش جامع فاز ۳ (`apps/psychology-api/internal/testsupport/acceptance_phase3_test.go`):**
  - **سناریوی پذیرش PSY-A08 (چرخه کامل خرید دوره، استریم، آرشیو و بازپرداخت):**
    - آپلود فایل ویدیویی تأییدشده با کلاس دسترسی اختصاصی `course_private` در باکت رسانه.
    - ساخت دوره با پیش‌نویس اولیه، پیوند جلسه به فایل اختصاصی و انتشار رسمی دوره با اعتبارسنجی حداقل ۱ جلسه.
    - احراز هویت خریدار با ارسال پیامک و اعتبارسنجی کد یک‌بارمصرف OTP.
    - آغاز خرید (Checkout) با کلید Idempotency و هدایت به درگاه پرداخت.
    - بازگشت از درگاه بانک (Callback Verify): صدور و فعال‌سازی اتمیک دسترسی خریدار (`active entitlement`) در همان تراکنش.
    - استریم موفق جلسه آموزشی توسط خریدار تأییدشده همراه با هدرهای ضد کش خصوصی (`Cache-Control: private, no-cache, no-store, must-revalidate`).
    - آزمون Retry فراخوانی درگاه: فراخوانی مجدد Callback دقیقاً یک دسترسی برای خریدار باقی می‌گذارد و اثر مضاعف یا تکراری ندارد.
    - تلاش برای خرید مجدد دوره از پیش خریداری‌شده با خطای ۴۰۹ (`ALREADY_ENTITLED`) مسدود می‌شود.
    - آرشیو دوره توسط مالک کلینیک: فروش دوره به خریداران جدید فوراً متوقف می‌شود، اما خریداران قبلی همچنان دسترسی کامل و پایدار خود به پخش جلسات را حفظ می‌کنند.
    - ثبت استرداد وجه (`RecordRefund`): دسترسی کاربر فوراً به وضعیت `revoked` منتقل شده و از صدور هرگونه URL استریم جدید جلوگیری به‌عمل می‌آید (خطای ۴۰۳ `ENTITLEMENT_REVOKED`).
  - **سناریوی پذیرش PSY-A09 (امنیت، انقضای توکن، سهمیه و ایزولاسیون کامل چندمستأجری):**
    - مسدودسازی درخواست کاربر مهمان (Unauthenticated) برای استریم جلسات خصوصی با خطای ۴۰۱ (`UNAUTHORIZED`).
    - امکان دسترسی عمومی و بدون لاگین به جلسات دارای نشان پیش‌نمایش رایگان (`is_free_preview = true`) با صدور URL امن کوتاه‌مدت.
    - مسدودسازی استریم جلسات قفل روی آدرس پیش‌نمایش رایگان با خطای ۴۰۳ (`PREVIEW_FORBIDDEN`).
    - مسدودسازی کاربر دیگری از همان کلینیک که دوره را نخریده است با خطای ۴۰۳ (`FORBIDDEN`).
    - ایزولاسیون سرسخت چندمستأجری (Cross-Tenant): تلاش برای دسترسی به دوره و جلسات کلینیک امید از طریق دامنه مرکز ماهان رد و مسدود می‌شود.
    - بررسی TTL کوتاه‌مدت URLهای امضاشده: طول عمر دقیقاً ۵ دقیقه (۳۰۰ ثانیه) است و مکانیزم تمدید خودکار (Renewal) لینک معتبر جدیدی را پس از انقضا یا درخواست مجدد صادر می‌کند.
    - جلوگیری از مصرف مجدد کد OTP (Single-Use Anti-Replay): استفاده دوباره از کد پیامکی مصرف‌شده با خطای اعتبارسنجی رد می‌شود.
    - رد آپلود انواع فایل ناسازگار: مسدودسازی فایل‌های پلی‌لیست (`.m3u8`, `.mpd`, `.pls`, `.ts`) با `ErrDisallowedPlaylist` و کانتینرهای نامعتبر (`.mkv`, `.avi`, `.flv`) با `ErrIncompatibleMediaContainer`.
    - بررسی سهمیه ذخیره‌سازی: جلوگیری از رزرو فایل فراتر از سقف مجاز مستاجر (۱۰ گیگابایت).
- **به‌روزرسانی هماهنگ در لایه پرداخت و وب (`apps/psychology-api/internal/payments` و `courses/http.go`):**
  - تعریف خطای صریح `ErrCourseAlreadyEntitled` در ماژول پرداخت.
  - بازگرداندن پاسخ استاندارد ۴۰۹ Conflict با کد `ALREADY_ENTITLED` در زمان تلاش برای خرید مجدد دوره‌ای که خریدار دسترسی فعال آن را دارد.
- **تأیید کامل و بی‌نقص:** اجرای بدون خطای `make verify-psychology` شامل تمام تست‌های Go (شامل آزمون‌های پایلوت و آزمون‌های فازهای ۱، ۲ و ۳ از `PSY-A01` تا `PSY-A10`)، بررسی‌های lint، تایپ‌چک و بیلد Astro در سرتاسر مونو‌ریپو.

### ۲۰۲۶-۱۱-۲۱ — اجرای تسک PSY-3.6 (پخش و تمدید خودکار URL فیلم و ویس، پشتیبانی Range و Seek، بهینه‌سازی اینترنت موبایل، و رد Playlistها و فایل‌های ناسازگار)

- **اعتبارسنجی رسانه و رد پلی‌لیست‌ها و کانتینرهای ناسازگار (`apps/psychology-api/internal/media/service.go`):**
  - پیاده‌سازی تابع `ValidateMediaFile`:
    - رد صریح پلی‌لیست‌های HLS/DASH و قطعات استریم (`.m3u8`, `.mpd`, `.pls`, `.m3u`, `.ts`) با خطای `ErrDisallowedPlaylist`.
    - رد انواع MIME مربوط به پلی‌لیست‌ها نظیر `application/x-mpegURL`, `application/vnd.apple.mpegurl`, `application/dash+xml`.
    - رد کانتینرهای ناسازگار ویدیویی و صوتی غیرمجاز (`.mkv`, `.avi`, `.flv`, `.wmv`, `.mov`, `.webm`, `.wav`, `.aac`, `.ogg`, `.flac`) با خطای `ErrIncompatibleMediaContainer`.
    - الزام تطابق پسوند فایل با نوع MIME مجاز: تنها `.mp4`/`.m4v` برای ویدیو (`video/mp4`) و `.mp3` برای صوت و ویس (`audio/mpeg`).
    - اعمال اعتبارسنجی در فاز درخواست آپلود موقت (`RequestStagingUpload`) و نهایی‌سازی.
- **تضمین سازگاری Range و Seek روی S3 و باکت پارس‌پک (`apps/psychology-api/internal/media/storage.go`):**
  - تضمین اینکه `PresignGetURL` در امضای AWS SigV4 تنها هدر `host` را در `X-Amz-SignedHeaders` امضا می‌کند تا درخواست‌های هدر `Range` مرورگر در زمان seek باعث خطای `SignatureDoesNotMatch` نشود.
  - اعتبارسنجی پاسخ‌های استاندارد RFC 7233 در باکت: `206 Partial Content` همراه با `Content-Range: bytes start-end/total` و `Accept-Ranges: bytes` برای بایت‌های ابتدا، میانه (Seek) و انتهای فایل، و بازگشت `416 Range Not Satisfiable` برای بازه‌های نامعتبر.
- **ارتقای خروجی استریم جهت تفکیک فیلم و ویس در کلاینت (`apps/psychology-api/internal/courses/buyer_service.go`):**
  - اضافه شدن فیلدهای `mime_type` و `size_bytes` و `expires_in_seconds` به ساختار `LessonStreamOutput` و `LessonStreamResponse` در قرارداد OpenAPI و کلاینت تایپ‌اسکریپت `@psychology/api-client`.
- **رابط پخش دوگانه (فیلم / ویس) و تمدید خودکار در مرورگر (`apps/psychology-site/src/pages/account/courses/[id].astro`):**
  - تشخیص خودکار نوع رسانه:
    - حالت ویدیو (`video/mp4`): نمایش پلیر ویدیویی واکنش‌گرا با `preload="metadata"`، نشانگر بافرینگ شبکه و کنترل‌های لمسی.
    - حالت ویس/پادکست (`audio/mpeg`): نمایش کامپوننت اختصاصی پخش صوت همراه با نشانگر جلسه صوتی، دکمه‌های پرش ۱۰ ثانیه‌ای به جلو و عقب (`↺ ۱۰ ثانیه قبل` و `↻ ۱۰ ثانیه بعد`) و انتخاب‌گر سرعت پخش (۰.۷۵x، ۱.۰x، ۱.۲۵x، ۱.۵x، ۲.۰x).
  - تمدید خودکار و نامحسوس لینک پخش:
    - رصد زمان انقضای توکن ۳۰۰ ثانیه‌ای (۵ دقیقه) و زمان‌بندی تمدید خودکار ۴۵ ثانیه پیش از پایان اعتبار.
    - حفظ و بازیابی دقیق موقعیت پخش (`currentTime`) در صورت خطا در seek یا تمدید خودکار بدون قطعی پخش کاربر.
    - بهینه‌سازی برای شبکه و اینترنت موبایل با جلوگیری از پیش‌بارگذاری سنگین (`preload="metadata"`).
- **آزمون‌های جامع خودکار (`apps/psychology-api/internal/media/media_test.go`):**
  - `TestPlaylistAndIncompatibleContainerRejection`: بررسی ۱۷ حالت رد پلی‌لیست‌ها (`.m3u8`, `.mpd`, `.pls`, `.ts`)، پسوندهای ناسازگار (`.mkv`, `.avi`, `.flv`, `.wmv`, `.wav`) و پذیرش فایل‌های معتبر MP4 و MP3.
  - `TestPresignedGetURL_RangeAndSeekSupport`: شبیه‌سازی کامل سرور S3 و آزمون درخواست‌های کامل، چانک اولیه موبایل (0-1023)، seek به میانه فایل (50000-74999) و رد بازه فراتر از حجم فایل (416).
  - `TestMediaStreamingOutputMimeAndSize`: بررسی متادیتای خروجی استریم برای تشخیص نوع پلیر در فرانت‌اند.
- **تأیید کامل:** اجرای بدون خطای `make verify-psychology` شامل تمام تست‌های Go و بررسی‌های تایپ‌چک و بیلد Astro.

### ۲۰۲۶-۱۱-۲۱ — اجرای تسک PSY-3.5 (حساب «دوره‌های من»، API امضای کوتاه‌مدت پخش رسانه خصوصی، و رابط پخش آنلاین)

- **سرویس صدور URL امن پخش رسانه دوره (`apps/psychology-api/internal/media/service.go`):**
  - پیاده‌سازی متد `GetCoursePrivateStreamURL`:
    - بررسی وضعیت تأیید فایل رسانه‌ای (`validation_status == 'verified'`).
    - اعتبارسنجی کلاس دسترسی: تنها فایل‌های با کلاس دسترسی `course_private` اجازه صدور لینک امضاشده خصوصی را دارند و دسترسی به فایل‌های دیگر با خطای `ErrMediaNotCoursePrivate` مسدود می‌شود.
    - تولید presigned URL با مدت اعتبار ۵ دقیقه (`StreamURLTTL = 300s`) روی فضای ابری S3.
- **منطق دسترسی و استریم درس در سرویس خریدار (`apps/psychology-api/internal/courses/buyer_service.go`):**
  - متد `GetPrivateLessonStreamURL`:
    - احراز هویت خریدار و اعتبارسنجی تعلق به Tenant جاری.
    - استعلام آخرین دسترسی خریدار به دوره (`GetCourseEntitlementByBuyerAndCourse`):
      - اگر خریدی ثبت نشده باشد -> خطای ۴۰۳ (`ErrNotPurchased`).
      - اگر دسترسی لغو یا استرداد شده باشد (`status == 'revoked'`) -> خطای ۴۰۳ (`ErrEntitlementRevoked`).
    - بررسی وضعیت دوره: دوره‌های پیش‌نویس (`draft`) مسدود می‌شوند، در حالی که دوره‌های `published` و دوره‌های قبلاً خریداری‌شده که اکنون `archived` شده‌اند به خریداران اجازه پخش جلسات را می‌دهند.
    - اعتبارسنجی تطابق جلسه با دوره و تعلق به همان مستاجر.
    - تولید URL کوتاه‌مدت پخش اختصاصی با مدت اعتبار ۳۰۰ ثانیه.
  - متد `GetPublicLessonPreviewStreamURL`:
    - بررسی دسترسی به جلسات پیش‌نمایش رایگان (`is_free_preview = true`) بدون نیاز به لاگین یا خرید.
    - رد درخواست استریم جلسات غیررایگان با خطای ۴۰۳ (`ErrLessonNotFreePreview`).
- **کنترلرهای HTTP استریم و دوره‌های من (`apps/psychology-api/internal/courses/http.go`):**
  - `GET /account/courses`: دریافت لیست دوره‌های خریداری‌شده خریدار لاگین‌شده با هدرهای `no-store, no-cache, must-revalidate, private`.
  - `GET /account/courses/{courseId}/lessons/{lessonId}/stream`: صدور URL امضاشده پخش برای درس خریده‌شده با هدرهای امنیتی ضد کش خصوصی.
  - `GET /public/courses/{slug}/lessons/{lessonId}/preview-stream`: صدور URL پیش‌نمایش رایگان جلسات عمومی با هدرهای `no-store`.
- **رابط کاربری و صفحات استورفرانت (`apps/psychology-site`):**
  - `src/pages/account/courses/index.astro`: صفحه «دوره‌های من» شامل تشخیص خودکار نشست خریدار، فرم ورود پیامکی یکپارچه با OTP، کارت‌های دوره‌های خریداری‌شده با قیمت ریال/تومان طبق D-14 و وضعیت دسترسی فعال/لغوشده.
  - `src/pages/account/courses/[id].astro`: پخش‌کننده ویدیویی/صوتی HTML5 با لیست سرفصل جلسات، دکمه تمدید خودکار لینک ۵ دقیقه‌ای و نمایش هشدارهای عدم دسترسی یا استرداد وجه.
  - افزودن پیوند «دوره‌های من» به هدر و ناوبری در `BaseLayout.astro`.
- **آزمون‌های جامع و اثبات ایزولاسیون (`apps/psychology-api/internal/courses/stream_test.go`):**
  - `TestLessonStreaming_Success_And_TTL`: بررسی صدور موفق لینک پخش با TTL پنج دقیقه‌ای و متادیتا.
  - `TestLessonStreaming_UnpurchasedBlocked`: رد درخواست پخش بدون خرید دوره با خطای ۴۰۳ (`NOT_PURCHASED`).
  - `TestLessonStreaming_OtherAccountBlocked`: رد درخواست کاربر دیگر با خطای ۴۰۳.
  - `TestLessonStreaming_CrossTenantIsolation`: رد قاطع دسترسی مستاجر B به جلسات یا خریدهای مستاجر A با خطای ۴۰۴/۴۰۳.
  - `TestLessonStreaming_RevokedEntitlementBlocked`: رد درخواست خریدار استردادشده با خطای ۴۰۳ (`ENTITLEMENT_REVOKED`).
  - `TestLessonStreaming_ArchivedCourse_ExistingBuyerKeepsAccess`: حفظ کامل دسترسی خریداران قدیمی به دوره پس از آرشیو شدن آن.
  - `TestLessonStreaming_FreePreview_PublicAccess`: دسترسی عمومی مهمان به جلسات پیش‌نمایش رایگان و رد جلسات قفل بدون لاگین.
- **تأیید کامل:** اجرای بدون خطای `make verify-psychology` شامل تمام تست‌های Go و بررسی‌های تایپ‌چک و بیلد Astro.

### ۲۰۲۶-۱۱-۲۱ — اجرای تسک PSY-3.4 (صفحه فروش و Checkout تک‌دوره، ایجاد اتمیک دسترسی در Verify، هندل پرداخت تکراری و استرداد وجه)

- **گسترش سرویس پرداخت برای فروش دوره (`apps/psychology-api/internal/payments/service.go`):**
  - افزودن اینترفیس `CoursePaymentQuerier` و متد `InitiateCoursePayment`:
    - بررسی وضعیت حساب خریدار (`active`) و دوره (`published` با مبلغ معتبر ریالی).
    - Snapshot مبلغ و عنوان جاری در رکورد `purchases` (`item_type = 'course'`).
    - مسدودسازی پرداخت جدید در صورتی که خریدار در حال حاضر `entitlement` فعال برای همان دوره دارد.
    - ثبت رکورد `course_entitlements` در وضعیت `pending`.
    - فراخوانی گیت‌وی پرداخت خارج از تراکنش دیتابیس با بازگشت آدرس درگاه بانکی.
  - یکپارچه‌سازی اتمیک در `HandleCallbackAndVerify`:
    - در صورت موفقیت تراکنش بانک، تغییر وضعیت خرید به `completed` و تغییر وضعیت `entitlement` به `active` و ثبت زمان `granted_at = now()` در همان تراکنش دیتابیس.
    - آزمون پرداخت تکراری هم‌زمان: در صورتی که کاربر قبلاً برای همان دوره دسترسی فعال دریافت کرده باشد، پرداخت دوم به وضعیت `refund_required` هدایت شده و هیچ entitlement تکراری فعال نمی‌شود.
  - لغو خودکار دسترسی هنگام استرداد در `RecordRefund`:
    - هنگام ثبت استرداد وجه برای سفارش‌های دوره، دسترسی مرتبط به وضعیت `revoked` با ثبت دلیل استرداد و زمان لغو انتقال می‌یابد.
    - تضمین کامل Idempotency با بررسی مرجع بانکی.
- **کنترلرهای HTTP دوره‌های عمومی و خرید (`apps/psychology-api/internal/courses/http.go`):**
  - `GET /public/courses`: فهرست دوره‌های منتشرشده کلینیک.
  - `GET /public/courses/{slug}`: جزئیات دوره به همراه سرفصل جلسات و نشان پیش‌نمایش رایگان.
  - `POST /public/courses/{slug}/checkout`: خرید تک‌دوره با اعتبارسنجی نشست خریدار (`RequireBuyerAuth`).
- **صفحات کاربری و خرید در استورفرانت (`apps/psychology-site`):**
  - `src/pages/courses/index.astro`: کاتالوگ دوره‌های آموزشی با نمایش مدرس، تعداد جلسات و قیمت به تومان طبق D-14.
  - `src/pages/courses/[slug].astro`: صفحه اختصاصی فروش دوره، سرفصل کامل، نشانگر جلسات قفل و رایگان، فرم ورود پیامکی خودکار (در صورت عدم ورود) و دکمه اتصال مستقیم به درگاه پرداخت.
- **تست‌ها و اعتبارسنجی:**
  - آزمون‌های Go در `courses_checkout_test.go` و `checkout_test.go` با پوشش ۱۰۰٪ سناریوهای شروع پرداخت، تایید اتمیک، پرداخت هم‌زمان تکراری و استرداد وجه.
  - گذر کامل دستور `make verify-psychology`.

### ۲۰۲۶-۱۱-۲۱ — اجرای تسک PSY-3.3 (پنل مدیریت دوره و درس، الزامات انتشار و آرشیو، و جداسازی قطعی رسانه دوره از بلاگ)

- **سرویس مدیریت دوره‌ها در بک‌اند (`apps/psychology-api/internal/courses/admin_service.go`):**
  - پیاده‌سازی متدهای کامل چرخه عمر دوره: `CreateCourse`, `UpdateCourse`, `PublishCourse`, `ArchiveCourse`, `DeleteCourse`, `ListCourses`, `GetCourse`.
  - اینواریانت انتشار: الزام وجود حداقل ۱ جلسه آموزشی آماده (`lessons_count >= 1`)، قیمت غیرمنفی (`price_rials >= 0`)، و ثبت زمان انتشار (`published_at = now()`).
  - اینواریانت آرشیو: تغییر وضعیت دوره به `archived` جهت توقف فروش جدید با حفظ کامل دسترسی خریداران پیشین به جلسات موجود.
  - اینواریانت جداسازی رسانه و اعمال سهمیه:
    - بررسی وضعیت تأیید مدیا (`validation_status == 'verified'`).
    - مسدودسازی قطعی اتصال فایل‌های عمومی یا بلاگ (`access_class == 'public'`) به جلسات آموزشی با خطای `ErrDisallowedMediaAccessClass`.
    - مسدودسازی قطعی اتصال فایل‌های متعلق به مستاجر دیگر (`tenant_id mismatch`) با خطای `ErrMediaCrossTenantForbidden`.
- **کنترلرهای HTTP مدیریت دوره و درس (`apps/psychology-api/internal/courses/http.go`):**
  - افزودن مسیرهای احراز هویت شده و مجهز به RBAC و محافظت CSRF:
    - `GET /admin/courses` (با امکان فیلتر بر اساس `status`)
    - `POST /admin/courses`
    - `GET /admin/courses/{id}`
    - `PUT /admin/courses/{id}`
    - `DELETE /admin/courses/{id}`
    - `POST /admin/courses/{id}/publish`
    - `POST /admin/courses/{id}/archive`
    - `GET /admin/courses/{id}/lessons`
    - `POST /admin/courses/{id}/lessons`
    - `GET /admin/courses/{id}/lessons/{lessonId}`
    - `PUT /admin/courses/{id}/lessons/{lessonId}`
    - `DELETE /admin/courses/{id}/lessons/{lessonId}`
    - `GET /admin/courses/{id}/entitlements`
- **پنل ادمین و صفحات کاربری (`apps/psychology-admin/`):**
  - اضافه شدن آیتم ناوبری `🎓 دوره‌های ضبط‌شده` به منوی سایدبار در `AdminLayout.astro`.
  - صفحه فهرست دوره‌ها (`src/pages/courses/index.astro`): کارت‌های آمار دوره‌ها، فیلتر وضعیت، جدول دوره‌ها با تعداد جلسات، قیمت به تومان طبق D-14 و دکمه‌های انتشار و بایگانی.
  - صفحه ثبت دوره جدید (`src/pages/courses/new.astro`): فرم دریافت عنوان، نامک (slug)، مدرس، مبلغ به تومان با تبدیل خودکار به ریال در ارسال، و توضیحات.
  - صفحه ویرایش دوره (`src/pages/courses/[id]/edit.astro`): به‌روزرسانی مشخصات دوره و کلیدهای اقدام سریع انتشار/آرشیو.
  - صفحه مدیریت جلسات دوره (`src/pages/courses/[id]/lessons.astro`): بنر راهنمای امنیت رسانه خصوصی، جدول جلسات مرتب‌شده با بج‌های پیش‌نمایش رایگان / خریداران، و فرم افزودن جلسه با شناسه رسانه `course_private`.
- **آزمون‌های جامع Go (`apps/psychology-api/internal/courses/admin_test.go`):**
  - `TestCourseLifecycle_Draft_Publish_Archive`: ممانعت از انتشار دوره بدون جلسه، انتشار موفق با جلسه، بایگانی موفق و رد انتشار مجدد دوره بایگانی‌شده.
  - `TestLessonMedia_StrictSeparationFromBlogMedia`: رد قطعی رسانه‌های بلاگ (`public`)، رد فایل‌های تاییدنشده (`pending`)، رد فایل‌های مستاجر دیگر (`cross-tenant`) و تایید موفق فایل مجاز `course_private`.
  - `TestAdminCourses_HTTP_Lifecycle_And_RBAC`: صحت احراز هویت، بررسی هدر `X-Tenant-ID` و `Content-Type: application/json`، ایجاد، افزودن درس، انتشار و فیلتر دوره‌ها.
- **صحت‌سنجی کامل:**
  - دستور `make verify-psychology` با موفقیت ۱۰۰٪ تمام تست‌های بک‌اند و فرانت‌اند را پاس کرد.

### ۲۰۲۶-۱۱-۲۱ — اجرای تسک PSY-3.2 (ورود، ثبت‌نام، بازیابی و نشست مستقل خریدار مقید به مستاجر با رمز یک‌بارمصرف)

- **ماژول دوره‌ها و هویت خریداران (`apps/psychology-api/internal/courses/`):**
  - پیاده‌سازی کوئری‌های sqlc در `queries.sql` برای جداول خریدار، نشست‌ها، کدهای OTP، دوره‌ها، دروس و دسترسی‌های خرید (`coursesdb`).
  - اعتبارسنجی و استانداردسازی سخت‌گیرانه شماره موبایل ایرانی (`NormalizePhone`) شامل تبدیل ارقام فارسی و عربی، حذف خط تیره و فاصله، پذیرش پیش‌شماره‌های `+98`، `0098`، `98` و `09` به فرمت استاندارد ۱۱ رقمی `09xxxxxxxxx`.
  - سرویس مدیریت حساب و نشست خریدار (`BuyerService`):
    - ارسال کد یک‌بارمصرف (`SendOTP`) با تولید کد ۶ رقمی امن تصادفی (`crypto/rand`)، ذخیره هش SHA-256 و مهلت انقضای ۱۲ شمارش معکوس (۱۲۰ ثانیه TTL) و لغو خودکار کدهای مصرف‌نشده پیشین همان کاربر.
    - تایید کد (`VerifyOTP`): بررسی عدم مصرف قبلی، مهلت انقضا و سقف ۵ تلاش ناموفق. شمارش و ثبت تلاش‌های غلط و قفل فوری در صورت رسیدن به ۵ تلاش (`ErrOTPExceededAttempts`).
    - جلوگیری قطعی از حملات Replay: ثبت بلادرنگ `consumed_at = now()` پس از اولین مصرف موفق و ممانعت از ارسال مجدد همان کد.
    - ثبت‌نام خودکار خریدار جدید در صورت عدم وجود حساب در همان مستاجر یا ورود به حساب موجود بدون ایجاد شناسه تکراری؛ و حفظ نام کامل.
    - صدور توکن نشست امن ۳۲ بایتی رمزنگاری‌شده (۶۴ کاراکتر هگز) با هش در پایگاه داده و مهلت ۳۰ روزه.
    - خروج (`Logout`) و احراز نشست (`AuthenticateSession`) با بررسی وضعیت فعال حساب (`active`) و ممانعت از ورود حساب‌های مسدود (`blocked`).
  - میدل‌ویر اختصاصی خریدار (`RequireBuyerAuth`): استخراج توکن از هدر `Authorization: Bearer <token>` یا کوکی `buyer_session_token`، اعتبارسنجی مقید به `tenant_id` از هاست دامنه یا context، و تزریق `BuyerAccount` به کانتکست درخواست.
  - کنترلرهای HTTP (`http.go`):
    - `POST /account/auth/otp/send`
    - `POST /account/auth/otp/verify`
    - `POST /account/auth/logout`
    - `GET /account/auth/me`
    - `GET /account/courses` (با هدر اجباری `Cache-Control: no-store, no-cache, must-revalidate, private`)
  - ثبت روت‌ها در روتر اصلی بک‌اند (`apps/psychology-api/cmd/server/main.go`).
- **سوییت تست جامع (`apps/psychology-api/internal/courses/buyer_test.go`):**
  - `TestNormalizePhone`: بررسی ۱۴ سناریوی مختلف شماره‌های مجاز و نامعتبر.
  - `TestSendAndVerifyOTP_RegistrationAndLogin`: ثبت‌نام اولیه، ورود مجدد و ثبات شناسه و نام خریدار.
  - `TestOTP_WrongCodeAndMaxAttemptsLockout`: افزایش گام‌به‌گام شمارنده خطاها و قفل شدن بعد از ۵ بار.
  - `TestOTP_ReplayPrevention`: مسدودسازی قطعی استفاده مجدد از کد مصرف‌شده.
  - `TestOTP_ExpiredCode`: رد کد پس از گذشت مهلت ۱۲۰ ثانیه.
  - `TestBuyerSession_AuthenticateAndExpiryAndLogout`: صحت توکن، خروج و رد دسترسی بعد از لاگ‌اوت.
  - `TestTenantScoping_StrictBuyerIsolation`: ثبت هم‌زمان یک شماره موبایل در کلینیک الف و کلینیک ب، ایجاد دو شناسه کاملاً مجزا، و مسدودسازی کامل استفاده از توکن کلینیک الف در کلینیک ب و بالعکس.
  - `TestPurchaseWithCurrentAccount_And_OTPRecovery`: خرید دوره توسط خریدار جاری، ثبت دسترسی فعال، خروج از سیستم، بازیابی با پیامک مجدد و اثبات حفظ صددرصدی دسترسی‌ها و نمایش دوره در «دوره‌های من».
  - `TestHTTPHandlers`: آزمون جامع اندپوینت‌های HTTP با شبیه‌سازی دامنه، ثبت کوکی، احراز توکن Bearer و پاک‌سازی در خروج.
- **صحت‌سنجی کامل:**
  - دستور `make verify-psychology` با موفقیت ۱۰۰٪ تمام آزمون‌های بک‌اند و فرانت‌اند را پاس کرد.

### ۲۰۲۶-۱۱-۲۱ — اجرای تسک PSY-3.1 (اسکیما، مایگریشن، RLS، قرارداد OpenAPI و کلاینت دوره‌های ضبط‌شده و خریداران)

- **مایگریشن پایگاه داده فاز ۳ (`apps/psychology-api/migrations/000006_create_courses.up.sql` و `down.sql`):**
  - ایجاد جدول `buyer_accounts` برای هویت مستقل خریداران و دانشجویان دوره‌ها مقید به هر مستاجر (`tenant_id, phone` یکتا).
  - ایجاد جدول `buyer_sessions` برای نشست‌های مستقل خریدار بر اساس هش توکن و تاریخ انقضا و ارجاع مرکب به `(tenant_id, buyer_id)`.
  - ایجاد جدول `buyer_otp_codes` جهت مدیریت ورود و ثبت‌نام با پیامک و رمز یک‌بارمصرف با شمارش تلاش‌ها و تاریخ مصرف.
  - ایجاد جدول `courses` شامل عنوان، slug یکتا در هر مستاجر، نام مدرس، قیمت به ریال (عدد صحیح `bigint` بدون ممیز شناور)، شناسه مدیا کاور، وضعیت انتشار (`draft`, `published`, `archived`) و حذف منطقی.
  - ایجاد جدول `course_lessons` شامل ترتیب نمایش، مدت زمان به ثانیه، پرچم پیش‌نمایش رایگان (`is_free_preview`)، ارجاع به دارایی رسانه خصوصی (`media_asset_id`) و کلید خارجی مرکب به جدول دوره‌ها.
  - ایجاد جدول `course_entitlements` برای نگه‌داری دسترسی‌های خرید، ثبت اسنپ‌شات تغییرناپذیر عنوان و قیمت ریالی (`course_title_snapshot`, `price_rials_snapshot`)، و ایندکس یکتای شرطی:
    `CREATE UNIQUE INDEX course_entitlements_active_uq ON course_entitlements (tenant_id, buyer_id, course_id) WHERE (status = 'active');` جهت تضمین قطعی عدم امکان دسترسی تکراری برای یک خریدار در یک دوره فعال.
  - فعال‌سازی و اجبار RLS (`ENABLE ROW LEVEL SECURITY` و `FORCE ROW LEVEL SECURITY`) و اعمال پالیسی ایزولاسیون کامل با `app.tenant_id` به همراه اعطای دسترسی‌های لازم به `app_user` روی تمامی ۶ جدول.
- **تست خودکار جامع مایگریشن و نگهبان اسکیما (`apps/psychology-api/migrations/migration_test.go`):**
  - افزودن سناریوی آزمون `TestCourseTablesRLSAndConstraints` جهت بررسی وجود هر ۶ جدول جدید، فعال و اجباری بودن RLS روی تک‌تک آن‌ها، ساختار کلیدهای خارجی مرکب، و راستی‌آزمایی ایندکس یکتای دسترسی فعال.
- **مستندسازی اسکیما (`docs/psychology/database.md`):**
  - اضافه شدن بخش ۷.۵ شامل معرفی جداول خریدار، دوره‌ها، دروس و دسترسی‌ها و روابط آن‌ها و جدول خلاصه RLS.
- **قرارداد OpenAPI و کلاینت تایپ‌شده (`packages/contracts/psychology/openapi.yaml` و `packages/psychology-api-client`):**
  - افزودن اندپوینت‌های عمومی کاتالوگ و جزییات دوره و پیش‌نمایش ویدیوی رایگان (`/public/courses`).
  - افزودن اندپوینت‌های احراز هویت خریدار با OTP، دوره‌های من و دریافت URL استریم امن درس‌های خریداری شده (`/account/*`).
  - افزودن اندپوینت‌های مدیریتی دوره‌ها، دروس، انتشار/آرشیو و لیست خریداران دوره (`/admin/courses/*`).
  - پیاده‌سازی متدهای متناظر در `@psychology/api-client` و نوشتن تست‌های جامع در `packages/psychology-api-client/test/index.test.mjs`.
- **صحت‌سنجی کامل:**
  - دستور `make verify-psychology` با موفقیت ۱۰۰٪ تمام تست‌های Go و بررسی‌های تایپ‌اسکریپت را پاس کرد.

### ۲۰۲۶-۱۱-۲۱ — اجرای تسک PSY-2.6 (اجرای عملی فرآیند پذیرش، سناریوهای حریم خصوصی، رقابت هم‌زمان و ثبت شواهد و ران‌بوک فاز ۲)

- **پیاده‌سازی سوییت آزمون‌های جامع پذیرش فاز ۲ (`apps/psychology-api/internal/testsupport/acceptance_phase2_test.go`):**
  - شبیه‌سازی دو کلینیک مستقل بر روی دامنه‌های واقعی: کلینیک امید (`omid-psych.ir`) با دو درمانگر مستقل (دکتر مریم علوی و دکتر کامران راد) و مرکز مشاوره ماهان (`mahan-clinic.ir`) با یک درمانگر (دکتر رضا مرادی).
  - `TestAcceptance_PSY_A07_TwoIndependentCalendarsAndDomainIsolation`: تایید تفکیک کامل تقویم‌ها و عدم امکان مشاهده یا دسترسی به درمانگران و سانس‌های کلینیک دیگر از طریق دامنه یا شناسه‌های نامرتبط.
  - `TestAcceptance_PSY_A07_ConcurrentSlotContention_ZeroOversell`: رقابت هم‌زمان ۱۰ متقاضی مجزا با گوروتین‌های موازی برای ۱ سانس آزاد؛ دریافت دقیقاً ۱ نگه‌داشت موقت ۲ ساعته (`201 Created` و وضعیت `reserved`) و رد قطعی ۹ درخواست دیگر با خطای ۴۰۹ (تضمین صددرصدی اصل Zero-Oversell و عدم بروز نوبت‌های تکراری).
  - `TestAcceptance_PSY_A07_LateConfirmationAndAutoExpiry`: تایید عدم امکان ثبت نوبت پس از پایان مهلت ۲ ساعته؛ عملکرد موفق ورکر انقضای خودکار (`ExpirePendingAppointments`)، تغییر وضعیت به `expired` و آزادسازی فوری سانس به `available` برای ثبت‌نام مراجع دیگر.
  - `TestAcceptance_PSY_A07_ReschedulingCollision_KeepsOriginalIntact`: شبیه‌سازی تلاش برای جابه‌جایی نوبت به سانس پر؛ بازگشت خطای ۴۰۹ و راستی‌آزمایی حیاتی عدم تغییر نوبت اولیه و سانس مبدأ (تضمین ثبات و عدم آسیب به داده‌های پیشین).
  - `TestAcceptance_PSY_A07_SuccessfulAtomicRescheduling`: جابه‌جایی اتمیک موفق به سانس خالی با اولویت قفل‌ها (`ORDER BY id ASC FOR UPDATE`)؛ آزادسازی سانس قدیم و اشغال سانس جدید؛ ثبت لاگ ممیزی کامل در `appointment_status_history`.
  - `TestAcceptance_PSY_A07_TherapistRBACIsolation`: اثبات عدم دسترسی درمانگر ۱ به نوبت‌های درمانگر ۲ در همان کلینیک (خطای ۴۰۳ در مشاهده و تغییر)، در کنار دسترسی مجاز منشی و مالک کلینیک.
  - `TestAcceptance_PSY_A07_PIIProtectionAndCacheHeaders`: حذف شماره تماس و نام مراجعین از فهرست‌های عمومی سانس‌ها؛ ممانعت از مشاهده وضعیت بدون هش محرمانه (خطای ۴۰۴)؛ عدم افشای شماره تلفن مراجع حتی در پاسخ پیگیری؛ و اعمال هدر اجباری `Cache-Control: no-store, no-cache, must-revalidate, private`.
  - `TestAcceptance_PSY_A07_GuestCancellationBeforeStartTime`: لغو نوبت توسط مراجع پیش از زمان جلسه با توکن پیگیری و آزادسازی فوری سانس؛ و مسدودسازی لغو پس از آغاز جلسه با خطای ۴۰۰.
- **تکمیل کتابچه عملیات پشتیبانی (`docs/psychology/SUPPORT_RUNBOOK.md`):**
  - تدوین بخش ۷ شامل رویه نگه‌داشت موقت ۲ ساعته و آزادسازی خودکار سانس‌ها، پروتکل لغو و جابه‌جایی اتمیک، و الزامات سفت‌وسخت حریم خصوصی و عدم نشت PII بیماران.
- **مستندسازی شواهد پایلوت (`docs/psychology/PILOT_EVIDENCE.md`):**
  - اضافه شدن بخش‌های ۵ و ۶ شامل ماتریس کامل سناریوی سراسری `PSY-A07` و تایید رسمی پایان فاز ۲.
- **صحت‌سنجی کامل:**
  - اجرای موفق تمامی ۸ سناریوی آزمون پذیرش بدون خطا (`PASS`).
  - دستور `make verify-psychology` با موفقیت ۱۰۰٪ اجرا شد.

### ۲۰۲۶-۱۱-۲۱ — اجرای تسک PSY-2.5 (صفحات عمومی و پنل نوبت و وضعیت، تقویم زمان تهران، حذف PII از وب عمومی، کش و لاگ)

- **صفحه عمومی درخواست و رزرو نوبت (`apps/psychology-site/src/pages/booking/index.astro`):**
  - نمایش کارت‌های معرفی درمانگران فعال کلینیک همراه با تخصص، بیوگرافی و طول هر جلسه مشاوره.
  - دریافت و گروه‌بندی پویای سانس‌های آزاد آینده به وقت رسمی تهران (`Asia/Tehran`) با روز هفته و تاریخ شمسی.
  - مودال فرم ثبت نوبت مهمان با فیلدهای نام، شماره موبایل و توضیحات مراجعه با ارسال به `POST /public/appointments`.
  - هدایت خودکار کاربر پس از رزرو موفق به صفحه پیگیری با شناسه و کلید اختصاصی پیگیری (`/appointments/${id}?hash=${guest_lookup_hash}`).
  - مدیریت خطاهای رقابت بر سر سانس (کد ۴۰۹) با هشدار تکمیل ظرفیت یا رزرو موقت هم‌زمان، و خطای سقف نرخ درخواست (کد ۴۲۹).
  - حذف کامل داده‌های شخصی: در فهرست سانس‌های عمومی هیچ‌گونه نام یا شماره تماسی از مراجعین دیگر وجود ندارد.
- **صفحه عمومی پیگیری اختصاصی نوبت (`apps/psychology-site/src/pages/appointments/[id].astro`):**
  - تنظیم هدر پاسخ `Cache-Control: no-store, no-cache, must-revalidate, private` جهت جلوگیری قطعی از ذخیره‌سازی داده‌های خصوصی مراجع در کش‌های عمومی و مرورگر.
  - احراز دسترسی با توکن محرمانه پیگیری مراجع (`X-Guest-Lookup-Hash` یا پارامتر `hash` / `lookup_hash`)؛ ممانعت از نمایش اطلاعات به افراد متفرقه و موتورهای جستجو (`<meta name="robots" content="noindex, nofollow" />`).
  - نمایش بج‌های رنگی وضعیت نوبت (`pending`, `confirmed`, `cancelled`, `expired`, `completed`) همراه با توضیحات راهنما و مهلت ۲ ساعته تأیید.
  - نمایش زمان شروع و پایان جلسه، روز و تاریخ شمسی با منطقه زمانی `Asia/Tehran`.
  - دکمه و مودال لغو نوبت توسط مراجع پیش از فرا رسیدن جلسه با ارسال درخواست به `POST /public/appointments/${id}/cancel`.
  - عدم بازگردانی یا نمایش شماره تماس مراجع در صفحه عمومی جهت تضمین کامل حریم خصوصی.
- **به‌روزرسانی نقشه سایت (`apps/psychology-site/src/pages/sitemap.xml.ts`):**
  - ثبت آدرس `/booking` در لیست URLهای استاتیک سایت جهت دسترسی عمومی مراجعین.
  - تضمین عدم وجود مسیرهای اختصاصی نوبت (`/appointments/*`) در sitemap.
- **داشبورد مدیریت نوبت‌ها در پنل پرسنل (`apps/psychology-admin/src/pages/appointments/index.astro`):**
  - طراحی مبتنی بر `AdminLayout` و اتصال به منوی `appointments`.
  - کارت‌های آماری نوبت‌های در انتظار بررسی، نوبت‌های قطعی پیش‌رو و کل نوبت‌ها.
  - تب‌های فیلتر وضعیت نوبت‌ها (`all`, `pending`, `confirmed`, `completed`, `cancelled`).
  - جدول نوبت‌ها شامل نام مراجع، شماره تلفن همراه فعال برای پرسنل احراز هویت شده، نام درمانگر، تاریخ و ساعت تهران و بج وضعیت.
  - دکمه تأیید مستقیم نوبت (`POST /admin/appointments/{id}/confirm`).
  - مودال لغو نوبت با ثبت اجباری دلیل لغو (`POST /admin/appointments/{id}/cancel`).
  - مودال جابه‌جایی اتمیک به سانس آزاد دیگر درمانگر (`POST /admin/appointments/{id}/reschedule`) با نمایش خطای ۴۰۹ در صورت پر بودن سانس مقصد بدون تغییر در نوبت مبدأ.
  - مودال نمایش تاریخچه کامل تغییرات وضعیت نوبت و ردپای ممیزی با زمان محلی تهران (`GET /admin/appointments/{id}/history`).
- **تنظیمات کنترل کش در بک‌اند API (`apps/psychology-api/internal/booking/http.go`):**
  - اعمال `Cache-Control: no-store, no-cache, must-revalidate, private` در پاسخ‌های وضعیت نوبت عمومی.
  - پشتیبانی ترکیبی از هدر `X-Guest-Lookup-Hash` و کوئری پارامتر `hash` و `lookup_hash`.
- **صحت‌سنجی کامل:**
  - دستور `make verify-psychology` با موفقیت ۱۰۰٪ اجرا شد (تمام آزمون‌های Go و تایپ‌چک و بیلد Astro با موفقیت پاس شدند).

### ۲۰۲۶-۱۱-۲۱ — اجرای تسک PSY-2.4 (تأیید، لغو، جابه‌جایی اتمیک با قفل‌های مرتب، و اعمال RBAC درمانگر و پذیرش)

- توسعه کوئری‌های SQL و بهینه‌سازی دسترسی‌ها (`apps/psychology-api/internal/booking/queries.sql` و `bookingdb`):
  - کوئری استعلام درمانگر بر اساس `user_id` برای احراز تقویم متصل به درمانگر لاگین‌شده.
  - کوئری‌های جزئیات نوبت (`GetAppointmentDetail`) و لیست نوبت‌های اداری (`ListAdminAppointments`) همراه با پیوند به نام درمانگر و زمان‌های سانس.
  - کوئری به‌روزرسانی سانس نوبت (`UpdateAppointmentSlot`) برای عملیات جابه‌جایی.
- پیاده‌سازی منطق بیزینس مدیریت نوبت‌ها در لایه سرویس (`apps/psychology-api/internal/booking/service.go`):
  - **تأیید نوبت (`ConfirmAdminAppointment`):** بررسی وضعیت `pending` و عدم انقضا؛ قفل سانس و انتقال وضعیت آن از `reserved` به `booked`؛ انتقال نوبت به `confirmed` و ثبت زمان `confirmed_at`؛ ثبت رویداد ممیزی با عامل `staff`.
  - **لغو از پنل (`CancelAdminAppointment`):** بررسی عدم شروع جلسه؛ انتقال نوبت به `cancelled` و ثبت دلیل؛ آزادسازی سانس به `available`؛ ثبت رویداد ممیزی با عامل `staff`.
  - **جابه‌جایی اتمیک نوبت (`RescheduleAdminAppointment`):** اعمال پروتکل ترتیب ثابت قفل‌ها (قفل سانس قبلی و سانس جدید به ترتیب لکسیکال شناسه‌های UUID با `ORDER BY id ASC FOR UPDATE` و Mutexهای ترتیبی) جهت جلوگیری قطعی از Deadlock؛ بررسی خالی بودن سانس مقصد (`status = 'available'`) و شروع آن در آینده؛ در صورت پر بودن سانس مقصد، عملیات فوراً با خطای ۴۰۹ متوقف شده و نوبت قبلی و سانس مبدأ کاملاً دست‌نخورده باقی می‌مانند؛ در صورت موفقیت، سانس جدید `booked` شده، سانس قدیم به `available` بازگشته، شناسه سانس نوبت به‌روز شده و تاریخچه با متادیتای `previous_slot_id` و `new_slot_id` ثبت می‌شود.
  - **کنترل دسترسی نقشی (RBAC):** نقش‌های `owner` و `receptionist` به تمامی نوبت‌های کلینیک دسترسی دارند؛ نقش `therapist` صرفاً می‌تواند نوبت‌های تقویم متصل به حساب خود را مشاهده، تأیید، لغو یا جابه‌جا کند و در صورت تلاش برای دسترسی به نوبت درمانگر دیگر با خطای ۴۰۳ (`ErrUnauthorizedTherapist`) مواجه می‌شود.
- پیاده‌سازی هندلرهای HTTP اداری با محافظت CSRF و احراز هویت (`apps/psychology-api/internal/booking/http.go`):
  - `GET /admin/appointments`: فهرست نوبت‌ها با فیلتر وضعیت و درمانگر (اعمال خودکار فیلتر درمانگر لاگین‌شده).
  - `GET /admin/appointments/{id}`: مشاهده جزئیات نوبت همراه با نام و تلفن مراجع.
  - `POST /admin/appointments/{id}/confirm`: تأیید نوبت پس از تماس هماهنگی.
  - `POST /admin/appointments/{id}/cancel`: لغو نوبت با ثبت دلیل.
  - `POST /admin/appointments/{id}/reschedule`: جابه‌جایی اتمیک نوبت به سانس جدید.
  - `GET /admin/appointments/{id}/history`: مشاهده گزارش کامل ممیزی تغییرات وضعیت.
- آزمون‌های خودکار جامع در `apps/psychology-api/internal/booking/booking_test.go`:
  - `TestAdminAppointment_Confirmation`: تأیید موفق نوبت معلق، تغییر سانس به `booked`، ثبت تاریخچه، رد تأیید نوبت منقضی‌شده و رد تأیید مجدد.
  - `TestAdminAppointment_Cancellation`: لغو موفق نوبت، بازگشت سانس به `available` و رد لغو پس از آغاز جلسه.
  - `TestAdminAppointment_Rescheduling_AtomicLockAndCollision`: شبیه‌سازی جابه‌جایی به سانس پر و اثبات دست‌نخورده ماندن نوبت و سانس مبدأ؛ و جابه‌جایی موفق به سانس آزاد و بررسی آزادسازی سانس قدیم و اشغال سانس جدید همراه با متادیتای تاریخچه.
  - `TestAdminAppointment_TherapistAccessControl`: آزمون دو درمانگر در یک کلینیک و اثبات قطعی عدم دسترسی درمانگر ۱ به نوبت‌های درمانگر ۲ (خطای ۴۰۳) و دسترسی کامل منشی به هر دو تقویم.
  - `TestAdminAppointment_HTTPEndpoints`: آزمون کامل اندپوینت‌های HTTP اداری با توکن نشست، هدر مستاجر و محافظت CSRF.
  - تمامی ۱۹ آزمون بسته بوکینگ و بررسی کامل `make verify-psychology` با موفقیت ۱۰۰٪ پاس شدند.

### ۲۰۲۶-۱۱-۲۱ — اجرای تسک PSY-2.3 (رزرو مهمان، نگه‌داشت موقت ۲ ساعته، محدودیت نرخ و پیشگیری از oversell)

- پیاده‌سازی مکانیزم محدودیت نرخ درخواست‌های عمومی (`apps/psychology-api/internal/booking/ratelimit.go`):
  - ریت‌لیمیتر کشویی Thread-safe با پنجره زمانی یک دقیقه‌ای و سقف ۱۰ درخواست بر مبنای IP کلاینت و مستاجر (`tenant:ip`).
  - پاسخ با کد `429 Too Many Requests` و هدر `Retry-After: 60`.
- پیاده‌سازی لایه سرویس رزرو مهمان و نگه‌داشت موقت (`apps/psychology-api/internal/booking/service.go`):
  - نرمال‌سازی شماره تلفن همراه ایران (`NormalizePhone`) با پشتیبانی از ارقام فارسی/عربی، پیش‌شماره‌های `+98`, `0098`, `98` و `09` به فرمت استاندارد ۱۱ رقمی `09xxxxxxxxx`.
  - تولید توکن محرمانه پیگیری مراجع (`guest_lookup_hash`) با ۶۴ کاراکتر هگزادیسمال رمزنگاری‌شده و غیرقابل حدس.
  - محاسبه دقیق مهلت نگه‌داشت موقت ۲ ساعته: `expires_at = min(now + 2h, slot.starts_at)`؛ در صورتی که سانس کمتر از ۲ ساعت آینده شروع شود، سقف انقضا دقیقاً بر زمان آغاز سانس منطبق می‌شود. رد سانس‌های در گذشته با خطای ۴۰۰ (`ErrSlotInThePast`).
  - نگه‌داشت اتمیک سانس با قفل سطر دیتابیس (`FOR UPDATE`) و قفل متقابل درون حافظه‌ای (`slotLocks`)؛ تغییر وضعیت سانس از `available` به `reserved` و ثبت نوبت با وضعیت `pending`.
  - ثبت ممیزی ایجاد نوبت در `appointment_status_history` با عمل `created` و عامل `guest`.
  - تضمین قطعی حریم خصوصی: فیلد شماره تماس (`guest_phone`) به هیچ وجه در پاسخ‌های عمومی یا لیست سانس‌ها منعکس نمی‌شود.
  - امکان بررسی وضعیت نوبت با توکن پیگیری (`GetPublicAppointmentStatus`) و تشخیص انقضا در زمان خواندن: در صورت گذشت ۲ ساعت، نوبت به وضعیت `expired` منتقل شده و سانس متصل به `available` بازمی‌گردد.
  - امکان لغو نوبت توسط مهمان (`CancelPublicAppointment`) با ارائه توکن پیگیری معتبر قبل از آغاز جلسه، آزادسازی سانس به `available` و ثبت در تاریخچه با عامل `guest`.
  - ممانعت از لغو جلسه‌ای که زمان شروع آن سپری شده است (`ErrCannotCancelPastSlot`).
  - متد دوره‌ای `ExpirePendingAppointments` برای همگام‌سازی و آزادسازی سانس‌های منقضی‌شده توسط Worker.
- پیاده‌سازی هندلرهای HTTP اندپوینت‌های عمومی (`apps/psychology-api/internal/booking/http.go`):
  - `POST /public/appointments`: ثبت درخواست رزرو با اعتبارسنجی ورودی، استخراج IP و پاسخ ۲0۱ Created.
  - `GET /public/appointments/{id}`: دریافت وضعیت نوبت منوط به ارسال هدر `X-Guest-Lookup-Hash`.
  - `POST /public/appointments/{id}/cancel`: لغو نوبت توسط مهمان با هدر `X-Guest-Lookup-Hash`.
- آزمون‌های خودکار جامع در `apps/psychology-api/internal/booking/booking_test.go`:
  - `TestPublicAppointment_CreationAndPrivacy`: تست اعتبارسنجی ورودی، نرمال‌سازی ارقام فارسی تلفن، محاسبه ۲ ساعته TTL، تغییر سانس به `reserved`، ثبت ممیزی و عدم نشت `guest_phone`.
  - `TestPublicAppointment_TTL_CappedBySlotStartTime`: آزمون سانس‌های نزدیک (۴۵ دقیقه آینده) و محدود شدن انقضا به آغاز سانس، و رد سانس‌های گذشته.
  - `TestPublicAppointment_ConcurrentSlotHold_ZeroOversell`: شبیه‌سازی رقابت ۱۰ گوروتین هم‌زمان برای گرفتن یک سانس واحد؛ موفقیت قطعی تنها ۱ درخواست و رد ۹ درخواست دیگر با خطای ۴۰۹ (Zero Oversell).
  - `TestPublicAppointment_StatusCheck_And_AutoExpiry`: رد استعلام با هش نامعتبر یا بدون هش، و آزادسازی خودکار سانس پس از گذشت ۲ ساعت.
  - `TestPublicAppointment_Cancellation`: لغو موفق قبل از جلسه با توکن پیگیری، آزادسازی سانس، و ممانعت از لغو پس از شروع جلسه.
  - `TestPublicAppointment_RateLimiting`: اعمال محدودیت نرخ و مسدودسازی درخواست‌های اضافه با کد ۴۲۹.
  - `TestPublicAppointment_CrossTenantIsolation`: تضمین کامل عدم دسترسی مستاجر B به نوبت‌ها یا امکان لغو آنها در مستاجر A.
  - `TestPublicAppointment_HTTPEndpoints`: آزمون کامل چرخه حیات اندپوینت‌های HTTP با روتر چی و استعلام دامنه‌ای.
  - اجرای کامل `make verify-psychology` با نتیجه ۱۰۰٪ سبز.

### ۲۰۲۶-۱۱-۲۱ — اجرای تسک PSY-2.2 (ساخت تقویم درمانگر، ساعات کاری، استثناها، تولید سانس‌های غیرهم‌پوشان و هندلرهای HTTP)

- پیاده‌سازی کوئری‌های `sqlc` و ساخت بسته `bookingdb` (`apps/psychology-api/internal/booking/queries.sql` و `sqlc.yaml`):
  - عملیات CRUD پروفایل درمانگران با اسلاگ یکتا و حذف منطقی.
  - عملیات ذخیره و بازیابی ساعات کاری هفتگی درمانگر (`therapist_working_hours`).
  - عملیات ثبت و حذف استثناهای تقویم و تعطیلات کامل یا ساعتی (`therapist_schedule_exceptions`).
  - عملیات ایجاد، استعلام و پاک‌سازی سانس‌های آزاد آینده با فیلتر وضعیت و زمان.
- پیاده‌سازی لایه سرویس تقویم و محاسبات زمانی (`apps/psychology-api/internal/booking/service.go`):
  - نگاشت استاندارد روزهای هفته ایران (شنبه = ۰ تا جمعه = ۶).
  - محاسبات تقویم در منطقه زمانی تهران (`Asia/Tehran`) و ذخیره زمان به صورت استاندارد UTC.
  - تولید سانس‌های غیرهم‌پوشان تا سقف ۳۰ روز بر مبنای `session_duration_minutes`، بازه‌های فعال کاری و کسر استثناهای روزانه و ساعتی.
  - اعمال اصل عدم آسیب به رزروهای قبلی: هنگام تغییر یا حذف ساعات کاری، صرفاً سانس‌های با وضعیت `available` در آینده پاک‌سازی می‌شوند و سانس‌های `booked` یا `reserved` مراجعین دست‌نخورده و قابل پیگیری باقی می‌مانند.
- پیاده‌سازی هندلرهای HTTP و مسیریابی (`apps/psychology-api/internal/booking/http.go`):
  - اندپوینت‌های عمومی: دریافت درمانگران فعال (`GET /public/therapists`)، پروفایل درمانگر (`GET /public/therapists/{id}`) و فهرست سانس‌های آزاد تا ۳۰ روز آینده (`GET /public/therapists/{id}/slots`) با تضمین کامل عدم نشت PII.
  - اندپوینت‌های پنل مدیریت: مدیریت درمانگر (مالک)، ساعات کاری، استثناها، فهرست و تولید سانس‌ها با اعتبارسنجی احراز هویت، عضویت مستاجر و CSRF.
  - اتصال به روتر اصلی سرور در `apps/psychology-api/cmd/server/main.go`.
- آزمون‌های جامع در `apps/psychology-api/internal/booking/booking_test.go`:
  - `TestTherapistProfileLifecycle`: ایجاد، جستجو با اسلاگ و ID، ویرایش و حذف منطقی.
  - `TestWorkingHoursAndSlotGeneration`: تولید سانس‌های غیرهم‌پوشان ۴۵ دقیقه‌ای و بررسی صحت مدت زمان و فواصل زمانی.
  - `TestScheduleExceptions_FullAndPartial`: اعمال استثناهای روزانه کامل و ساعتی و عدم ایجاد سانس در ساعات مسدودشده.
  - `TestWorkingHoursChange_KeepsBookedSlotsIntact`: شبیه‌سازی تغییر ساعات کاری هفتگی پس از ثبت یک نوبت قطعی و اثبات دست‌نخورده ماندن سوابق نوبت قبلی.
  - `TestCrossTenantIsolation`: تضمین عدم امکان مشاهده یا دستکاری تقویم و سانس‌های کلینیک A توسط کلینیک B.
  - `TestHTTPEndpointsAndRoleEnforcement`: بررسی کنترل نقش‌ها و رد درخواست‌های غیرمجاز.
  - تمامی تست‌ها با موفقیت کامل پاس شدند.

### ۲۰۲۶-۱۱-۲۱ — اجرای تسک PSY-2.1 (طراحی و تکمیل Schema، Migration، OpenAPI و آزمون‌های RLS نوبت‌دهی)

- ایجاد فایل‌های مهاجرت پایگاه داده (`apps/psychology-api/migrations/000005_create_appointments.up.sql` و `000005_create_appointments.down.sql`):
  - جدول `therapists`: اطلاعات درمانگران، پیوند اختیاری به هویت پنل (`panel_identities`)، تصویر نمایه، اسلاگ یکتا در هر مستاجر، مدت جلسه و حذف منطقی.
  - جدول `therapist_working_hours`: ساعات کاری هفتگی برای روزهای ۰ تا ۶ با بررسی `start_time < end_time` و کلید خارجی ترکیبی.
  - جدول `therapist_schedule_exceptions`: روزهای تعطیل، مرخصی یا ساعات کاری استثنایی با کلید خارجی ترکیبی.
  - جدول `appointment_slots`: سانس‌های نوبت‌دهی ذخیره‌شده به زمان UTC با قید عدم هم‌پوشانی `(tenant_id, therapist_id, starts_at)`.
  - جدول `appointments`: نوبت‌های رزروشده با نام، تلفن و هش امن پیگیری مراجع (`guest_lookup_hash`)، وضعیت‌های `pending`, `confirmed`, `cancelled`, `expired`, `completed`، مهلت انقضای ۲ ساعته و قید یکتایی `appointments_active_slot_uq` برای جلوگیری از ثبت بیش از یک نوبت فعال در هر سانس.
  - جدول `appointment_status_history`: ممیزی تغییرات وضعیت نوبت همراه با ثبت عامل (`guest`, `staff`, `system`)، دلیل و فیلد متادیتا.
  - امنیت کامل RLS: اعمال دستورات `ENABLE ROW LEVEL SECURITY` و `FORCE ROW LEVEL SECURITY` و policy تفکیک مستاجر بر مبنای `current_setting('app.tenant_id', true)` بر تمامی جداول ۶گانه و اعطای مجوزهای لازم به نقش `app_user`.
- آزمون‌های خودکار مهاجرت و اعتبارسنجی RLS (`apps/psychology-api/migrations/migration_test.go`):
  - اضافه شدن تست `TestAppointmentTablesRLSAndConstraints` جهت بررسی وجود جداول، فعال بودن RLS، کلیدهای خارجی ترکیبی و قید یکتایی سانس‌های فعال.
  - اجرای موفق آزمون‌های یکپارچگی `TestMigrationFilesPairingAndIntegrity` و آزمون ساختار جداول نوبت‌دهی.
- به‌روزرسانی مستندات پایگاه داده (`docs/psychology/database.md`):
  - مستندسازی ساختار کامل جداول ۶گانه فاز ۲ در بخش ۷.
  - ثبت ماتریس نقش و مجوزهای نوبت‌دهی (تفکیک اختیارات `owner`, `content_manager`, `receptionist`, `therapist`).
  - ثبت چرخه تغییر وضعیت سانس‌ها و نوبت‌ها، سیاست مهلت ۲ ساعته نگه‌داشت موقت (`pending` TTL) و پیش‌تولید ۳۰ روزه سانس‌ها.
  - تدوین پروتکل دقیق ترتیب قفل‌ها (`Lock Ordering`) در عملیات رزرو، تأیید، لغو و جابه‌جایی اتمیک به ترتیب شناسه جهت پیشگیری قطعی از Deadlock.
- گسترش قرارداد سراسری API (`packages/contracts/psychology/openapi.yaml`):
  - افزودن اندپوینت‌های عمومی: دریافت لیست درمانگران، پروفایل، سانس‌های آزاد آینده بدون نشت PII، ثبت درخواست نوبت، پیگیری وضعیت با هش و لغو نوبت توسط مهمان.
  - افزودن اندپوینت‌های پنل مدیریت: مدیریت درمانگران، ساعات کاری، استثناها، تولید سانس‌ها، فهرست نوبت‌ها، تأیید، لغو، جابه‌جایی و تاریخچه ممیزی.
  - تعریف اسکیمای مدل‌های داده و درخواست/پاسخ‌های مربوطه در `components.schemas`.
- به‌روزرسانی کلاینت تایپ‌شده TypeScript (`packages/psychology-api-client`):
  - اضافه شدن تمامی تایپ‌ها و متدهای نوبت‌دهی و درمانگران.
  - آزمون‌های اندپوینت‌های نوبت‌دهی در `test/index.test.mjs` (۱۱ تست کاملاً سبز).
  - موفقیت کامل فرآیند build و typecheck تمامی بسته‌های Astro و کلاینت.

### ۲۰۲۶-۱۱-۲۱ — اجرای تسک PSY-1.13 (اجرای سناریوهای پایلوت، ثبت رخدادها، راهنمای پشتیبانی و شواهد پذیرش فاز ۱)

- پیاده‌سازی آزمون‌های خودکار پذیرش و سناریوهای پایلوت (`apps/psychology-api/internal/testsupport/acceptance_phase1_test.go`):
  - `TestAcceptance_PSY_A02_WorkshopPublishAndRegistration`: انتشار کارگاه رایگان در کلینیک امید (`omid-psych.ir`) و ثبت‌نام مهمان با تایید آنی بدون نیاز به پرداخت؛ اعتبارسنجی کاهش اتمیک ظرفیت و عدم امکان ارسال قیمت از سمت کلاینت.
  - `TestAcceptance_PSY_A03_ConcurrentSeatContentionAndZeroOversell`: اجرای رقابت هم‌زمان با ۵ گوروتین برای ۱ صندلی باقیمانده کارگاه پولی در کلینیک ماهان (`mahan-clinic.ir`)؛ موفقیت قطعی تنها ۱ رزرو و رد ۴ درخواست دیگر با خطای ۴۰۹ (تضمین Zero-Oversell و عدم منفی‌شدن موجودی).
  - `TestAcceptance_PSY_A04_PaymentIdempotencyAndRefundRouting`: فرآیند کامل صدور اسنپ‌شات ریالی، پرداخت موفق درگاه، اعتبارسنجی سروری Verify و تایید صندلی؛ تضمین رفتار Idempotent کال‌بک‌های تکراری بدون ایجاد اثر مالی یا صندلی مضاعف؛ شبیه‌سازی پرداخت پس از انقضای مهلت نگه‌داشت صندلی و هدایت خودکار سفارش به `refund_required`؛ و تسویه دستی بازپرداخت با ثبت شناسه پیگیری توسط مالک.
  - `TestAcceptance_PSY_A05_BlogDraftIsolationAndPerHostSitemap`: ایجاد پیش‌نویس مقاله و تایید عدم افشای آن در اندپوینت‌های عمومی، فهرست‌ها و نقشه سایت؛ انتشار مطلب و در دسترس بودن آن صرفاً روی Host همان مستاجر؛ و حذف آنی از نقشه سایت و دسترس عموم پس از لغو انتشار.
  - `TestAcceptance_PSY_A06_MediaStorageValidationAndQuota`: فرآیند دو مرحله‌ای آپلود امن در استوریج پارس‌پک (صدور Presigned Staging URL با انقضای ۱۵ دقیقه و سپس Finalize)؛ اعتبارسنجی سخت‌گیرانه نوع فایل (رد فایل‌های اجرایی خطرناک با خطای ۴۰۰)؛ ثبت شیء نهایی با کلید تغییرناپذیر؛ و ایزولاسیون کامل فایل‌ها در برابر دسترسی سایر مستاجران.
  - `TestAcceptance_PSY_A10_PilotFullScenariosAndPIIProtection`: بررسی نهایی حریم خصوصی و عدم نشت اطلاعات شخصی (PII)؛ بررسی فیلتر بودن شماره تلفن و نام مهمان در لیست‌های عمومی کارگاه‌ها و نقشه سایت، و محدود بودن مشاهده اطلاعات رزرو به توکن محرمانه آغازکننده.
- تدوین کتابچه جامع عملیات پشتیبانی و ثبت رخداد (`docs/psychology/SUPPORT_RUNBOOK.md`):
  - اصول لاگ‌برداری ساختاریافته بدون PII (عدم ثبت شماره تماس، نام بیمار، توکن خام یا Presigned URL).
  - رویه رهگیری پرداخت‌های بلاتکلیف و عملکرد ورکر دوره‌ای تطبیق (`ReconcileStalePayments`).
  - دستورالعمل گام‌به‌گام مالک و پشتیبان برای بازپرداخت‌های بانکی `refund_required` و ثبت شناسه مرجع.
  - دستورالعمل‌های کنترل سهمیه ۱۰ گیگابایت رسانه، پاک‌سازی استیجینگ و مانور بازیابی دیتابیس با اعتبارسنجی RLS.
- تدوین سند رسمی شواهد پذیرش پایلوت (`docs/psychology/PILOT_EVIDENCE.md`):
  - مستندسازی کامل ماتریس پذیرش سناریوهای PSY-A01 تا PSY-A06 و PSY-A10.
  - شواهد تفکیک‌شده دو پایلوت واقعی (کلینیک امید و مرکز مشاوره ماهان) و تایید آمادگی استقرار جهت پایان فاز ۱.

### ۲۰۲۶-۱۱-۲۱ — اجرای تسک PSY-1.12 (زیرساخت Compose، پروکسی Caddy، کش تغییرناپذیر و no-store، اسکریپت‌های بازیابی با RLS و پاک‌سازی حریم خصوصی ۹۰ روزه)

- معماری کانتینری و پیکربندی Compose (`infrastructure/psychology/docker-compose.yml`):
  - راه‌اندازی سرویس‌های مستقل `postgres` (نسخه ۱۶)، `api` (بک‌اند سبک Alpine Go)، `admin` (پنل مدیریت Astro Node 22 SSR)، `site` (وب‌سایت عمومی Astro Node 22 SSR)، `caddy` (لبه TLS و Reverse Proxy) و سرویس زمان‌بندی‌شده `backup`.
  - تعریف شبکه‌های مجزای داخلی (`psychology_internal`) و بیرونی (`psychology_edge`)، حجم‌های پایدار برای داده‌های دیتابیس، گواهی‌های Caddy و بکاپ‌ها.
  - جداسازی کامل متغیرهای محیطی در `infrastructure/psychology/.env.example`؛ تفکیک کاربر ادمین میگریشن از کاربر محدود اپلیکیشن با سلب امتیاز دور زدن RLS (`NOBYPASSRLS`)، تفکیک کلیدها و دسترسی‌های باکت S3 پارس‌پک، درگاه پرداخت زرین‌پال، سکرت‌های نشست و توکن وب‌هوک.
- تنظیمات لبه Caddy (`infrastructure/psychology/Caddyfile`):
  - حفظ نام کامل میزبان در هدایت پروکسی با `header_up Host {host}` برای تضمین ایزولاسیون کامل چندمستاجری بر مبنای دامنه.
  - سیاست کش سفت‌وسخت:
    - دارایی‌های ثابت و فایل‌های بیلد کلاینت (`/_astro/*`, `/assets/*`, `/favicon.ico`, `/robots.txt`): `Cache-Control "public, max-age=31536000, immutable"`.
    - مسیرهای خصوصی، جریان استریم و پردازش‌های داینامیک (`/admin*`, `/public/media/*/stream*`, `/callbacks*`, رزروها و لاگین): اعمال هدر اجباری `Cache-Control "no-store, no-cache, must-revalidate, private"` و `Pragma "no-cache"` برای ممانعت قطعی از هرگونه کش در CDN آروان یا پروکسی‌های واسط.
  - هدرهای امنیتی دفاعی: `X-Content-Type-Options nosniff`, `X-Frame-Options DENY`, `Referrer-Policy strict-origin-when-cross-origin`, و حذف هدر افشاکننده `Server`.
- امنیت پایگاه داده و اسکریپت‌های پشتیبان‌گیری/بازیابی:
  - تنظیم نقش `app_user` با `NOSUPERUSER NOBYPASSRLS` در `infrastructure/psychology/initdb/01-roles.sql`.
  - اسکریپت بکاپ امن `infrastructure/psychology/scripts/backup.sh`: تهیه پشتیبان فشرده با فرمت سفارشی PostgreSQL (`pg_dump -Fc`)، ساخت نام فایل‌های تاریخ‌دار، نگهداری دوره‌ای ۳۰ روزه و پاک‌سازی خودکار نسخه‌های قدیمی‌تر.
  - اسکریپت بازیابی بلایا `infrastructure/psychology/scripts/restore.sh`: پاک‌سازی و بازیابی ساختار با `pg_restore --clean`، و آزمون خودکار پس از بازیابی جهت اطمینان از فعال بودن و اجباری ماندن کامل `ROW LEVEL SECURITY` و سیاست‌های امنیتی مستاجران (`FORCE ROW LEVEL SECURITY`).
- پاک‌سازی دوره‌ای و محرمانگی داده‌ها (`apps/psychology-api/internal/platform/retention`):
  - ماژول `cleaner.go`: اسکن دوره‌ای ثبت‌نام‌های قدیمی‌تر از ۹۰ روز پس از اتمام کارگاه یا لغو آن، و جایگزینی داده‌های حساس مهمان با مقادیر بی‌نام (`guest_name = 'مهمان'`, `guest_phone = ''`, `guest_lookup_hash = ''`).
  - حفظ یکپارچگی مالی و حسابرسی: برای کارگاه‌های پولی، حذف اطلاعات تا زمان حل‌وفصل و بسته‌شدن اختلافات مالی (پایان وضعیت‌های در جریان `refund_required` یا `pending`) به تعویق می‌افتد؛ سوابق مالی سفارشات در `purchases`، اسنپ‌شات مبالغ ریالی، شناسه درگاه و کدهای پیگیری `refunds` به‌صورت کامل و دست‌نخورده حفظ می‌شوند.
  - آزمون‌های جامع در `cleaner_test.go` جهت بررسی شرایط ۹۰ روز، عدم پاک‌سازی پیش از موعد، عدم پاک‌سازی ثبت‌نام‌های دارای مناقشه مالی فعال، و آزمون پاک‌سازی ایمن پس از تسویه.
- آزمون‌های راستی‌آزمایی استقرار (`apps/psychology-api/internal/platform/deployment/deployment_test.go`):
  - بررسی صحت ساختار سرویس‌ها، شبکه‌ها و ولوم‌ها در Docker Compose.
  - بررسی هدرهای کش تغییرناپذیر، `no-store` و امنیت هدرهای Caddy.
  - اعتبارسنجی جداسازی سکرت‌ها و متغیرهای پیکربندی در `.env.example`.
  - اعتبارسنجی اسکریپت‌های بکاپ و ری‌استور و تطابق با دستورالعمل‌های RLS.
  تمامی آزمون‌ها با موفقیت کامل پاس شدند.

### ۲۰۲۶-۱۱-۲۱ — اجرای تسک PSY-1.11 (ویرایشگر امن بلاگ، صفحات فهرست و مطلب، پخش استریم صوت و فیلم و sitemap per Host)

- پکیج کلاینت `@psychology/api-client`:
  - متدهای تایپ‌شده کامل برای بلاگ پنل (`listAdminBlogPosts`, `getAdminBlogPost`, `createAdminBlogPost`, `updateAdminBlogPost`, `publishAdminBlogPost`, `unpublishAdminBlogPost`, `deleteAdminBlogPost`)، مدیریت رسانه‌ها (`requestMediaUploadUrl`, `finalizeMediaUpload`, `listAdminMedia`)، و بلاگ و استریم عمومی (`listPublicBlogPosts`, `getPublicBlogPost`, `getPublicMediaStreamUrl`) پیاده‌سازی شدند.
- پنل مدیریت کلینیک (`apps/psychology-admin`):
  - `src/pages/blog/index.astro`: فهرست مقالات با فیلتر وضعیت (همه، منتشرشده، پیش‌نویس)، تاریخ شمسی، برچسب وضعیت، دسترسی‌های ویرایش و دکمه‌های انتشار، لغو انتشار و حذف با تأیید کاربر.
  - `src/pages/blog/new.astro`: ویرایشگر امن ساخت‌یافته؛ تولید خودکار نامک معتبر از عنوان، توضیحات سئو، ایجاد و جابه‌جایی انواع بلوک‌های محتوایی (پاراگراف متن، تیترهای سطوح ۲ تا ۴، تصویر، صوت و ویدئو)؛ بارگذاری مستقیم مرورگر به باکت S3 پارس‌پک از طریق Presigned URL و Finalize خودکار؛ دکمه‌های ذخیره پیش‌نویس یا انتشار مستقیم.
  - `src/pages/blog/[id].astro`: بارگذاری و ویرایش مقالات موجود با بلوک‌های ساخت‌یافته، تغییر وضعیت میان پیش‌نویس و انتشار، و پیوند مستقیم به مشاهده پیش‌نمایش زنده در سایت عمومی.
  - `src/pages/media/index.astro`: صفحه کتابخانه رسانه‌ها با نمایش مصرف سهمیه از سقف ۱۰ گیگابایت، نوار پیشرفت درصد مصرف، آمار فضای آزاد، جدول فایل‌ها همراه با کپی آسان شناسه رسانه و فرم آپلود مستقیم هر نوع فایل سازگار به باکت.
  - افزودن منوی دسترسی «رسانه‌ها و فایل‌ها» به سایدبار در `AdminLayout.astro`.
- وب‌سایت عمومی کلینیک (`apps/psychology-site`):
  - `src/pages/blog/index.astro`: صفحه فهرست مقالات عمومی با کارت‌های واکنش‌گرا و سازگار با موبایل، خلاصه سئو و پیوند مطالعه کامل.
  - `src/pages/blog/[slug].astro`: صفحه نمایش مطلب با اعتبارسنجی اسلاگ؛ تگ canonical بر اساس Host دامنه؛ بازگشت خطای ۴۰۴ در صورت عدم انتشار یا نبود مطلب؛ رندر امن بلوک‌های محتوا و عدم اجرای اسکریپت یا تگ‌های خطرناک؛ دریافت پیوند موقت استریم برای مدیاهای متصل و پلیرهای HTML5 صوت و ویدئو با قابلیت‌های کامل کنترل زمان، جلو/عقب بردن (seek) و بافر بهینه‌شده.
  - `src/pages/sitemap.xml.ts`: اندپوینت تولید نقشه سایت داینامیک مطابق پروتکل استاندارد sitemaps.org؛ تفکیک دقیق بر مبنای Host درخواست؛ درج آدرس‌های پایه، کارگاه‌ها و تمامی مقالات منتشرشده؛ تاریخ آخرین ویرایش (`lastmod`)؛ و حذف کامل و قطعی هرگونه پیش‌نویس، کارگاه لغوشده یا داده‌های تأییدنشده.
- آزمون‌های خودکار در `packages/psychology-api-client/test/index.test.mjs`:
  - تست فراخوانی صحیح اندپوینت‌های بلاگ و رسانه در کلاینت.
  - تست دریافت جزییات مطلب و پرتاب خطای ۴۰۴ برای پیش‌نویس یا اسلاگ نامعتبر.
  - تست صدور لینک پخش استریم با طول عمر ۳۰۰ ثانیه (۵ دقیقه).
  - تست چرخه کامل ساخت پیش‌نویس، انتشار، لغو انتشار (بازگشت به پیش‌نویس) و حذف در پنل.
  - تست چرخه آپلود مستقیم، نهایی‌سازی و سهمیه ۱۰ گیگابایت رسانه.
  - تست نقشه سایت per Host: تضمین درج دامنه‌ی ارائه‌دهنده در URLها و بررسی حذف آنی و کامل مقاله از sitemap به محض لغو انتشار.
  همه تست‌ها پاس شدند.

### ۲۰۲۶-۱۱-۲۱ — اجرای تسک PSY-1.10 (آپلود مستقیم پارس‌پک با Staging، فایل نهایی ثابت، اعتبارسنجی حجم و نوع، سهمیه ۱۰ گیگابایت، پاک‌سازی و ایزولاسیون)

- فایل‌های مهاجرت `000004_create_media.up.sql` و `000004_create_media.down.sql`:
  - جدول `media_assets`: نگهداری رسانه‌های نهایی و تأییدشده با کلید اصلی UUID، کلید ترکیبی `(tenant_id, id)`، کلید یکتای باکت `storage_key`، کلاس دسترسی (`public`, `course_private`)، وضعیت اعتبارسنجی (`pending`, `verified`, `rejected`)، فیلد اندازه ریالی/بایتی، کلید خارجی بارگذاری‌کننده و زمان تأیید.
  - جدول `media_uploads`: رهگیری رزروهای پیش از آپلود با کلید اختصاصی `staging_key`، مشخصات نوع و حجم مورد انتظار، زمان انقضای رزرو (۱۵ دقیقه)، وضعیت چرخه حیات (`reserved`, `completed`, `expired`).
  - فعال‌سازی و اجباری بودن RLS (`ENABLE ROW LEVEL SECURITY` و `FORCE ROW LEVEL SECURITY`) و خط‌مشی ایزولاسیون `tenant_isolation` با `app.tenant_id`.
  - مجوزهای دسترسی `app_user` و آزمون تایید صحت جفت مهاجرت‌ها در `migrations/migration_test.go`.
- لایه ذخیره‌سازی شیء در `internal/media/storage.go`:
  - اینترفیس `StorageProvider` با متدهای `PresignPutURL`, `PresignGetURL`, `HeadObject`, `CopyObject`, `DeleteObject`.
  - پیاده‌سازی `MockStorageProvider` در حافظه برای تست‌های خودکار بدون نیاز به شبکه خارجی.
  - پیاده‌سازی `S3StorageProvider` برای باکت خصوصی پارس‌پک مبتنی بر امضای استاندارد AWS SigV4 با کتابخانه استاندارد Go (`crypto/hmac`, `crypto/sha256`) بدون افزودن هرگونه dependency خارجی جدید (رعایت دقیق قانون ۴ در `AGENTS.md`).
- منطق کسب‌وکار در `internal/media/service.go`:
  - اعتبارسنجی سخت‌گیرانه نوع و حجم فایل بر اساس استانداردهای بریف:
    - تصویر عمومی: فرمت‌های JPEG، PNG و WebP حداکثر تا ۵ مگابایت.
    - صوت بلاگ: فرمت MP3 حداکثر تا ۳۲ مگابایت.
    - ویدئوی بلاگ: فرمت MP4 حداکثر تا ۱۲۸ مگابایت.
    - صوت درس: فرمت MP3 حداکثر تا ۱۲۸ مگابایت.
    - ویدئوی درس: فرمت MP4 حداکثر تا ۵۱۲ مگابایت.
    - رد هرگونه فرمت نامعتبر یا اندازه بزرگتر از سقف با خطای `ErrInvalidMimeType` یا `ErrFileSizeExceeded`.
  - کنترل سهمیه ذخیره‌سازی: سقف کل ۱۰ گیگابایت برای هر کلینیک؛ محاسبه اتمیک از مجموع فایل‌های تأییدشده و رزروهای معتبر در جریان؛ رد درخواست جدید در صورت عبور از سقف با خطای 403 Forbidden (`ErrQuotaExceeded`).
  - فرآیند آپلود مستقیم با دو فاز Staging و Finalize:
    - فاز اول: صدور URL موقت (TTL ۱۵ دقیقه) برای کلید اختصاصی Staging در باکت خصوصی (`staging/{tenant_id}/{upload_id}_{filename}`).
    - فاز دوم: استعلام سروری فایل از استوریج با `HeadObject` و اطمینان از تطابق حجم و نوع، انتقال اتمیک شیء به کلید نهایی تغییرناپذیر (`assets/{tenant_id}/{asset_id}_{filename}`)، حذف شیء موقت Staging، ثبت رکورد تأییدشده در `media_assets` و خاتمه رزرو.
  - تضمین عدم بازنویسی و تغییرناپذیری فایل‌ها: اختصاص UUID مجزا برای هر دارایی حتی با نام فایل مشابه، ممانعت از بازنویسی فایل‌های موجود، و یکتایی `storage_key` در سطح دیتابیس.
  - پخش رسانه با URL کوتاه‌مدت: اندپوینت پخش عمومی با عمر حداکثر ۵ دقیقه (۳۰۰ ثانیه) پس از بررسی انتشار و وضعیت عمومی بودن رسانه؛ ممانعت از پخش فایل‌های خصوصی درس (`course_private`) از طریق اندپوینت عمومی.
  - کارگر پاک‌سازی رزروهای منقضی (`PurgeExpiredUploads`): پاک‌سازی اشیاء موقت باقیمانده در Staging و به‌روزرسانی وضعیت به `expired` جهت آزادسازی سهمیه رزروشده.
- اندپوینت‌های HTTP در `internal/media/http.go`:
  - `POST /admin/media/upload-url`: درخواست مجوز آپلود مستقیم با احراز هویت، عضویت کلینیک، نقش‌های `owner` و `content_manager` و محافظت CSRF.
  - `POST /admin/media/finalize`: نهایی‌سازی و بررسی فایل آپلودشده.
  - `GET /admin/media`: فهرست رسانه‌های کلینیک و محاسبه میزان مصرف و سقف سهمیه.
  - `GET /public/media/{id}/stream-url` و `GET /public/media/{id}/stream`: دریافت لینک موقت پخش استریم برای دارایی عمومی تأییدشده بر مبنای Host دامنه.
- آزمون‌های خودکار در `internal/media/media_test.go`:
  - `TestValidateFileSpecs`: سنجش اعتبارسنجی تمام فرمت‌ها و سقف‌های حجمی و رد انواع نامعتبر یا اندازه‌های نامجاز.
  - `TestStorageQuotaEnforcement`: آزمون اعمال سقف ۱۰ گیگابایت و رد رزرو مازاد با خطای ۴۰۳.
  - `TestStagingUploadAndFinalization`: آزمون کامل چرخه رزرو موقت، بررسی سروری، انتقال به فایل نهایی و حذف Staging و ممانعت از Finalize تکراری.
  - `TestFileOverwritePreventionAndImmutability`: بارگذاری دو فایل با نام یکسان و بررسی تغییرناپذیری و عدم بازنویسی با کلیدهای متمایز.
  - `TestCrossTenantIsolation`: تلاش مستاجر B برای نهایی‌سازی، مشاهده یا پخش فایل مستاجر A و ممانعت قطعی بر اساس ایزولاسیون RLS و دامنه.
  - `TestExpiredStagingCleanup`: پاک‌سازی رکوردهای رزرو منقضی و حذف فایل‌های Staging و بازیافت سهمیه.
  - `TestPublicMediaStreamingEndpoint`: صدور لینک کوتاه‌مدت ۵ دقیقه‌ای برای دارایی عمومی و رد فایل‌های خصوصی درس با ۴۰۳.
  - `TestAdminMediaEndpointsAndRoles`: اعتبارسنجی نقش‌ها (پذیرش `owner` و `content_manager`، رد `receptionist` با ۴۰۳)، الزام CSRF و لیست رسانه‌ها.
  همه تست‌ها پاس شدند.

- فایل‌های مهاجرت `000003_create_blog.up.sql` و `000003_create_blog.down.sql`:
  - جدول `blog_posts` با کلید اصلی UUID، کلید خارجی `tenant_id`، محدودیت یکتایی ترکیبی `(tenant_id, id)` و `(tenant_id, slug)`، فیلدهای سئو و بلوک‌های ساخت‌یافته JSONB.
  - فعال‌سازی و اجباری بودن RLS (`ENABLE ROW LEVEL SECURITY` و `FORCE ROW LEVEL SECURITY`) و خط‌مشی ایزولاسیون با `app.tenant_id`.
  - مجوزهای دسترسی `app_user` و آزمون تایید صحت جفت مهاجرت‌ها در `migrations/migration_test.go`.
- ماژول `internal/blog`:
  - کوئری‌های SQL تدوین و پکیج `blogdb` توسط `sqlc generate` تولید شد.
  - سرویس تجاری در `service.go`:
    - اعتبارسنجی اسلاگ (تنها ارقام، حروف کوچک و خط تیره مجاز است).
    - یکتایی اسلاگ در سطح هر کلینیک: ثبت اسلاگ تکراری در همان کلینیک با خطای 409 (`ErrDuplicateSlug`) رد می‌شود، در حالی که استفاده از اسلاگ مشابه در کلینیک‌های متفاوت کاملاً مجاز است.
    - اعتبارسنجی بلوک‌های محتوا (`paragraph`, `heading`, `image`, `video`, `audio`): ممانعت اکید از تزریق کدهای جاوااسکریپت و تگ‌های HTML ناامن (`<script>`, `javascript:`, `<iframe>`, `<object>`, `<embed>`, رویدادهای `onload`/`onerror`) با خطای `ErrScriptInjection`.
    - ایزولاسیون پیش‌نویس: اندپوینت‌های عمومی (`GET /public/blog` و `GET /public/blog/{slug}`) صرفاً مطالب در وضعیت `published` را بازمی‌گردانند و هرگونه تلاش برای دسترسی به پیش‌نویس از مسیر عمومی منجر به 404 Not Found می‌شود.
- هندلر HTTP در `http.go`:
  - اندپوینت‌های عمومی: `GET /public/blog` و `GET /public/blog/{slug}` با تفکیک بر مبنای Host دامنه.
  - اندپوینت‌های پنل مدیریت: `GET /admin/blog`, `GET /admin/blog/{id}`, `POST /admin/blog`, `PUT /admin/blog/{id}`, `POST /admin/blog/{id}/publish`, `POST /admin/blog/{id}/unpublish`, `DELETE /admin/blog/{id}` با کنترل احراز هویت نشست، عضویت کلینیک، محافظت CSRF و محدودیت به نقش‌های `owner` و `content_manager` (رد نقش `receptionist` یا متفرقه با 403 Forbidden).
- آزمون‌های خودکار در `internal/blog/blog_test.go`:
  - `TestPublicBlog_DraftIsolation`: عدم نشت پیش‌نویس به عمومی و مشاهده بلافاصله پس از انتشار و مخفی شدن پس از لغو انتشار.
  - `TestAdminBlog_ContentBlockValidation_And_ScriptInjection`: پذیرش بلوک‌های استاندارد، رد انواع نامعتبر، رد تیتر با سطح غیرمجاز و ممانعت از تزریق اسکریپت و آی‌فریم.
  - `TestAdminBlog_SlugUniquenessPerTenant`: تایید یکتایی در مستاجر واحد و مجاز بودن در مستاجرهای مختلف.
  - `TestAdminBlog_RoleAccessControl`: مجاز بودن `owner` و `content_manager` و رد `receptionist` با ۴۰۳ و محافظت CSRF.
  - `TestCrossTenantIsolation`: عدم دسترسی متقابل مستاجر A به مقالات مستاجر B.
  همگی پاس شدند.

### ۲۰۲۶-۱۱-۲۱ — اجرای تسک PSY-1.8 (رابط کاربری کارگاه، پنل شرکت‌کنندگان، فرم و نتیجه ثبت‌نام موبایل)

- ماژول کلاینت `@psychology/api-client`:
  - متدهای تایپ‌شده کامل برای کارگاه‌های عمومی (`listPublicWorkshops`, `getPublicWorkshop`, `registerPublicWorkshop`, `getPublicRegistrationStatus`)، مدیریت کارگاه در پنل (`listAdminWorkshops`, `getAdminWorkshop`, `createAdminWorkshop`, `updateAdminWorkshop`, `publishAdminWorkshop`, `closeAdminWorkshop`, `cancelAdminWorkshop`, `listAdminRegistrations`, `cancelAdminRegistration`) و پیگیری بازپرداخت‌ها (`listAdminPayments`, `recordAdminRefund`) پیاده‌سازی و تایپ‌چک شدند.
- صفحات وب‌سایت عمومی کلینیک (`apps/psychology-site`):
  - `src/pages/workshops/index.astro`: فهرست کارگاه‌های منتشرشده با کارت‌های واکنش‌گرا و سازگار با موبایل، نمایش وضعیت باز بودن ثبت‌نام، نمایش ظرفیت باقیمانده، زمان شروع در منطقه زمانی `Asia/Tehran` و نمایش قیمت به تومان طبق مصوبه D-14 (یا برچسب «رایگان»).
  - `src/pages/workshops/[id].astro`: صفحه معرفی و ثبت‌نام کارگاه؛ نمایش مشخصات کامل، توضیحات و سرفصل‌ها؛ فرم ثبت‌نام واکنش‌گرا با نرمال‌سازی خودکار ارقام فارسی به انگلیسی و اعتبارسنجی شماره ۱۱ رقمی همراه با پیش‌شماره 09؛ ثبت آنی و قطعی برای کارگاه رایگان یا ایجاد رزرو موقت ۱۵ دقیقه‌ای و ارائه پیوند درگاه پرداخت؛ و اطلاع‌رسانی روشن در زمان تکمیل ظرفیت یا پایان مهلت ثبت‌نام.
  - `src/pages/workshops/registrations/[id].astro`: صفحه پیگیری خصوصی وضعیت ثبت‌نام با توکن اختصاصی (`lookup_token`)؛ پوشش وضعیت‌های `confirmed` (ثبت قطعی و کد رهگیری بانک)، `pending_payment` (شمارش معکوس ۱۵ دقیقه و دکمه پرداخت مجدد)، `refund_required` (توضیح شفاف انتقال وجه به صف استرداد به دلیل انقضای رزرو یا لغو کارگاه با عدم اشغال صندلی)، `expired` و `cancelled`.
- صفحات پنل مدیریت کلینیک (`apps/psychology-admin`):
  - `src/pages/workshops/index.astro`: فهرست کارگاه‌ها با فیلتر وضعیت (همه، منتشرشده، پیش‌نویس، بسته‌شده، لغو شده)، درصد تکمیل ظرفیت، آمار تفکیکی صندلی‌های قطعی و موقت و دسترسی‌های سریع.
  - `src/pages/workshops/new.astro`: فرم تعریف کارگاه جدید با فیلدهای عنوان، نامک انگلیسی، مدرس، محل، تاریخ و ساعت شروع و پایان، مهلت ثبت‌نام، ظرفیت و شهریه ریالی همراه با پیش‌نمایش زنده معادل تومانی و ذخیره پیش‌نویس.
  - `src/pages/workshops/[id].astro`: مدیریت چرخه کارگاه (انتشار، بستن ثبت‌نام و لغو کامل کارگاه با اخطار آزادسازی صندلی‌ها و انتقال خریدهای پرداخت‌شده به بازپرداخت)، مدیریت ظرفیت با ممانعت از کاهش به کمتر از صندلی‌های فعال، و جدول شرکت‌کنندگان با امکان لغو انفرادی ثبت‌نام توسط پذیرش یا مالک جهت آزادسازی فوری صندلی برای متقاضیان بعدی.
  - `src/pages/payments/index.astro`: صف پیگیری `refund_required` ویژه مالک با جدول مشخصات و کد رهگیری درگاه و مودال ثبت دستی سند بازپرداخت به صورت کاملاً Idempotent.
  - پیوند بخش کارگاه‌ها و بازپرداخت‌ها در سایدبار `AdminLayout.astro`.
- اجرای کامل `pnpm run verify:psychology` و تست‌ها و بیلد Go بدون هیچ خطا.

### ۲۰۲۶-۱۱-۲۱ — اجرای تسک PSY-1.7 (پردازش محدود تطبیق پرداخت، بازیابی پس از کرش، صف و ثبت بازپرداخت توسط مالک)

- منطق پردازش پس‌زمینه و تطبیق معوقات (`ReconcileStalePayments`) در `internal/payments/service.go`:
  - پردازش دسته‌ای محدود (`LIMIT`) روی رکوردهای `initiated` با عمر بیش از ۱۵ دقیقه در PostgreSQL بدون نیاز به صف یا سرویس خارجی (مانند Redis/Kafka) طبق اصول معماری تک‌پایه.
  - انقضای اتمیک نگه‌داشت‌های معوقه پیش از آغاز پردازش (`ExpirePendingRegistrations`).
  - تطبیق خودکار با درگاه: در صورت پرداخت موفق مشتری اگر نگه‌داشت منقضی شده یا کارگاه لغو شده باشد، به `refund_required` منتقل شده و هیچ صندلی جدیدی اشغال نمی‌شود (تضمین Zero-Oversell).
  - تفکیک خطاهای گذرا و شبکه (`ErrGatewayTimeout` / `ErrGatewayUnavailable`): رکوردها در وضعیت `initiated` باقی مانده و در چرخهٔ بعدی مجدداً بررسی می‌شوند بدون این‌که به اشتباه شکست خورده اعلام شوند.
  - بازیابی پس از کرش (`Crash Recovery`): با اجرای پردازش بدون وضعیت حافظه‌ای و تکیه بر وضعیت سطرها در پایگاه داده، در صورت توقف ناگهانی پروسه یا ری‌استارت سرور، اجرای مجدد بدون اثر تکراری موارد باقیمانده را تکمیل می‌کند.
- اندپوینت‌های اختصاصی پنل مدیریت (محدود به نقش `owner` و محافظت‌شده با هدرهای نشست و CSRF):
  - `GET /admin/payments?status=refund_required`: دریافت فهرست خریدهای نیازمند بازپرداخت همراه با مشخصات، مبلغ ریالی و کد رهگیری درگاه.
  - `POST /admin/payments/{purchase_id}/refund`: ثبت دستی بازپرداخت با شماره پیگیری بانکی، علت و مبلغ.
  - تضمین رفتار Idempotent: ثبت دوبارهٔ همان شماره مرجع برای همان خرید اثر تازه ندارد و همان رکورد قبلی را بازمی‌گرداند.
  - بررسی صحت تجاری: مسدودسازی بازپرداخت برای خریدهای غیر از `refund_required` با خطای 400، مغایرت مبلغ با خطای 400، و ثبت مرجع تکراری روی خرید دیگر با خطای 409 Conflict.
  - ایزولاسیون نقش و مستاجر: جلوگیری از دسترسی نقش `receptionist` یا سایر کاربران به بخش بازپرداخت با خطای 403 Forbidden و ایزولاسیون کامل چندمستاجری.
- آزمون‌های خودکار در `internal/payments/payments_test.go`:
  - `TestReconcileStalePayments_AutoConfirmAndRefund`
  - `TestReconcileStalePayments_CrashRecoveryAndRetry`
  - `TestAdminPayments_OwnerOnlyAccessAndListing`
  - `TestRecordRefund_SuccessAndIdempotency`
  همگی با موفقیت پاس شدند.

### ۲۰۲۶-۱۱-۲۱ — اجرای تسک PSY-1.6 (کال‌بک درگاه، اعتبارسنجی سروری Verify و تطبیق اتمیک اثر تجاری)

- متد `HandleCallbackAndVerify` در `internal/payments/service.go` پیاده‌سازی شد:
  - استعلام اعتبارسنجی سمت سرور (`VerifyPayment`) کاملاً خارج از هرگونه قفل یا تراکنش باز پایگاه داده انجام می‌شود.
  - تطبیق نتیجه در پایگاه داده با رعایت دقیق سلسله‌مراتب قفل‌ها پیاده شد: قفل سطر کارگاه ← قفل سطر ثبت‌نام ← به‌روزرسانی خرید و تلاش پرداخت.
- تضمین رفتار Idempotent: فراخوانی تکراری Callback برای تلاشی که قبلاً تایید شده است، بدون فراخوانی مجدد درگاه، همان نتیجه معتبر را بازمی‌گرداند و هیچ اثر تجاری تکراری (اعطای صندلی مضاعف) تولید نمی‌کند.
- شناسایی Mismatch و عدم اعتماد به کلاینت: پارامترهای ارسالی مرورگر بررسی شده و هرگونه ناهماهنگی میان مبلغ، مستاجر یا شناسه مرجع منجر به مسدودسازی تراکنش با `ErrAmountMismatch` می‌شود.
- مدیریت سناریوی پرداخت با تاخیر (انقضای نگه‌داشت صندلی یا لغو کارگاه):
  - در صورتی که مشتری وجه را پرداخت کرده باشد اما مهلت ۱۵ دقیقه‌ای رزرو صندلی سپری شده یا کارگاه توسط مالک لغو شده باشد:
    - پرداخت در `payment_attempts` با شناسه مرجع بانک به عنوان `verified` ثبت می‌شود.
    - وضعیت سفارش در `purchases` به `refund_required` تغییر می‌یابد.
    - صندلی کارگاه اشغال نمی‌شود (ثبت‌نام منقضی/لغو باقی می‌ماند تا تضمین Zero-Oversell نقض نشود).
- هندلر HTTP در `internal/payments/http.go` برای مسیر `GET /callbacks/payments/zarinpal` همراه با تفکیک Host مستاجر و هدایت ریدایرکت ۳۰۲ به صفحه پیگیری نتیجه پیاده‌سازی شد.
- آزمون‌های خودکار `TestCallback_SuccessfulPaymentAndHoldConfirmation`، `TestCallback_IdempotentDuplicateCallback`، `TestCallback_UserCanceledOnGateway`، `TestCallback_MismatchDetection`، `TestCallback_PaymentAfterHoldExpired_RefundRequired`، `TestCallback_PaymentAfterWorkshopCancelled_RefundRequired` و `TestCallback_HTTPHandlerIntegration` پیاده‌سازی شدند و همگی پاس شدند.

### ۲۰۲۶-۱۱-۲۱ — اجرای تسک PSY-1.5 (اینترفیس PaymentProvider، آداپتور زرین‌پال و شروع پرداخت با اسنپ‌شات ریالی)

- اینترفیس `PaymentProvider` همراه با ساختارهای استاندارد `PaymentRequest`، `PaymentResponse`، `VerifyRequest`، `VerifyResponse` و خطاهای تایپ‌شده در `internal/payments/provider.go` پیاده‌سازی شد.
- آداپتور آزمایشی `MockPaymentProvider` در `internal/payments/mock_provider.go` با تولید شناسه Authority قطعی، ثبت تاریخچه فراخوانی‌ها، و پشتیبانی از شبیه‌سازی انواع خطاها (تایم‌اوت، خطای شبکه، رد شدن توسط درگاه) ایجاد شد.
- آداپتور `ZarinPalProvider` در `internal/payments/zarinpal_provider.go` برای تعامل مستقیم با API نسخه ۴ زرین‌پال (پشتیبانی از Sandbox و Production، تشخیص تایم‌اوت‌های شبکه با `context.DeadlineExceeded` و `net.Error`، تفکیک کدهای خطای درگاه و مدیریت پاسخ‌های ناهمگن آرایه/شیء PHP) پیاده‌سازی شد.
- متد `InitiateWorkshopPayment` در سرویس پرداخت طراحی شد:
  - ایجاد Snapshot ریالی و غیرقابل‌تغییر در `purchases` از روی قیمت قطعی کارگاه در پایگاه داده (عدم اعتماد به داده کلاینت).
  - نگه داشتن کامل فراخوانی‌های شبکه خارج از هرگونه قفل یا تراکنش باز پایگاه داده جهت جلوگیری از قفل شدن ردیف‌ها و منابع در زمان تاخیر درگاه.
  - پوشش مسیر شروع با نتیجه نامعلوم: در زمان وقوع تایم‌اوت شبکه، هیچ تلاش نامعتبر با Authority خالی در دیتابیس ثبت نمی‌شود و سفارش در وضعیت `pending` باقی می‌ماند تا کلاینت بتواند با همان `idempotency_key` درخواست را دوباره به صورت امن ارسال کند.
  - پشتیبانی از idempotency: درخواست تکراری با همان کلید، بدون فراخوانی مجدد درگاه، همان تلاش قبلی را بازمی‌گرداند.
  - جلوگیری از ارسال هم‌زمان تلاش‌های جدید برای یک خرید با کلیدهای متناقض طبق قانون «تا زمان تعیین تکلیف تلاش قبلی، همان خرید دوباره به درگاه فرستاده نمی‌شود».
  - اتصال خودکار به `handleRegisterPublicWorkshop` جهت ارائه پیوند پرداخت در پاسخ ثبت‌نام‌های پولی.
- آزمون‌های خودکار `TestMockPaymentProvider_Basic`، `TestZarinPalProvider_RequestPayment_Success`، `TestZarinPalProvider_RequestPayment_GatewayRejection`، `TestZarinPalProvider_RequestPayment_Timeout`، `TestInitiatePayment_RialsSnapshot`، `TestInitiatePayment_UncertainResultAndTimeout`، `TestInitiatePayment_IdempotencyWithSameKey`، `TestInitiatePayment_ActiveAttemptInProgressBlocked`، `TestInitiatePayment_ExpiredRegistrationBlocked` و `TestInitiatePayment_CrossTenantIsolation` نوشته شدند و همگی با موفقیت پاس شدند.

### ۲۰۲۶-۱۱-۲۱ — اجرای تسک PSY-1.4 (ثبت‌نام مهمان، رزرو اتمیک صندلی و صفحه پیگیری خصوصی)

- تابع `NormalizePhone` جهت تبدیل ارقام فارسی و عربی، حذف پیشوندهای `+98`، `0098`، `98` و بدون صفر به قالب استاندارد ۱۱ رقمی `09XXXXXXXXX` پیاده‌سازی و با انواع داده‌های معتبر و نامعتبر آزموده شد.
- فرایند رزرو صندلی در `RegisterGuest` پیاده شد:
  - کارگاه رایگان (`price_rials = 0`) در همان مرحله به وضعیت `confirmed` درآمده و `expires_at` ندارد.
  - کارگاه پولی به وضعیت `pending_payment` با مهلت ۱۵ دقیقه‌ای رفته و در صورت نزدیک بودن `registration_deadline`، زمان انقضا دقیقاً به مهلت کارگاه محدود (cap) می‌شود.
  - توکن پیگیری امن (`lookup_token`) تولید شده و هش SHA-256 آن در پایگاه داده ذخیره می‌شود.
- رفتار Idempotent در ارسال‌های مکرر پیاده‌سازی شد: اگر کلاینت با همان توکن درخواست ثبت‌نام فعال را دوباره بفرستد، همان رکورد بدون خطای تکراری و به صورت امن بازگردانده می‌شود؛ اما ارسال شماره یکسان توسط کلاینت دیگر با خطای 409 Conflict (`DUPLICATE_REGISTRATION`) بدون نشت هیچ داده‌ای از متقاضی قبلی مسدود می‌شود.
- تضمین Zero-Oversell: کنترل هم‌روندی با قفل اتمیک کارگاه پیش از شمارش صندلی‌ها اعمال شد. آزمون رقابت ۲۵ گوروتین هم‌زمان برای تنها ۱ صندلی خالی کارگاه اجرا گردید که دقیقاً ۱ درخواست موفق و ۲۴ درخواست دیگر با خطای `ErrCapacityFull` متوقف شدند و تعداد صندلی‌های رزروشده در دیتابیس دقیقاً ۱ باقی ماند.
- اندپوینت‌های عمومی و پنل اضافه شدند:
  - `POST /public/workshops/{id}/register`
  - `GET /public/workshops/registrations/{id}?lookup_token=...` (استعلام وضعیت بدون نشت اطلاعات، بررسی توکن و ایزولاسیون دامنه)
  - `GET /admin/workshops/{id}/registrations` (مشاهده فهرست شرکت‌کنندگان توسط نقش‌های `receptionist` و `owner`)
  - `POST /admin/workshops/{id}/registrations/{reg_id}/cancel` (لغو ثبت‌نام توسط پذیرش/مالک و آزادسازی فوری صندلی برای متقاضیان بعدی)
- آزمون‌های خودکار `TestPhoneNormalization`، `TestRegisterGuest_FreeVsPaidWorkshops`، `TestDuplicateRegistration_IdempotencyAndNoDataLeak`، `TestAtomicSeatReservation_ConcurrentRaceCondition`، `TestPublicRegistrationStatusQuery` و `TestAdminAttendeeManagement` نوشته و همگی پاس شدند.

- هندلرهای عمومی `GET /public/workshops` و `GET /public/workshops/{id}` پیاده‌سازی شدند: داده‌ها تنها از روی Host تأییدشده تفکیک می‌شوند، پیش‌نویس‌ها از دید عموم مخفی هستند و ظرفیت باقیمانده و وضعیت باز بودن مهلت ثبت‌نام در سرور محاسبه می‌شود.
- هندلرهای مدیریت پنل `GET /admin/workshops`، `GET /admin/workshops/{id}`، `POST /admin/workshops`، `PUT /admin/workshops/{id}`، `POST /admin/workshops/{id}/publish` و `POST /admin/workshops/{id}/close` با نقش‌های `owner` و `content_manager` و هدر `X-Tenant-ID` و کوکی نشست پیاده شدند.
- عملیات لغو کارگاه `POST /admin/workshops/{id}/cancel` با نقش ویژه `owner` به دلیل پیامدهای مالی/بازپرداخت محدود شد و تلاش سایر نقش‌ها با 403 Forbidden رد می‌شود.
- قانون ممنوعیت تغییر قیمت، تاریخ‌ها یا مکان کارگاه پس از ثبت اولین شرکت‌کننده قطعی با خطای 409 Conflict (`CANNOT_MODIFY_ACTIVE`) پیاده‌سازی و آزموده شد.
- آزمون‌های خودکار `TestPublicWorkshops_OnlyTenantData`، `TestAdminWorkshops_RoleAccessAndTransitions` و `TestAdminWorkshops_ActiveWorkshopModificationBlocked` نوشته شدند و پاس شدند.

### ۲۰۲۶-۱۱-۲۰ — اجرای تسک PSY-1.2 (مهاجرت و کوئری‌های کارگاه، ثبت‌نام و پرداخت با RLS)

- فایل‌های مهاجرت `000002_create_workshops_and_payments.up.sql` و `000002_create_workshops_and_payments.down.sql` شامل جدول‌های `workshops`، `workshop_registrations`، `purchases`، `payment_attempts` و `refunds` با RLS فعال و اجباری (`FORCE ROW LEVEL SECURITY`) و دسترسی به `app_user` ایجاد شدند.
- کوئری‌های SQL در `internal/workshops/queries.sql` و `internal/payments/queries.sql` تدوین شده و پکیج‌های `workshopsdb` و `paymentsdb` با `sqlc generate` تولید شدند.
- منطق تجاری و مدیریت ظرفیت صندلی‌ها در `internal/workshops/service.go` با رعایت قفل ردیف والد، بررسی مهلت ثبت‌نام، محاسبه اتمیک صندلی‌های اشغال‌شده و کنترل انقضای ۱۵ دقیقه‌ای نگه‌داشت صندلی پیاده‌سازی شد.
- آزمون‌های خودکار در `internal/workshops/workshops_test.go` نوشته شدند و پاس شدند:
  - `TestZeroSeats_CapacityFullRejection`: رد درخواست ثبت‌نام پس از تکمیل ظرفیت صندلی‌ها با `ErrCapacityFull`.
  - `TestExpiredReservation_DoesNotOccupySeat`: عدم اشغال صندلی توسط نگه‌داشت‌های منقضی (`pending_payment` با `expires_at` سپری‌شده) و امکان ثبت‌نام متقاضی جدید.
  - `TestCapacityUpdate_ConstraintsAndValidation`: مجاز بودن کاهش ظرفیت تا سقف ثبت‌نام‌های فعال و مسدودسازی کاهش ظرفیت به کمتر از رزروهای جاری با `ErrCannotReduceCapacityBelowActive`.
  - `TestDuplicateActivePhone_Blocked`: مسدودسازی ثبت‌نام تکراری هم‌زمان با یک شماره تماس در کارگاه واحد.
  - `TestCrossTenantIsolation_WorkshopAccess`: حفظ کامل ایزولاسیون و عدم دسترسی مستاجر B به کارگاه‌ها یا ثبت‌نام مستاجر A.

### ۲۰۲۶-۱۱-۲۰ — اجرای تسک PSY-1.1 (طراحی ERD، قرارداد OpenAPI و تایپ‌های فاز ۱)

- جدول‌های فاز ۱ در `docs/psychology/database.md` مستند شدند: `workshops`، `workshop_registrations`، `purchases`، `payment_attempts`، `refunds`، `blog_posts`، `media_assets` و `media_uploads` با RLS اجباری، کلیدهای خارجی ترکیبی و ایندکس‌های دقیق.
- ماتریس دسترسی نقش‌ها (`owner`, `content_manager`, `receptionist`, `therapist`)، نمودارهای چرخه وضعیت (State Machines)، مهلت‌های زمانی (TTL صندلی ۱۵ دقیقه، لینک پخش ۵ دقیقه، سهمیه ۱۰ گیگابایت) و ترتیب قفل‌ها (ابتدا کارگاه، سپس ثبت‌نام) ثبت و نهایی شدند.
- قرارداد OpenAPI در `packages/contracts/psychology/openapi.yaml` برای تمام اندپوینت‌های عمومی و پنل مدیریت کارگاه، ثبت‌نام مهمان، پیگیری وضعیت، پرداخت و استرداد، بلاگ و آپلود امن مستقیم رسانه تدوین شد.
- مدل‌های تایپ‌شده کلاینت TypeScript در `packages/psychology-api-client/src/types.ts` به‌روزرسانی شده و صحت بیلد و تایپ‌چک پروژه با `pnpm run verify:psychology` و تست‌های Go تأیید شد.

### ۲۰۲۶-۱۱-۱۹ — پایان فاز PSY-0 و اجرای تسک PSY-0.8 (تأیید یکپارچه و آزمون PSY-A01)

- دستور `make verify-psychology` در `Makefile` ریشه و اسکریپت معادل `pnpm run verify:psychology` در `package.json` تعریف شدند.
- سناریوی پذیرش `PSY-A01` در `apps/psychology-api/internal/testsupport/acceptance_a01_test.go` با دو fixture کامل:
  - کلینیک A: «مطب دکتر رضایی» روی دامنه `rezaei.ir`
  - کلینیک B: «مرکز مشاوره نیلوفر» روی دامنه `niloofar.ir`
  - دامنه تأییدنشده `pending-clinic.ir`
  پیاده‌سازی و آزموده شد: تفکیک کامل پاسخ‌های عمومی، رد دامنه‌های غیرمعتبر، و مسدودسازی دستکاری متقاطع تنظیمات کلینیک دیگر با خطای 403 Forbidden تأیید گردیدند.
- تمام ۸ تسک فاز `PSY-0` با موفقیت تیک خوردند و زیرساخت هسته آماده ورود به فاز ۱ (کارگاه و بلاگ) شد.

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
