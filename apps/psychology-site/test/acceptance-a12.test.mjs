import test from 'node:test';
import assert from 'node:assert';
import { spawn } from 'node:child_process';

test('PSY-A12 Acceptance Test: Comprehensive Regression, RTL, Mobile, Zero Client JS, Persian Calendar and Multi-Tenant Isolation', async () => {
  const port = 4394;
  const siteDir = new URL('..', import.meta.url).pathname;
  const child = spawn('node', ['./dist/server/entry.mjs'], {
    cwd: siteDir,
    env: { ...process.env, PORT: String(port), HOST: '127.0.0.1' },
    stdio: ['ignore', 'pipe', 'pipe'],
  });

  try {
    let stderrOutput = '';
    child.stderr.on('data', (d) => {
      stderrOutput += d.toString();
    });

    await new Promise((resolve, reject) => {
      child.stdout.on('data', (data) => {
        if (data.toString().includes('Server listening')) {
          resolve();
        }
      });
      child.on('error', reject);
      setTimeout(() => reject(new Error(`timeout waiting for server to start: ${stderrOutput}`)), 6000);
    });

    const hostA = 'rezaei-clinic.ir';
    const hostB = 'niloofar-psych.ir';

    // =========================================================================
    // 1. Host A Previewing minimal vs clinical before saving
    // =========================================================================
    const resPrevMinimal = await fetch(`http://127.0.0.1:${port}/preview?template=minimal`, {
      headers: { Host: hostA },
    });
    assert.strictEqual(resPrevMinimal.status, 200);
    assert.strictEqual(resPrevMinimal.headers.get('cache-control'), 'private, no-store');
    const htmlPrevMinimal = await resPrevMinimal.text();
    assert.match(htmlPrevMinimal, /data-theme="minimal"/);
    assert.match(htmlPrevMinimal, /<meta name="robots" content="noindex, nofollow"/);
    assert.match(htmlPrevMinimal, /حالت پیش‌نمایش ظاهر/);
    assert.match(htmlPrevMinimal, /dir="rtl"/);
    assert.match(htmlPrevMinimal, /<meta name="viewport" content="width=device-width/);

    const resPrevClinical = await fetch(
      `http://127.0.0.1:${port}/preview?template=clinical&primary=%230f766e&radius=lg&scale=sm`,
      { headers: { Host: hostA } },
    );
    assert.strictEqual(resPrevClinical.status, 200);
    assert.strictEqual(resPrevClinical.headers.get('cache-control'), 'private, no-store');
    const htmlPrevClinical = await resPrevClinical.text();
    assert.match(htmlPrevClinical, /data-theme="clinical"/);
    assert.match(htmlPrevClinical, /data-radius="lg"/);
    assert.match(htmlPrevClinical, /data-scale="sm"/);
    assert.match(htmlPrevClinical, /--color-primary:\s*#0f766e/);
    assert.match(htmlPrevClinical, /مرکز تخصصی روان‌شناسی و مشاوره/);

    // =========================================================================
    // 2. Fallback to minimal when unknown template ID is requested
    // =========================================================================
    const resPrevFallback = await fetch(`http://127.0.0.1:${port}/preview?template=unregistered_template_999`, {
      headers: { Host: hostA },
    });
    assert.strictEqual(resPrevFallback.status, 200);
    const htmlPrevFallback = await resPrevFallback.text();
    assert.match(htmlPrevFallback, /data-theme="minimal"/);
    assert.match(htmlPrevFallback, /قالب اختصاصی مینیمال/);

    // =========================================================================
    // 3. FOUC and CSS injection prevention: invalid hex color falls back safely to #0284c7
    // =========================================================================
    const resPrevXss = await fetch(`http://127.0.0.1:${port}/preview?primary=red;body{display:none}`, {
      headers: { Host: hostA },
    });
    assert.strictEqual(resPrevXss.status, 200);
    const htmlPrevXss = await resPrevXss.text();
    assert.doesNotMatch(htmlPrevXss, /body\{display:none\}/);
    assert.match(htmlPrevXss, /--color-primary:\s*#0284c7/);

    // =========================================================================
    // 4. Verification that Home page / strictly displays published settings and ignores query overrides
    // =========================================================================
    const resHomeA = await fetch(
      `http://127.0.0.1:${port}/?template=clinical&primary=%230f766e&radius=lg&scale=sm`,
      { headers: { Host: hostA } },
    );
    assert.strictEqual(resHomeA.status, 200);
    const homeCacheControl = resHomeA.headers.get('cache-control') || '';
    assert.match(homeCacheControl, /public/);
    assert.match(homeCacheControl, /s-maxage=3600/);
    const htmlHomeA = await resHomeA.text();
    // Must remain minimal, published primary color, and NOT have noindex
    assert.match(htmlHomeA, /data-theme="minimal"/);
    assert.doesNotMatch(htmlHomeA, /data-theme="clinical"/);
    assert.doesNotMatch(htmlHomeA, /<meta name="robots" content="noindex, nofollow"/);
    assert.match(htmlHomeA, /dir="rtl"/);
    assert.match(htmlHomeA, /lang="fa"/);
    assert.match(htmlHomeA, /<meta name="viewport" content="width=device-width/);

    // =========================================================================
    // 5. Host B isolation check: Host B public home page renders independently
    // =========================================================================
    const resHomeB = await fetch(`http://127.0.0.1:${port}/`, {
      headers: { Host: hostB },
    });
    assert.strictEqual(resHomeB.status, 200);
    const htmlHomeB = await resHomeB.text();
    assert.match(htmlHomeB, /data-theme="minimal"/);
    assert.match(htmlHomeB, /dir="rtl"/);

    // =========================================================================
    // 6. Content Pages Regression: Courses, Workshops, Blog (RTL, Viewport, 0 KB client JS)
    // =========================================================================
    const contentPages = [
      { path: '/courses', title: 'دوره‌های آموزشی تخصصی', zeroClientJs: true },
      { path: '/courses/anxiety-management', title: 'دوره جامع مدیریت استرس و اضطراب فراگیر', zeroClientJs: false },
      { path: '/workshops', title: 'کارگاه‌های تخصصی و آموزشی', zeroClientJs: true },
      { path: '/workshops/f47ac10b-58cc-4372-a567-0e02b2c3d479', title: 'کارگاه روان‌درمانی تحلیلی و تنظیم هیجان', zeroClientJs: false },
      { path: '/blog', title: 'مقالات و یادداشت‌های تخصصی', zeroClientJs: true },
      { path: '/blog/self-awareness-and-emotional-regulation', title: 'نقش خودآگاهی و تنظیم هیجان در روابط بین‌فردی', zeroClientJs: true },
    ];

    for (const page of contentPages) {
      const res = await fetch(`http://127.0.0.1:${port}${page.path}`, {
        headers: { Host: hostA },
      });
      assert.strictEqual(res.status, 200, `Page ${page.path} should return 200`);
      const cc = res.headers.get('cache-control') || '';
      assert.ok(cc.includes('public'), `Page ${page.path} must have public cache`);

      const html = await res.text();
      assert.match(html, /dir="rtl"/, `Page ${page.path} must be RTL`);
      assert.match(html, /lang="fa"/, `Page ${page.path} must have lang="fa"`);
      assert.match(html, /<meta name="viewport" content="width=device-width/, `Page ${page.path} must have responsive viewport`);
      assert.match(html, new RegExp(page.title), `Page ${page.path} must contain its title`);
      assert.match(html, /card bg-base-100/, `Page ${page.path} must use shared DaisyUI cards`);

      // Ensure no hardcoded hex color inline styles remain in content
      assert.doesNotMatch(html, /color:\s*#0f172a/, `Page ${page.path} should not have hardcoded color`);
      assert.doesNotMatch(html, /background:\s*#ffffff/, `Page ${page.path} should not have hardcoded white background`);

      // 0 KB Client JS verification for pure content pages
      if (page.zeroClientJs) {
        assert.strictEqual(
          html.includes('<script type="module"'),
          false,
          `Pure content page ${page.path} must have 0 KB client JavaScript (no module scripts)`
        );
      }
    }

    // =========================================================================
    // 7. Interactive Pages & Booking Flow (PersianDatePicker Island & Jalali Calendar)
    // =========================================================================
    const resBooking = await fetch(`http://127.0.0.1:${port}/booking`, {
      headers: { Host: hostA },
    });
    assert.strictEqual(resBooking.status, 200);
    const htmlBooking = await resBooking.text();
    assert.match(htmlBooking, /dir="rtl"/);
    assert.match(htmlBooking, /<meta name="viewport" content="width=device-width/);
    assert.match(htmlBooking, /درخواست نوبت مشاوره و روان‌درمانی/);
    assert.match(htmlBooking, /انتخاب درمانگر/);
    assert.match(htmlBooking, /دکتر مریم علوی/);

    // Verify PersianDatePicker Island is properly rendered and hydrated
    assert.ok(htmlBooking.includes('astro-island'), 'Booking page must render PersianDatePicker as an Astro island');
    assert.match(htmlBooking, /client="visible"/, 'PersianDatePicker must have client:visible hydration directive');
    assert.match(htmlBooking, /booking_preferred_date/);
    // Jalali date output check
    assert.ok(htmlBooking.includes('۱۴۰۵') || htmlBooking.includes('1405'), 'Persian Jalali year must be present in calendar markup');

    // =========================================================================
    // 8. Strict Private Caching Verification (Account & Private Appointments)
    // =========================================================================
    const privatePages = [
      { path: '/account/courses', matchText: 'دوره‌های من' },
      { path: '/account/courses/c-demo-1', matchText: 'پخش جلسات دوره' },
      { path: '/appointments/appt-demo-1?lookup_hash=hash-test-123', matchText: 'وضعیت نوبت مشاوره' },
      { path: '/workshops/registrations/reg-test-123', matchText: 'وضعیت ثبت‌نام کارگاه' },
    ];

    for (const priv of privatePages) {
      const res = await fetch(`http://127.0.0.1:${port}${priv.path}`, {
        headers: { Host: hostA },
      });
      assert.strictEqual(res.status, 200, `Private page ${priv.path} should return 200`);
      const cc = res.headers.get('cache-control') || '';
      assert.ok(cc.includes('private'), `Private page ${priv.path} must have private cache-control`);
      assert.ok(cc.includes('no-store'), `Private page ${priv.path} must have no-store cache-control`);

      const html = await res.text();
      assert.match(html, /dir="rtl"/, `Private page ${priv.path} must be RTL`);
      assert.match(html, /<meta name="viewport" content="width=device-width/, `Private page ${priv.path} must have responsive viewport`);
      assert.match(html, new RegExp(priv.matchText), `Private page ${priv.path} must contain expected content`);
      assert.doesNotMatch(html, /color:\s*#0f172a/);
    }
  } finally {
    child.kill('SIGTERM');
  }
});
