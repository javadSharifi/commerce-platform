import { bookingFixture } from './booking-fixture.mjs';
import { fetchOnHost } from './http-host.mjs';
import test from 'node:test';
import assert from 'node:assert';
import http from 'node:http';
import { spawn } from 'node:child_process';

test('PSY-A13 Acceptance Test: Multi-Tenant Appearance Selection, Preview, Publishing, Mobile App 390px and Strict Isolation (Shared Cache OFF)', async () => {
  const sitePort = 4450;
  const mockApiPort = 4451;

  const hostA = 'omid-psych.ir';
  const hostB = 'mahan-clinic.ir';

  // 1. Mock API server with initial published settings for two distinct tenants
  let siteA = {
    name: 'کلینیک تخصصی روان‌شناسی امید',
    title: 'کلینیک تخصصی روان‌شناسی امید',
    description: 'روان‌درمانی فردی و زوج‌درمانی تخصصی',
    phone: '۰۲۱-۸۸۸۸۱۱۱۱',
    address: 'تهران، ونک',
    palette_id: 'calm-sage',
    template_id: 'minimal',
    template_version: '1.0.0',
    theme_radius: 'md',
    theme_scale: 'md',
  };

  const siteB = {
    name: 'مرکز مشاوره ماهان',
    title: 'مرکز مشاوره ماهان',
    description: 'مشاوره کودک و خانواده',
    phone: '۰۲۱-۲۲۲۲۳۳۳۳',
    address: 'تهران، شریعتی',
    palette_id: 'warm-sand',
    template_id: 'minimal',
    template_version: '1.0.0',
    theme_radius: 'md',
    theme_scale: 'md',
  };

  const courses = [
    {
      id: 'c-1',
      title: 'دوره جامع مدیریت استرس و اضطراب',
      slug: 'anxiety-management',
      price_rials: 15000000,
      description: 'توضیحات دورهٔ مطب امید',
      instructor_name: 'مدرس دوره',
      lessons: [],
      status: 'published',
      published_at: new Date().toISOString(),
    },
  ];

  const workshops = [
    {
      id: 'ws-1',
      title: 'کارگاه تنظیم هیجان',
      price: 5000000,
      capacity: 20,
      status: 'active',
    },
  ];

  const blogPosts = [
    {
      id: 'b-1',
      title: 'نقش خودآگاهی در روابط فردی',
      slug: 'self-awareness',
      status: 'published',
      published_at: new Date().toISOString(),
    },
  ];

  const mockApiServer = http.createServer((req, res) => {
    const rawHost = req.headers['x-forwarded-host'] || req.headers['host'] || '';
    const cleanHost = String(rawHost).split(':')[0].trim();
    const url = new URL(req.url, `http://${rawHost}`);
    const pathname = url.pathname;

    res.setHeader('Content-Type', 'application/json; charset=utf-8');
    if (bookingFixture(req, res, cleanHost !== hostA)) return;

    if (pathname === '/v1/public/site') {
      if (cleanHost === hostA) {
        res.writeHead(200);
        res.end(JSON.stringify(siteA));
        return;
      }
      if (cleanHost === hostB) {
        res.writeHead(200);
        res.end(JSON.stringify(siteB));
        return;
      }
      res.writeHead(404);
      res.end(JSON.stringify({ code: 'DOMAIN_NOT_FOUND', message: 'دامنه یافت نشد' }));
      return;
    }

    if (pathname === '/v1/public/courses') {
      res.writeHead(200);
      res.end(JSON.stringify(courses));
      return;
    }

    if (pathname === '/v1/public/courses/anxiety-management' && cleanHost === hostA) {
      res.writeHead(200);
      res.end(JSON.stringify(courses[0]));
      return;
    }

    if (pathname === '/v1/public/workshops') {
      res.writeHead(200);
      res.end(JSON.stringify(workshops));
      return;
    }

    if (pathname === '/v1/public/blog') {
      res.writeHead(200);
      res.end(JSON.stringify(blogPosts));
      return;
    }

    if (pathname === '/v1/public/booking') {
      res.writeHead(201);
      res.end(JSON.stringify({ id: 'appt-test-123', status: 'pending' }));
      return;
    }

    res.writeHead(404);
    res.end(JSON.stringify({ code: 'NOT_FOUND' }));
  });

  await new Promise((resolve) => mockApiServer.listen(mockApiPort, '127.0.0.1', resolve));

  // 2. Spawn Astro SSR child process with shared HTML cache OFF (direct SSR execution)
  const siteDir = new URL('..', import.meta.url).pathname;
  const child = spawn('node', ['./dist/server/entry.mjs'], {
    cwd: siteDir,
    env: {
      ...process.env,
      PORT: String(sitePort),
      HOST: '127.0.0.1',
      INTERNAL_API_URL: `http://127.0.0.1:${mockApiPort}`,
    },
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

    // =========================================================================
    // Step 1: Initial Published Appearance and Strict Tenant Isolation
    // =========================================================================
    // Host A Home (/)
    const resHomeA = await fetchOnHost(`http://127.0.0.1:${sitePort}/`, {
      headers: { Host: hostA, 'X-Forwarded-Host': hostA },
    });
    assert.strictEqual(resHomeA.status, 200);
    const htmlHomeA = await resHomeA.text();
    assert.match(htmlHomeA, /data-theme="minimal"/);
    assert.match(htmlHomeA, /data-palette="calm-sage"/);
    assert.match(htmlHomeA, /data-radius="md"/);
    assert.match(htmlHomeA, /data-scale="md"/);
    assert.ok(htmlHomeA.includes(siteA.title), 'Host A must render its title');
    assert.ok(!htmlHomeA.includes(siteB.title), 'Host A must NOT leak Host B title');

    // Host B Home (/)
    const resHomeB = await fetchOnHost(`http://127.0.0.1:${sitePort}/`, {
      headers: { Host: hostB, 'X-Forwarded-Host': hostB },
    });
    assert.strictEqual(resHomeB.status, 200);
    const htmlHomeB = await resHomeB.text();
    assert.match(htmlHomeB, /data-theme="minimal"/);
    assert.match(htmlHomeB, /data-palette="warm-sand"/);
    assert.ok(htmlHomeB.includes(siteB.title), 'Host B must render its title');
    assert.ok(!htmlHomeB.includes(siteA.title), 'Host B must NOT leak Host A title');

    // =========================================================================
    // Step 2: Preview in Form: Fixed Template / Two Palettes, Fixed Palette / Two Templates
    // =========================================================================
    // 2.a Fixed template (clinical) with 2 palettes: calm-sage vs ocean-trust
    const resPrevPal1 = await fetchOnHost(
      `http://127.0.0.1:${sitePort}/preview?template=clinical&palette=calm-sage&radius=lg&scale=sm`,
      { headers: { Host: hostA, 'X-Forwarded-Host': hostA } }
    );
    assert.strictEqual(resPrevPal1.status, 200);
    assert.strictEqual(resPrevPal1.headers.get('cache-control'), 'private, no-store');
    const htmlPrevPal1 = await resPrevPal1.text();
    assert.match(htmlPrevPal1, /data-theme="clinical"/);
    assert.match(htmlPrevPal1, /data-palette="calm-sage"/);
    assert.match(htmlPrevPal1, /data-radius="lg"/);
    assert.match(htmlPrevPal1, /data-scale="sm"/);
    assert.match(htmlPrevPal1, /<meta name="robots" content="noindex, nofollow"/);

    const resPrevPal2 = await fetchOnHost(
      `http://127.0.0.1:${sitePort}/preview?template=clinical&palette=ocean-trust&radius=lg&scale=sm`,
      { headers: { Host: hostA, 'X-Forwarded-Host': hostA } }
    );
    assert.strictEqual(resPrevPal2.status, 200);
    assert.strictEqual(resPrevPal2.headers.get('cache-control'), 'private, no-store');
    const htmlPrevPal2 = await resPrevPal2.text();
    assert.match(htmlPrevPal2, /data-theme="clinical"/);
    assert.match(htmlPrevPal2, /data-palette="ocean-trust"/);

    // 2.b Fixed palette (ocean-trust) with 2 templates: minimal vs mobile-app
    const resPrevTpl1 = await fetchOnHost(
      `http://127.0.0.1:${sitePort}/preview?template=minimal&palette=ocean-trust&radius=md&scale=md`,
      { headers: { Host: hostA, 'X-Forwarded-Host': hostA } }
    );
    assert.strictEqual(resPrevTpl1.status, 200);
    const htmlPrevTpl1 = await resPrevTpl1.text();
    assert.match(htmlPrevTpl1, /data-theme="minimal"/);
    assert.match(htmlPrevTpl1, /data-palette="ocean-trust"/);

    const resPrevTpl2 = await fetchOnHost(
      `http://127.0.0.1:${sitePort}/preview?template=mobile-app&palette=ocean-trust&radius=md&scale=md`,
      { headers: { Host: hostA, 'X-Forwarded-Host': hostA } }
    );
    assert.strictEqual(resPrevTpl2.status, 200);
    const htmlPrevTpl2 = await resPrevTpl2.text();
    assert.match(htmlPrevTpl2, /data-theme="mobile-app"/);
    assert.match(htmlPrevTpl2, /data-palette="ocean-trust"/);
    assert.match(htmlPrevTpl2, /mobile-bottom-nav/);

    // 2.c Sitemap verification: /preview must NOT be present in /sitemap.xml
    const resSitemap = await fetchOnHost(`http://127.0.0.1:${sitePort}/sitemap.xml`, {
      headers: { Host: hostA, 'X-Forwarded-Host': hostA },
    });
    assert.strictEqual(resSitemap.status, 200);
    const sitemapXml = await resSitemap.text();
    assert.ok(!sitemapXml.includes('/preview'), 'sitemap.xml must exclude /preview');
    assert.ok(sitemapXml.includes('/courses'), 'sitemap.xml must include /courses');

    // 2.d Verify Home page ignores preview parameters before publishing
    const resHomeWithQuery = await fetchOnHost(
      `http://127.0.0.1:${sitePort}/?template=clinical&palette=ocean-trust&radius=lg&scale=sm`,
      { headers: { Host: hostA, 'X-Forwarded-Host': hostA } }
    );
    assert.strictEqual(resHomeWithQuery.status, 200);
    const htmlHomeWithQuery = await resHomeWithQuery.text();
    // Home MUST ignore preview parameters and strictly render published minimal and calm-sage
    assert.match(htmlHomeWithQuery, /data-theme="minimal"/);
    assert.match(htmlHomeWithQuery, /data-palette="calm-sage"/);

    // =========================================================================
    // Step 3: Appearance Publishing on Host A (Transition to mobile-app & royal-plum)
    // =========================================================================
    // Simulate Host A publishing appearance
    siteA = {
      ...siteA,
      template_id: 'mobile-app',
      palette_id: 'royal-plum',
      theme_radius: 'lg',
      theme_scale: 'lg',
    };

    // 3.a Host A Home reflects newly published mobile-app and royal-plum
    const resPublishedHomeA = await fetchOnHost(`http://127.0.0.1:${sitePort}/`, {
      headers: { Host: hostA, 'X-Forwarded-Host': hostA },
    });
    assert.strictEqual(resPublishedHomeA.status, 200);
    const htmlPublishedHomeA = await resPublishedHomeA.text();
    assert.match(htmlPublishedHomeA, /data-theme="mobile-app"/);
    assert.match(htmlPublishedHomeA, /data-palette="royal-plum"/);
    assert.match(htmlPublishedHomeA, /data-radius="lg"/);
    assert.match(htmlPublishedHomeA, /data-scale="lg"/);

    // 3.b Host A Detail Page (/courses/anxiety-management) reflects published appearance
    const resPublishedCourseA = await fetchOnHost(`http://127.0.0.1:${sitePort}/courses/anxiety-management`, {
      headers: { Host: hostA, 'X-Forwarded-Host': hostA },
    });
    assert.strictEqual(resPublishedCourseA.status, 200);
    const htmlPublishedCourseA = await resPublishedCourseA.text();
    assert.match(htmlPublishedCourseA, /data-theme="mobile-app"/);
    assert.match(htmlPublishedCourseA, /data-palette="royal-plum"/);

    // 3.c Host A Booking Page (/booking) reflects published appearance
    const resPublishedBookingA = await fetchOnHost(`http://127.0.0.1:${sitePort}/booking`, {
      headers: { Host: hostA, 'X-Forwarded-Host': hostA },
    });
    assert.strictEqual(resPublishedBookingA.status, 200);
    const htmlPublishedBookingA = await resPublishedBookingA.text();
    assert.match(htmlPublishedBookingA, /data-theme="mobile-app"/);
    assert.match(htmlPublishedBookingA, /data-palette="royal-plum"/);

    // 3.d Host B remains completely unchanged (strict multi-tenant isolation)
    const resHostBPostPublish = await fetchOnHost(`http://127.0.0.1:${sitePort}/`, {
      headers: { Host: hostB, 'X-Forwarded-Host': hostB },
    });
    assert.strictEqual(resHostBPostPublish.status, 200);
    const htmlHostBPostPublish = await resHostBPostPublish.text();
    assert.match(htmlHostBPostPublish, /data-theme="minimal"/);
    assert.match(htmlHostBPostPublish, /data-palette="warm-sand"/);
    assert.ok(htmlHostBPostPublish.includes(siteB.title));
    assert.ok(!htmlHostBPostPublish.includes(siteA.title));

    // =========================================================================
    // Step 4: Fallback & Security for Unknown Identifiers
    // =========================================================================
    const resFallbackTpl = await fetchOnHost(`http://127.0.0.1:${sitePort}/preview?template=unregistered_template_xyz`, {
      headers: { Host: hostA, 'X-Forwarded-Host': hostA },
    });
    assert.strictEqual(resFallbackTpl.status, 200);
    const htmlFallbackTpl = await resFallbackTpl.text();
    assert.match(htmlFallbackTpl, /data-theme="minimal"/);

    const resFallbackPal = await fetchOnHost(`http://127.0.0.1:${sitePort}/preview?palette=unregistered_palette_xyz`, {
      headers: { Host: hostA, 'X-Forwarded-Host': hostA },
    });
    assert.strictEqual(resFallbackPal.status, 200);
    const htmlFallbackPal = await resFallbackPal.text();
    assert.match(htmlFallbackPal, /data-palette="calm-sage"/);

    // =========================================================================
    // Step 5: Mobile App Template & 390px Mobile Navigation
    // =========================================================================
    // Verify all 6 destinations in mobile-app template:
    // 4 primary destinations on bottom bar: خانه, دوره‌ها, کارگاه‌ها, نوبت
    assert.match(htmlPublishedHomeA, /href="\/"[^>]*aria-label="خانه"/);
    assert.match(htmlPublishedHomeA, /href="\/courses"[^>]*aria-label="دوره‌ها"/);
    assert.match(htmlPublishedHomeA, /href="\/workshops"[^>]*aria-label="کارگاه‌ها"/);
    assert.match(htmlPublishedHomeA, /href="\/booking"[^>]*aria-label="درخواست نوبت"/);

    // 2 secondary destinations in "بیشتر" dropdown: وبلاگ, حساب کاربری
    assert.match(htmlPublishedHomeA, /href="\/blog"/);
    assert.match(htmlPublishedHomeA, /وبلاگ و مقالات/);
    assert.match(htmlPublishedHomeA, /href="\/account\/courses"/);
    assert.match(htmlPublishedHomeA, /حساب کاربری/);

    // Safe layout & viewport verification:
    // Bottom bar has fixed positioning and safe-area padding
    assert.match(htmlPublishedHomeA, /fixed bottom-0/);
    assert.match(htmlPublishedHomeA, /pb-\[env\(safe-area-inset-bottom\)\]/);
    // Root container has scroll-padding-bottom to avoid obscuring booking forms/buttons
    assert.match(htmlPublishedHomeA, /scroll-padding-bottom:\s*5rem/);
    assert.match(htmlPublishedHomeA, /<meta name="viewport" content="width=device-width/);
    assert.match(htmlPublishedHomeA, /dir="rtl"/);

    // =========================================================================
    // Step 6: Booking Form on Mobile App (Persian Calendar & ISO Date Submission)
    // =========================================================================
    // Jalali date numerals in rendered HTML segments
    assert.ok(htmlPublishedBookingA.includes('۱۴۰۵') || htmlPublishedBookingA.includes('1405'));
    assert.match(htmlPublishedBookingA, /booking_preferred_date/);
    assert.match(htmlPublishedBookingA, /astro-island/);
    assert.match(htmlPublishedBookingA, /client="visible"/);

    // Verify DaisyUI tokens in booking form
    assert.match(htmlPublishedBookingA, /card bg-base-100/);
    assert.match(htmlPublishedBookingA, /btn-primary/);
    assert.match(htmlPublishedBookingA, /input-bordered/);

    // Verify standard ISO Gregorian YYYY-MM-DD initial date value in picker
    assert.match(htmlPublishedBookingA, /value="\d{4}-\d{2}-\d{2}"/, 'DatePicker value must be standard Gregorian ISO YYYY-MM-DD');

    // Verify 390px mobile responsiveness and RTL
    assert.match(htmlPublishedBookingA, /overflow-x-hidden/);
    assert.match(htmlPublishedBookingA, /dir="rtl"/);

    // =========================================================================
    // Step 7: Strict Private Caching Verification (Account & Private Appointments)
    // =========================================================================
    const privatePages = [
      { path: '/account/courses', check: /دوره‌های من/ },
      { path: '/account/courses/c-demo-1', check: /پخش جلسات دوره/ },
      { path: '/appointments/appt-demo-1?lookup_hash=hash-test-123', check: /وضعیت نوبت مشاوره/ },
      { path: '/workshops/registrations/reg-test-123', check: /وضعیت ثبت‌نام کارگاه/ },
    ];

    for (const priv of privatePages) {
      const res = await fetchOnHost(`http://127.0.0.1:${sitePort}${priv.path}`, {
        headers: { Host: hostA, 'X-Forwarded-Host': hostA },
      });
      assert.strictEqual(res.status, 200, `Private page ${priv.path} should return 200`);
      const cc = res.headers.get('cache-control') || '';
      assert.ok(cc.includes('private'), `Private page ${priv.path} must have private cache-control`);
      assert.ok(cc.includes('no-store'), `Private page ${priv.path} must have no-store cache-control`);

      const html = await res.text();
      assert.match(html, /dir="rtl"/);
      assert.match(html, priv.check);
    }
  } finally {
    child.kill('SIGTERM');
    await new Promise((resolve) => mockApiServer.close(resolve));
  }
});
