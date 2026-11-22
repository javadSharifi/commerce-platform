# الگوی ۰۴ — دیزاین سیستم مشترک (`packages/ui`) و موتور چندقالبی (`templates`)

> هر زمان روی `packages/ui`، صفحات `psychology-site`، صفحات `psychology-admin` یا ساخت یک قالب جدید کار می‌کنی، **اول این فایل را بخوان و کلمه به کلمه رعایت کن.**

---

## ۱. اصل معماری ۸۰/۲۰ (قالب‌ها احمق‌اند)

برای اینکه بتوانیم در آینده ۱۰ قالب یا بیشتر بدون دوباره‌کاری اضافه کنیم، مرز بین **منطق** و **ظاهر** مطلق است:

- **۸۰٪ هسته مشترک (در `src/pages/` و `packages/ui`):** دریافت دیتا از API، ساخت SEO/Meta، کش HTTP، احراز هویت، منطق رزرو نوبت، پرداخت، پخش دوره و فرم‌های پنل ادمین. این بخش‌ها **هرگز** داخل پوشه قالب‌ها تکرار نمی‌شوند.
- **۲۰٪ پوسته قابل تعویض (در `apps/psychology-site/src/templates/<template-id>/`):** فقط چیدمان بصری هدر، فوتر، بخش Hero صفحه اصلی، کارت دوره، کارت کارگاه و کارت مقاله + متغیرهای تم CSS آن قالب.

---

## ۲. قوانین غیرقابل نقض برای تمام قالب‌ها

1. **ممنوعیت مطلق منطق و API در قالب:** هیچ فایلی داخل `apps/psychology-site/src/templates/` حق ایمپورت کردن `@psychology/api-client`، صدا زدن `fetch`، خواندن کوکی یا اجرای منطق تاریخ/مالی را ندارد. تمام دیتا از طریق `src/pages/` به شکل ViewModel آماده (مثل `priceLabel: "۲۵۰,۰۰۰ تومان"`) به قالب پاس داده می‌شود.
2. **ممنوعیت رنگ و فونت هاردکد:** نوشتن رنگ ثابت مثل `#fff`، `#0284c7` یا `bg-blue-500` در قالب‌ها و کامپوننت‌های `packages/ui` ممنوع است. فقط از توکن‌های معنایی DaisyUI/Tailwind استفاده کن:
   - پس‌زمینه: `bg-base-100`, `bg-base-200`, `bg-base-300`
   - متن: `text-base-content`, `text-primary`, `text-primary-content`
   - اکشن‌ها و وضعیت‌ها: `btn-primary`, `btn-secondary`, `badge-success`, `alert-error`
3. **قانون اجباری RTL (پراپرتی‌های منطقی):** استفاده از `ml-`, `mr-`, `pl-`, `pr-`, `left-`, `right-`, `text-left`, `text-right` ممنوع است. فقط از `ms-`, `me-`, `ps-`, `pe-`, `start-`, `end-`, `text-start`, `text-end` استفاده کن.
4. **صفر کیلوبایت JS اضافه روی صفحات محتوایی:** کامپوننت‌های نمایشی قالب با `.astro` و SSR رندر می‌شوند. برای بخش‌های تعاملی (مثل منوی موبایل، مودال، سلکت یا تقویم شمسی) فقط کامپوننت آماده را از `@psychology/ui` ایمپورت کن و در سایت عمومی فقط با `client:visible` یا `client:idle` هیدریت کن.
5. **ممنوعیت اجرای کد دلخواه مشتری (قانون ۱۲ `AGENTS.md`):** قالب فقط با شناسه ثبت‌شده در `registry.ts` انتخاب می‌شود. هر `template_id` نامعتبر باید بدون خطا به قالب پیش‌فرض (`minimal`) برگردد (Safe Fallback).
6. **تزریق امن توکن‌ها در SSR (بدون FOUC):** رنگ برند (`primary_color`) قبل از تزریق در `BaseLayout.astro` باید دوباره با Regex معتبر `#RRGGBB` چک شود. مقادیر `theme_radius` و `theme_scale` فقط از مجموعه‌های مجاز (`sm | md | lg`) پذیرفته می‌شوند.
7. **بررسی API کتابخانه‌ها قبل از کدنویسی:** هنگام استفاده از `daisyui`، `react-aria-components` یا `@internationalized/date`، هرگز نام export یا props را حدس نزن؛ فایل `.d.ts` نصب‌شده در `node_modules` را چک کن.
8. **منبع مشترک قالب‌ها:** پنل و سایت متادیتا را از `packages/ui/src/template-catalog.ts` می‌خوانند. رجیستری سایت پیاده‌سازی هر شناسه را نگه می‌دارد و تست نگهبان برابری دقیق شناسه‌های کاتالوگ و رجیستری را بررسی می‌کند.
9. **پیش‌نمایش مستقل:** فقط `/preview` ورودی ظاهری اعتبارسنجی‌شده را روی محتوای عمومی همان Host اعمال می‌کند. این مسیر `private, no-store` و `noindex, nofollow` دارد و خانهٔ `/` با تنظیمات منتشرشده رندر می‌شود. [قرارداد پیش‌نمایش](../psychology/modules/ui-templates.md)

---

## ۳. ساختار استاندارد پوشه‌ها

```text
packages/ui/
├── package.json              # نام پکیج: "@psychology/ui"
├── tsconfig.json
└── src/
    ├── styles/
    │   └── base.css          # @import "tailwindcss"; @plugin "daisyui"; و توکن‌های پایه
    ├── tokens.ts             # تایپ‌ها و مقادیر مجاز Radius / Scale / Palette
    ├── template-catalog.ts   # منبع متادیتای مشترک پنل و سایت
    ├── primitives/           # Button, Input, Textarea, Card, Badge, Alert (قابل رندر SSR بدون JS)
    ├── interactive/          # Modal, Select, Tabs, Dropdown, PersianDatePicker (React + React Aria)
    └── index.ts

apps/psychology-site/src/templates/
├── contract.ts               # قرارداد TypeScript (اجباری برای همه قالب‌ها)
├── view-models.ts            # تایپ‌های آماده نمایش (SiteHeaderVM, HeroVM, CourseCardVM, ...)
├── registry.ts               # نگاشت مرکزی قالب‌ها + تابع resolveTemplate(id)
├── minimal/                  # قالب ۱: مینیمال و آرامش‌بخش (درمانگر فردی)
│   ├── theme.css             # تعریف [data-theme="minimal"] با متغیرهای DaisyUI
│   ├── Header.astro
│   ├── Footer.astro
│   ├── HomeHero.astro
│   ├── CourseCard.astro
│   ├── WorkshopCard.astro
│   ├── BlogCard.astro
│   └── index.ts
└── clinical/                 # قالب ۲: کلینیکال و مدرن (مراکز مشاوره)
    ├── theme.css
    ├── Header.astro
    ├── Footer.astro
    ├── HomeHero.astro
    ├── CourseCard.astro
    ├── WorkshopCard.astro
    ├── BlogCard.astro
    └── index.ts

```

---

## ۴. قرارداد TypeScript قالب (`contract.ts`)

هر قالب جدید **اجباراً** باید شیء زیر را export کند تا `tsc` و `astro check` کامل بودن آن را در زمان Build تضمین کنند:

`TemplateMeta` در کاتالوگ مشترک تعریف می‌شود و قرارداد سایت آن را مصرف می‌کند. مقدار `meta` هر قالب از ورودی همان شناسه در کاتالوگ می‌آید. `template_version` در دیتابیس نسخهٔ فعال هنگام ذخیره است. رندر فقط بر اساس شناسه و پیاده‌سازی فعال رجیستری انجام می‌شود.

```ts
// packages/ui/src/template-catalog.ts
export interface TemplateMeta {
  id: string;
  version: string;          // مثال: "1.0.0" (مطابق قانون ۱۲ AGENTS.md)
  labelFa: string;          // نام فارسی نمایشی در پنل ادمین
  descriptionFa: string;    // توضیح کوتاه برای روان‌شناس
  daisyTheme: string;       // نام [data-theme] در theme.css این قالب
}
```

```ts
// apps/psychology-site/src/templates/contract.ts
import type { TemplateMeta } from '@psychology/ui';
import type { AstroComponentFactory } from 'astro/runtime/server/index.js';

export interface SiteTemplate {
  meta: TemplateMeta;
  components: {
    Header: AstroComponentFactory;
    Footer: AstroComponentFactory;
    HomeHero: AstroComponentFactory;
    CourseCard: AstroComponentFactory;
    WorkshopCard: AstroComponentFactory;
    BlogCard: AstroComponentFactory;
  };
}

```

---

## ۵. چک‌لیست افزودن قالب جدید (برای قالب‌های ۳ تا ۱۰+)

برای اضافه کردن هر قالب جدید، فقط این ۴ قدم انجام می‌شود و هیچ فایلی از قالب‌های دیگر ویرایش نمی‌شود:

1. ساخت پوشه `apps/psychology-site/src/templates/<new-id>/` شامل `theme.css`، شش کامپوننت `.astro` و `index.ts` مطابق `SiteTemplate`.
2. ثبت متادیتا در `packages/ui/src/template-catalog.ts`، ایمپورت `theme.css` و ثبت کامپوننت‌ها در `apps/psychology-site/src/templates/registry.ts`. پنل گزینه را از همان کاتالوگ می‌خواند.
3. اجرای تست نگهبان برابری شناسه‌ها و بررسی ظاهر با `/preview?template=<new-id>` روی Host مطب در محیط توسعه.
4. اجرای `make verify-psychology` شامل `astro check` هر دو اپ و `pnpm lint`.
