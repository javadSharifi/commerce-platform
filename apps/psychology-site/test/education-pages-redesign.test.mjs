import test from 'node:test';
import assert from 'node:assert';
import { spawn } from 'node:child_process';

const siteDir = new URL('..', import.meta.url).pathname;

test('PSY-7.3: Education routes redesign (/courses, /courses/[slug], /workshops, /workshops/[id]), CTA routing and edge-cases (empty, full, archived)', async () => {
  const port = 4470;
  const child = spawn('node', ['./dist/server/entry.mjs'], {
    cwd: siteDir,
    env: { ...process.env, PORT: String(port), HOST: '127.0.0.1' },
    stdio: ['ignore', 'pipe', 'pipe'],
  });

  try {
    await new Promise((resolve, reject) => {
      child.stdout.on('data', (data) => {
        if (data.toString().includes('Server listening')) {
          resolve();
        }
      });
      child.on('error', reject);
      setTimeout(() => reject(new Error('timeout waiting for server to start')), 8000);
    });

    const host = 'rezaei-clinic.ir';

    // ---------------------------------------------------------------------------------
    // 1. /courses listing
    // ---------------------------------------------------------------------------------
    const resCourses = await fetch(`http://127.0.0.1:${port}/courses`, {
      headers: { 'X-Forwarded-Host': host, Host: host },
    });
    assert.strictEqual(resCourses.status, 200);
    assert.ok(resCourses.headers.get('cache-control')?.includes('public'));
    const htmlCourses = await resCourses.text();

    assert.match(htmlCourses, /آموزش‌های تخصصی روانشناسی/);
    assert.match(htmlCourses, /<h1[^>]*>[\s\S]*?دوره‌های آموزشی تخصصی[\s\S]*?<\/h1>/);
    assert.match(htmlCourses, /همه دوره‌ها/);
    assert.match(htmlCourses, /دوره جامع مدیریت استرس و اضطراب فراگیر/);
    // Every CTA in the courses list points to its matching /courses/[slug]
    assert.match(htmlCourses, /href="\/courses\/anxiety-management"/);
    assert.match(htmlCourses, /href="\/courses\/communication-skills"/);
    // Cross-link to workshops present
    assert.match(htmlCourses, /href="\/workshops"/);
    // Semantic tokens used, no hardcoded colors
    assert.match(htmlCourses, /card bg-base-100/);
    assert.doesNotMatch(htmlCourses, /color:\s*#0f172a/);
    assert.doesNotMatch(htmlCourses, /background:\s*#ffffff/);

    // ---------------------------------------------------------------------------------
    // 2. /courses empty state regression
    // ---------------------------------------------------------------------------------
    const resCoursesEmpty = await fetch(`http://127.0.0.1:${port}/courses?empty=1`, {
      headers: { 'X-Forwarded-Host': host, Host: host },
    });
    assert.strictEqual(resCoursesEmpty.status, 200);
    const htmlCoursesEmpty = await resCoursesEmpty.text();
    assert.match(htmlCoursesEmpty, /در حال حاضر دوره‌ای منتشر نشده است/);
    assert.match(htmlCoursesEmpty, /به زودی دوره‌های آموزشی جدید در این بخش قرار خواهند گرفت/);
    assert.match(htmlCoursesEmpty, /href="\/workshops"/);
    assert.doesNotMatch(htmlCoursesEmpty, /anxiety-management/);

    // ---------------------------------------------------------------------------------
    // 3. /courses/[slug] active detail
    // ---------------------------------------------------------------------------------
    const resCourseDetail = await fetch(`http://127.0.0.1:${port}/courses/anxiety-management`, {
      headers: { 'X-Forwarded-Host': host, Host: host },
    });
    assert.strictEqual(resCourseDetail.status, 200);
    assert.ok(resCourseDetail.headers.get('cache-control')?.includes('public'));
    const htmlCourseDetail = await resCourseDetail.text();

    // Breadcrumb / back link
    assert.match(htmlCourseDetail, /href="\/courses"/);
    assert.match(htmlCourseDetail, /بازگشت به لیست دوره‌ها/);
    // Title and instructor
    assert.match(htmlCourseDetail, /<h1[^>]*>[\s\S]*?دوره جامع مدیریت استرس و اضطراب فراگیر[\s\S]*?<\/h1>/);
    assert.match(htmlCourseDetail, /دکتر مریم علوی/);
    // Metrics strip
    assert.match(htmlCourseDetail, /تعداد جلسات/);
    assert.match(htmlCourseDetail, /مدت کل دوره/);
    assert.match(htmlCourseDetail, /مدت دسترسی/);
    // Learning outcomes
    assert.match(htmlCourseDetail, /در این دوره چه مهارت‌هایی می‌آموزید؟/);
    // Curriculum details/summary accordion
    assert.match(htmlCourseDetail, /سرفصل‌ها و جلسات دوره/);
    assert.match(htmlCourseDetail, /<details/);
    assert.match(htmlCourseDetail, /<summary/);
    assert.match(htmlCourseDetail, /جلسه اول: آشنایی با مکانیسم زیستی استرس/);
    assert.match(htmlCourseDetail, /پیش‌نمایش رایگان/);
    // Instructor bio & reviews
    assert.match(htmlCourseDetail, /مدرس و هدایت‌کننده دوره/);
    assert.match(htmlCourseDetail, /تجربه و دیدگاه شرکت‌کنندگان/);
    // Active checkout root and inputs
    assert.match(htmlCourseDetail, /id="checkout-root"/);
    assert.match(htmlCourseDetail, /id="buyer-phone-input"/);
    assert.match(htmlCourseDetail, /id="btn-start-checkout"/);
    assert.match(htmlCourseDetail, /تومان/);
    assert.doesNotMatch(htmlCourseDetail, /دوره در وضعیت آرشیو است/);

    // ---------------------------------------------------------------------------------
    // 4. /courses/[slug] archived course state regression
    // ---------------------------------------------------------------------------------
    const resCourseArchived = await fetch(`http://127.0.0.1:${port}/courses/anxiety-management?status=archived`, {
      headers: { 'X-Forwarded-Host': host, Host: host },
    });
    assert.strictEqual(resCourseArchived.status, 200);
    const htmlCourseArchived = await resCourseArchived.text();

    assert.match(htmlCourseArchived, /دورهٔ آرشیوشده/);
    assert.match(htmlCourseArchived, /این دوره به پایان رسیده و هم‌اکنون در وضعیت آرشیو قرار دارد/);
    assert.match(htmlCourseArchived, /ثبت‌نام جدید در این دوره امکان‌پذیر نیست/);
    assert.match(htmlCourseArchived, /btn-disabled/);
    assert.match(htmlCourseArchived, /دوره در وضعیت آرشیو است/);
    // Active checkout root should NOT be active for new purchases
    assert.doesNotMatch(htmlCourseArchived, /id="checkout-root"/);
    assert.doesNotMatch(htmlCourseArchived, /id="buyer-phone-input"/);

    // ---------------------------------------------------------------------------------
    // 5. /workshops listing
    // ---------------------------------------------------------------------------------
    const resWorkshops = await fetch(`http://127.0.0.1:${port}/workshops`, {
      headers: { 'X-Forwarded-Host': host, Host: host },
    });
    assert.strictEqual(resWorkshops.status, 200);
    assert.ok(resWorkshops.headers.get('cache-control')?.includes('public'));
    const htmlWorkshops = await resWorkshops.text();

    assert.match(htmlWorkshops, /آموزش‌های تعاملی و گروهی/);
    assert.match(htmlWorkshops, /<h1[^>]*>[\s\S]*?کارگاه‌های تخصصی و آموزشی[\s\S]*?<\/h1>/);
    assert.match(htmlWorkshops, /همه کارگاه‌ها/);
    assert.match(htmlWorkshops, /کارگاه روان‌درمانی تحلیلی و تنظیم هیجان/);
    // Every CTA in the workshops list points to its matching /workshops/[id]
    assert.match(htmlWorkshops, /href="\/workshops\/f47ac10b-58cc-4372-a567-0e02b2c3d479"/);
    assert.match(htmlWorkshops, /href="\/workshops\/a12bc34d-58cc-4372-a567-0e02b2c3d480"/);
    // Cross-link to courses present
    assert.match(htmlWorkshops, /href="\/courses"/);
    // Semantic tokens used, no hardcoded colors
    assert.match(htmlWorkshops, /card bg-base-100/);
    assert.doesNotMatch(htmlWorkshops, /color:\s*#0f172a/);
    assert.doesNotMatch(htmlWorkshops, /background:\s*#ffffff/);

    // ---------------------------------------------------------------------------------
    // 6. /workshops empty state regression
    // ---------------------------------------------------------------------------------
    const resWorkshopsEmpty = await fetch(`http://127.0.0.1:${port}/workshops?empty=1`, {
      headers: { 'X-Forwarded-Host': host, Host: host },
    });
    assert.strictEqual(resWorkshopsEmpty.status, 200);
    const htmlWorkshopsEmpty = await resWorkshopsEmpty.text();
    assert.match(htmlWorkshopsEmpty, /در حال حاضر کارگاهی برای ثبت‌نام فعال نیست/);
    assert.match(htmlWorkshopsEmpty, /به زودی برنامه‌ریزی کارگاه‌های حضوری و آنلاین جدید اعلام خواهد شد/);
    assert.match(htmlWorkshopsEmpty, /href="\/courses"/);
    assert.doesNotMatch(htmlWorkshopsEmpty, /f47ac10b-58cc-4372-a567-0e02b2c3d479/);

    // ---------------------------------------------------------------------------------
    // 7. /workshops/[id] active detail
    // ---------------------------------------------------------------------------------
    const resWorkshopDetail = await fetch(
      `http://127.0.0.1:${port}/workshops/f47ac10b-58cc-4372-a567-0e02b2c3d479`,
      { headers: { 'X-Forwarded-Host': host, Host: host } }
    );
    assert.strictEqual(resWorkshopDetail.status, 200);
    assert.ok(resWorkshopDetail.headers.get('cache-control')?.includes('public'));
    const htmlWorkshopDetail = await resWorkshopDetail.text();

    // Breadcrumb navigation
    assert.match(htmlWorkshopDetail, /href="\/workshops"/);
    assert.match(htmlWorkshopDetail, /کارگاه‌ها/);
    // Badge and title
    assert.match(htmlWorkshopDetail, /badge-success/);
    assert.match(htmlWorkshopDetail, /ثبت‌نام فعال/);
    assert.match(htmlWorkshopDetail, /<h1[^>]*>[\s\S]*?کارگاه روان‌درمانی تحلیلی و تنظیم هیجان[\s\S]*?<\/h1>/);
    // Key details strip
    assert.match(htmlWorkshopDetail, /مدرس کارگاه/);
    assert.match(htmlWorkshopDetail, /زمان برگزاری/);
    assert.match(htmlWorkshopDetail, /محل برگزاری/);
    assert.match(htmlWorkshopDetail, /ظرفیت باقیمانده/);
    // Active registration form and inputs
    assert.match(htmlWorkshopDetail, /فرم ثبت‌نام آنلاین در کارگاه/);
    assert.match(htmlWorkshopDetail, /id="register-form"/);
    assert.match(htmlWorkshopDetail, /id="guest_name"/);
    assert.match(htmlWorkshopDetail, /id="guest_phone"/);
    assert.match(htmlWorkshopDetail, /id="submit-btn"/);
    assert.doesNotMatch(htmlWorkshopDetail, /تکمیل شده \(۰ صندلی\)/);

    // ---------------------------------------------------------------------------------
    // 8. /workshops/[id] sold-out workshop regression
    // ---------------------------------------------------------------------------------
    const resWorkshopSoldOut = await fetch(
      `http://127.0.0.1:${port}/workshops/f47ac10b-58cc-4372-a567-0e02b2c3d479?sold_out=1`,
      { headers: { 'X-Forwarded-Host': host, Host: host } }
    );
    assert.strictEqual(resWorkshopSoldOut.status, 200);
    const htmlWorkshopSoldOut = await resWorkshopSoldOut.text();

    assert.match(htmlWorkshopSoldOut, /badge-error/);
    assert.match(htmlWorkshopSoldOut, /ظرفیت تکمیل است/);
    assert.match(htmlWorkshopSoldOut, /تکمیل شده \(۰ صندلی\)/);
    assert.match(htmlWorkshopSoldOut, /متأسفانه ظرفیت این کارگاه تکمیل شده است/);
    assert.match(htmlWorkshopSoldOut, /btn-disabled/);
    assert.match(htmlWorkshopSoldOut, /تکمیل ظرفیت/);
    // Active form inputs should NOT be rendered when sold out
    assert.doesNotMatch(htmlWorkshopSoldOut, /id="register-form"/);
    assert.doesNotMatch(htmlWorkshopSoldOut, /id="guest_name"/);

    // ---------------------------------------------------------------------------------
    // 9. Regression check for zero hardcoded hex styles across all 4 pages
    // ---------------------------------------------------------------------------------
    for (const html of [htmlCourses, htmlCourseDetail, htmlWorkshops, htmlWorkshopDetail]) {
      assert.doesNotMatch(html, /color:\s*#0f172a/);
      assert.doesNotMatch(html, /background:\s*#ffffff/);
    }
  } finally {
    child.kill('SIGTERM');
  }
});
