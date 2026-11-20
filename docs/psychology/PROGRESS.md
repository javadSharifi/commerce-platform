# وضعیت مسیر روان‌شناسی

- آخرین کار تکمیل‌شده — `PSY-1.13` (پایان فاز ۱)؛ اجرای کامل سناریوهای پایلوت برای دو کلینیک مستقل (کارگاه رایگان/بلاگ/رسانه و کارگاه پولی/درگاه/ظرفیت/بازپرداخت)، آزمون‌های خودکار سناریوهای پذیرش PSY-A01 تا PSY-A06 و PSY-A10، تدوین کتابچه عملیات پشتیبانی و ثبت شواهد پذیرش.
- تسک در حال انجام — هیچ‌کدام.
- تسک پیشنهادی بعدی — `PSY-2.1` (شروع نسخهٔ ۲ · نوبت حضوری: تکمیل schema، migration، OpenAPI و تست RLS درمانگر، زمان کاری، سانس، نوبت و تاریخچهٔ وضعیت).
- دستور تأیید برنامه — `make verify-psychology` (یا در ریشه `pnpm run verify:psychology` و در بک‌اند `cd apps/psychology-api && go test -v ./...`).

## تأییدهای محیط هدف

- حساب هدف زرین‌پال، باکت پارس‌پک، دامنه و تنظیمات آروان، شرایط عرضه و نگه‌داری داده در تسک‌های عرضهٔ هر نسخه بررسی می‌شوند. مسیر محلی با دادهٔ ساختگی و Adapter پرداخت تستی قابل اجراست. [نقشهٔ راه](TASKS.md)

## گزارش‌ها

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
