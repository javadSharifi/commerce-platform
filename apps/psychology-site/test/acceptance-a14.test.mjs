import { bookingFixture, fixtureTime } from './booking-fixture.mjs';
import { fetchOnHost } from './http-host.mjs';
import test from 'node:test';
import assert from 'node:assert';
import http from 'node:http';
import { spawn } from 'node:child_process';

const siteDir = new URL('..', import.meta.url).pathname;

test('PSY-A14 Acceptance Test: Editorial Redesign, Multi-Tenant Appearance Selection, Preview, Publishing, Content Zero JS, Booking Flow and Strict Two-Host Isolation', async () => {
  const sitePort = 4490;
  const mockApiPort = 4491;

  const hostA = 'sharifi-clinic.ir';
  const hostB = 'moradi-center.ir';

  // 1. Initial State for two distinct tenants
  let siteA = {
    name: 'کلینیک تخصصی روان‌شناسی شریفی',
    title: 'کلینیک تخصصی روان‌شناسی شریفی',
    description: 'روان‌درمانی فردی و زوج‌درمانی تخصصی با رویکرد علمی',
    phone: '۰۲۱-۲۲۸۴۹۱۰۳',
    address: 'تهران، خیابان شریعتی، بالاتر از ظفر',
    primary_color: '#2D4F43',
    palette_id: 'calm-sage',
    template_id: 'minimal',
    template_version: '1.0.0',
    theme_radius: 'md',
    theme_scale: 'md',
  };

  const siteB = {
    name: 'مرکز مشاوره و روان‌شناسی مرادی',
    title: 'مرکز مشاوره و روان‌شناسی مرادی',
    description: 'مرکز جامع سلامت روان و خدمات روان‌شناختی',
    phone: '۰۲۱-۸۸۸۸۴۴۴۴',
    address: 'تهران، سعادت‌آباد',
    primary_color: '#0284c7',
    palette_id: 'warm-sand',
    template_id: 'clinical',
    template_version: '1.0.0',
    theme_radius: 'lg',
    theme_scale: 'sm',
  };

  const coursesA = [
    {
      id: 'c-sharifi-1',
      title: 'دوره جامع مدیریت استرس و اضطراب',
      slug: 'stress-anxiety-management',
      description: 'راهنمای گام‌به‌گام برای شناسایی و کاهش اضطراب و استرس با تکنیک‌های بالینی.',
      instructor_name: 'دکتر محمد جواد شریفی',
      price_rials: 12500000,
      status: 'published',
      lessons_count: 2,
      cover_url: null,
      lessons: [
        {
          id: 'l-s-1',
          title: 'جلسه اول: مکانیسم‌های بیولوژیک استرس',
          description: 'مبانی نظری اضطراب و فعال‌سازی پاراسمپاتیک',
          display_order: 1,
          duration_seconds: 1800,
          is_free_preview: true,
        },
        {
          id: 'l-s-2',
          title: 'جلسه دوم: بازسازی شناختی و تکنیک‌های CBT',
          description: 'شناسایی و تصحیح باورهای ناکارآمد',
          display_order: 2,
          duration_seconds: 2400,
          is_free_preview: false,
        },
      ],
    },
    {
      id: 'c-sharifi-2',
      title: 'دوره مهارت‌های ارتباط موثر',
      slug: 'effective-communication',
      description: 'آموزش اصول گفتگوی سازنده و همدلی فعال در روابط عاطفی و شغلی.',
      instructor_name: 'دکتر محمد جواد شریفی',
      price_rials: 9800000,
      status: 'published',
      lessons_count: 1,
      cover_url: null,
      lessons: [
        {
          id: 'l-s-3',
          title: 'جلسه اول: مهارت گوش دادن فعال',
          description: 'اصول گوش سپردن همدلانه بدون قضاوت',
          display_order: 1,
          duration_seconds: 1500,
          is_free_preview: true,
        },
      ],
    },
  ];

  const workshopsA = [
    {
      id: 'ws-sharifi-1',
      title: 'کارگاه روان‌درمانی تحلیلی و تنظیم هیجان',
      description: 'شناسایی و تنظیم هیجانات دشوار، کار با تله‌های شخصیتی و تجربه خودآگاهی عمیق.',
      instructor_name: 'دکتر محمد جواد شریفی',
      capacity: 15,
      enrolled_count: 8,
      status: 'active',
      price_rials: 35000000,
      held_at: '2026-12-05T09:00:00Z',
    },
  ];

  const blogPostsA = [
    {
      id: 'b-sharifi-1',
      title: 'چگونه اضطراب پنهان را در زندگی روزمره شناسایی کنیم؟',
      slug: 'identifying-hidden-anxiety',
      excerpt: 'بررسی نشانه‌های فیزیولوژیک و روان‌شناختی اضطراب‌های کنترل‌نشده و روش‌های مقابله با آن‌ها.',
      status: 'published',
      published_at: new Date().toISOString(),
      reading_time_minutes: 6,
      cover_url: null,
      content_blocks: [
        { type: 'paragraph', data: { text: 'اضطراب همیشه با تپش قلب شدید یا حملات پانیک همراه نیست؛ گاهی به شکل اهمال‌کاری مداوم ظاهر می‌شود.' } },
        { type: 'heading', data: { text: 'علائم شایع اضطراب پنهان', level: 2 } },
        { type: 'paragraph', data: { text: 'خستگی مزمن بدون دلیل پزشکی، نیاز افراطی به کنترل همه‌چیز و اختلال در خواب از نشانه‌های آن است.' } },
      ],
    },
    {
      id: 'b-draft-2',
      title: 'پیش‌نویس منتشرنشده درباره طرحواره‌ها',
      slug: 'draft-schema-post',
      excerpt: 'این پست نباید در سایت رندر شود چون پیش‌نویس است.',
      status: 'draft',
      published_at: null,
    },
  ];

  const therapistsA = [
    {
      id: 'th-sharifi-01',
      name: 'دکتر محمد جواد شریفی',
      slug: 'dr-mohammad-sharifi',
      title: 'روان‌شناس بالینی و روان‌درمانگر تحلیلی',
      bio: 'بیش از ۱۲ سال سابقه بالینی در درمان اختلالات فردی و روابط بین‌فردی.',
      session_duration_minutes: 50,
    },
  ];

  const slotsA = [
    {
      id: 'slot-sharifi-101',
      therapist_id: 'th-sharifi-01',
      starts_at: fixtureTime(0),
      ends_at: fixtureTime(50),
      status: 'available',
    },
    {
      id: 'slot-sharifi-102',
      therapist_id: 'th-sharifi-01',
      starts_at: fixtureTime(60),
      ends_at: fixtureTime(110),
      status: 'booked',
    },
  ];

  // Mock API Server
  const mockApiServer = http.createServer((req, res) => {
    const rawHost = req.headers['x-forwarded-host'] || req.headers['host'] || '';
    const cleanHost = String(rawHost).split(':')[0].trim();
    const url = new URL(req.url, `http://${rawHost}`);
    const pathname = url.pathname;

    res.setHeader('Content-Type', 'application/json; charset=utf-8');

    // 1. Public Site
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
      res.end(JSON.stringify({ code: 'NOT_FOUND' }));
      return;
    }

    // 2. Admin Appearance Update
    if (pathname === '/v1/admin/site/appearance' && req.method === 'PUT') {
      let bodyStr = '';
      req.on('data', (chunk) => {
        bodyStr += chunk;
      });
      req.on('end', () => {
        try {
          const body = JSON.parse(bodyStr);
          if (cleanHost === hostA) {
            siteA = {
              ...siteA,
              template_id: body.template_id || siteA.template_id,
              palette_id: body.palette_id || siteA.palette_id,
              template_version: '1.0.0',
            };
            res.writeHead(200);
            res.end(JSON.stringify(siteA));
            return;
          }
          res.writeHead(400);
          res.end(JSON.stringify({ code: 'BAD_REQUEST' }));
        } catch {
          res.writeHead(400);
          res.end(JSON.stringify({ code: 'BAD_REQUEST' }));
        }
      });
      return;
    }

    // 3. Courses
    if (pathname === '/v1/public/courses') {
      if (cleanHost === hostA) {
        res.writeHead(200);
        res.end(JSON.stringify(coursesA));
        return;
      }
      res.writeHead(200);
      res.end(JSON.stringify([]));
      return;
    }

    if (pathname.startsWith('/v1/public/courses/')) {
      const slug = pathname.replace('/v1/public/courses/', '');
      const found = coursesA.find((c) => c.slug === slug || c.id === slug);
      if (found) {
        res.writeHead(200);
        res.end(JSON.stringify(found));
        return;
      }
      res.writeHead(404);
      res.end(JSON.stringify({ code: 'COURSE_NOT_FOUND' }));
      return;
    }

    // 4. Workshops
    if (pathname === '/v1/public/workshops') {
      if (cleanHost === hostA) {
        res.writeHead(200);
        res.end(JSON.stringify(workshopsA));
        return;
      }
      res.writeHead(200);
      res.end(JSON.stringify([]));
      return;
    }

    if (pathname.startsWith('/v1/public/workshops/')) {
      const id = pathname.replace('/v1/public/workshops/', '');
      const found = workshopsA.find((w) => w.id === id);
      if (found) {
        res.writeHead(200);
        res.end(JSON.stringify(found));
        return;
      }
      res.writeHead(404);
      res.end(JSON.stringify({ code: 'WORKSHOP_NOT_FOUND' }));
      return;
    }

    // 5. Blog
    if (pathname === '/v1/public/blog') {
      if (cleanHost === hostA) {
        // Return only published articles
        const publishedOnly = blogPostsA.filter((p) => p.status === 'published');
        res.writeHead(200);
        res.end(JSON.stringify(publishedOnly));
        return;
      }
      res.writeHead(200);
      res.end(JSON.stringify([]));
      return;
    }

    if (pathname.startsWith('/v1/public/blog/')) {
      const slug = pathname.replace('/v1/public/blog/', '');
      const found = blogPostsA.find((p) => (p.slug === slug || p.id === slug) && p.status === 'published');
      if (found) {
        res.writeHead(200);
        res.end(JSON.stringify(found));
        return;
      }
      res.writeHead(404);
      res.end(JSON.stringify({ code: 'POST_NOT_FOUND' }));
      return;
    }

    // 6. Therapists & Slots
    if (pathname.startsWith('/v1/public/appointments/') && bookingFixture(req, res, cleanHost !== hostA)) return;

    if (pathname === '/v1/public/therapists') {
      if (cleanHost === hostA) {
        res.writeHead(200);
        res.end(JSON.stringify(therapistsA));
        return;
      }
      res.writeHead(200);
      res.end(JSON.stringify([]));
      return;
    }

    if (pathname.includes('/therapists/') && pathname.includes('/slots')) {
      res.writeHead(200);
      res.end(JSON.stringify(slotsA));
      return;
    }

    // 7. Appointments Booking
    if (pathname === '/v1/public/appointments' && req.method === 'POST') {
      let bodyStr = '';
      req.on('data', (chunk) => {
        bodyStr += chunk;
      });
      req.on('end', () => {
        const body = JSON.parse(bodyStr || '{}');
        if (body.slot_id === 'slot-sharifi-102') {
          res.writeHead(409);
          res.end(JSON.stringify({ code: 'CONFLICT', message: 'Slot already taken' }));
          return;
        }
        res.writeHead(201);
        res.end(
          JSON.stringify({
            id: 'appt-a14-success',
            guest_lookup_hash: 'hash-a14-secret',
            status: 'pending_confirmation',
          })
        );
      });
      return;
    }

    res.writeHead(404);
    res.end(JSON.stringify({ code: 'NOT_FOUND' }));
  });

  await new Promise((resolve) => mockApiServer.listen(mockApiPort, '127.0.0.1', resolve));

  const child = spawn('node', ['./dist/server/entry.mjs'], {
    cwd: siteDir,
    env: {
      ...process.env,
      PORT: String(sitePort),
      HOST: '127.0.0.1',
      INTERNAL_API_URL: `http://127.0.0.1:${mockApiPort}`,
      PUBLIC_API_URL: `http://127.0.0.1:${mockApiPort}`,
    },
    stdio: ['ignore', 'pipe', 'pipe'],
  });

  try {
    child.stderr.on('data', (d) => process.stderr.write(`[child stderr] ${d}`));
    await new Promise((resolve, reject) => {
      child.stdout.on('data', (data) => {
        if (data.toString().includes('Server listening')) {
          resolve();
        }
      });
      child.on('error', reject);
      child.on('exit', (code) => {
        if (code !== 0 && code !== null) {
          console.error(`child exited with code ${code}`);
        }
      });
      setTimeout(() => reject(new Error('timeout waiting for site server to start')), 6000);
    });

    // =========================================================================
    // SCENARIO 1: Preview on Host A, then Publish Editorial and verify
    // =========================================================================
    const reqA = { Host: hostA, 'X-Forwarded-Host': hostA };
    const reqB = { Host: hostB, 'X-Forwarded-Host': hostB };

    // 1a. Preview Host A with template=editorial & palette=calm-sage
    const resPreviewA = await fetchOnHost(
      `http://127.0.0.1:${sitePort}/preview?template=editorial&palette=calm-sage`,
      { headers: reqA }
    );
    assert.strictEqual(resPreviewA.status, 200, 'Preview must return 200 OK');
    const ccPreview = resPreviewA.headers.get('cache-control') || '';
    assert.ok(ccPreview.includes('no-store'), 'Preview must have no-store');
    assert.ok(ccPreview.includes('private'), 'Preview must have private');

    const htmlPreviewA = await resPreviewA.text();
    assert.match(htmlPreviewA, /<meta name="robots" content="noindex, nofollow"/, 'Preview must have noindex, nofollow');
    assert.match(htmlPreviewA, /data-theme="editorial"/, 'Preview must render editorial theme');
    assert.match(htmlPreviewA, /data-palette="calm-sage"/, 'Preview must render calm-sage palette');

    // 1b. Verify Host A root / before publishing is still minimal
    const resRootBeforeA = await fetchOnHost(`http://127.0.0.1:${sitePort}/`, {
      headers: reqA,
    });
    assert.strictEqual(resRootBeforeA.status, 200);
    const htmlRootBeforeA = await resRootBeforeA.text();
    assert.match(htmlRootBeforeA, /data-theme="minimal"/, 'Host A root must remain minimal prior to publishing');

    // 1c. Publish "editorial" template with "calm-sage" on Host A via Mock API
    const putResA = await fetchOnHost(`http://127.0.0.1:${mockApiPort}/v1/admin/site/appearance`, {
      method: 'PUT',
      headers: {
        'Content-Type': 'application/json',
        Host: hostA,
        'X-Forwarded-Host': hostA,
      },
      body: JSON.stringify({
        template_id: 'editorial',
        palette_id: 'calm-sage',
      }),
    });
    assert.strictEqual(putResA.status, 200, 'Appearance publishing must succeed');

    // 1d. Now fetch Host A root /: must render published editorial template
    const resRootAfterA = await fetchOnHost(`http://127.0.0.1:${sitePort}/`, {
      headers: reqA,
    });
    assert.strictEqual(resRootAfterA.status, 200);
    const htmlRootAfterA = await resRootAfterA.text();
    assert.match(htmlRootAfterA, /data-theme="editorial"/, 'Published appearance must be editorial');
    assert.match(htmlRootAfterA, /data-palette="calm-sage"/, 'Published palette must be calm-sage');
    assert.match(htmlRootAfterA, /کلینیک تخصصی روان‌شناسی شریفی/, 'Must display Clinic A title');
    assert.doesNotMatch(htmlRootAfterA, /مرکز مشاوره و روان‌شناسی مرادی/, 'Zero tenant leak: Clinic B name must not appear in Clinic A');

    // 1e. Verify Host B root / remains clinical and warm-sand (Isolation)
    const resRootB = await fetchOnHost(`http://127.0.0.1:${sitePort}/`, {
      headers: reqB,
    });
    assert.strictEqual(resRootB.status, 200);
    const htmlRootB = await resRootB.text();
    assert.match(htmlRootB, /data-theme="clinical"/, 'Host B must remain clinical');
    assert.match(htmlRootB, /data-palette="warm-sand"/, 'Host B must remain warm-sand');
    assert.match(htmlRootB, /مرکز مشاوره و روان‌شناسی مرادی/, 'Host B must display Clinic B title');
    assert.doesNotMatch(htmlRootB, /کلینیک تخصصی روان‌شناسی شریفی/, 'Zero tenant leak: Clinic A name must not appear in Clinic B');

    // =========================================================================
    // SCENARIO 2: One Template with Two Palettes & One Palette with Two Templates
    // =========================================================================
    const resCalmSage = await fetchOnHost(
      `http://127.0.0.1:${sitePort}/preview?template=editorial&palette=calm-sage`,
      { headers: reqA }
    );
    const htmlCalmSage = await resCalmSage.text();
    assert.match(htmlCalmSage, /data-theme="editorial"/);
    assert.match(htmlCalmSage, /data-palette="calm-sage"/);

    const resOceanTrust = await fetchOnHost(
      `http://127.0.0.1:${sitePort}/preview?template=editorial&palette=ocean-trust`,
      { headers: reqA }
    );
    const htmlOceanTrust = await resOceanTrust.text();
    assert.match(htmlOceanTrust, /data-theme="editorial"/);
    assert.match(htmlOceanTrust, /data-palette="ocean-trust"/);

    // =========================================================================
    // SCENARIO 3: Path from Home to Courses, Workshops, and Blog
    // =========================================================================

    // 3a. /courses listing on Host A
    const resCoursesA = await fetchOnHost(`http://127.0.0.1:${sitePort}/courses`, {
      headers: reqA,
    });
    assert.strictEqual(resCoursesA.status, 200);
    const htmlCoursesA = await resCoursesA.text();
    assert.match(htmlCoursesA, /data-theme="editorial"/, 'Courses page must inherit editorial template');
    assert.match(htmlCoursesA, /دوره جامع مدیریت استرس و اضطراب/, 'Must list Course 1');
    assert.match(htmlCoursesA, /href="\/courses\/stress-anxiety-management"/, 'CTA must link to specific course slug');
    // Zero client JS verification on content page
    assert.doesNotMatch(htmlCoursesA, /<script[^>]*astro-island/, 'Content page must have 0 KB client JS');

    // 3b. /courses/[slug] detail
    const resCourseDetailA = await fetchOnHost(`http://127.0.0.1:${sitePort}/courses/stress-anxiety-management`, {
      headers: reqA,
    });
    assert.strictEqual(resCourseDetailA.status, 200);
    const htmlCourseDetailA = await resCourseDetailA.text();
    assert.match(htmlCourseDetailA, /راهنمای گام‌به‌گام برای شناسایی و کاهش اضطراب/, 'Must render course description');
    assert.match(htmlCourseDetailA, /دکتر محمد جواد شریفی/, 'Must render instructor name');

    // 3c. Invalid course slug returns 404
    const resCourse404 = await fetchOnHost(`http://127.0.0.1:${sitePort}/courses/nonexistent-slug-xyz`, {
      headers: reqA,
    });
    assert.strictEqual(resCourse404.status, 404, 'Invalid course slug must return 404');

    // 3d. /workshops listing on Host A
    const resWorkshopsA = await fetchOnHost(`http://127.0.0.1:${sitePort}/workshops`, {
      headers: reqA,
    });
    assert.strictEqual(resWorkshopsA.status, 200);
    const htmlWorkshopsA = await resWorkshopsA.text();
    assert.match(htmlWorkshopsA, /کارگاه روان‌درمانی تحلیلی و تنظیم هیجان/);
    assert.match(htmlWorkshopsA, /href="\/workshops\/ws-sharifi-1"/, 'CTA must link to specific workshop ID');

    // 3e. /blog listing and blog draft isolation
    const resBlogA = await fetchOnHost(`http://127.0.0.1:${sitePort}/blog`, {
      headers: reqA,
    });
    assert.strictEqual(resBlogA.status, 200);
    const htmlBlogA = await resBlogA.text();
    assert.match(htmlBlogA, /چگونه اضطراب پنهان را در زندگی روزمره شناسایی کنیم؟/, 'Must list published article');
    assert.doesNotMatch(htmlBlogA, /پیش‌نویس منتشرنشده/, 'Draft articles must never be rendered in public blog');

    // 3f. /blog/[slug] article detail
    const resArticleA = await fetchOnHost(`http://127.0.0.1:${sitePort}/blog/identifying-hidden-anxiety`, {
      headers: reqA,
    });
    assert.strictEqual(resArticleA.status, 200);
    const htmlArticleA = await resArticleA.text();
    assert.match(htmlArticleA, /علائم شایع اضطراب پنهان/, 'Must render whitelisted heading block');
    assert.match(htmlArticleA, /خستگی مزمن بدون دلیل پزشکی/, 'Must render whitelisted paragraph block');

    // 3g. Invalid blog slug returns 404
    const resBlog404 = await fetchOnHost(`http://127.0.0.1:${sitePort}/blog/nonexistent-blog-slug-404`, {
      headers: reqA,
    });
    assert.strictEqual(resBlog404.status, 404, 'Invalid blog slug must return 404');

    // =========================================================================
    // SCENARIO 4: Booking Flow on Host A with Editorial Template
    // =========================================================================
    const resBookingA = await fetchOnHost(`http://127.0.0.1:${sitePort}/booking`, {
      headers: reqA,
    });
    assert.strictEqual(resBookingA.status, 200);
    const htmlBookingA = await resBookingA.text();

    assert.match(htmlBookingA, /data-theme="editorial"/, 'Booking page must inherit editorial theme');
    assert.match(htmlBookingA, /رزرو وقت مشاوره • محرمانه و مطمئن/, 'Booking page must display trust badge');
    assert.match(htmlBookingA, /اصول رازداری و هماهنگی/, 'Sidebar must display confidentiality card');
    assert.match(htmlBookingA, /slot-sharifi-101/, 'Available slot must be present');
    assert.match(htmlBookingA, /slot-sharifi-102/, 'Booked slot must be present');
    assert.match(htmlBookingA, /slot-btn-unavailable/, 'Booked slot must have unavailable style');
    assert.match(htmlBookingA, /تکمیل‌شده/, 'Booked slot must display (تکمیل‌شده) label');

    // =========================================================================
    // SCENARIO 5: RTL Navigation, Accessibility & 390px Mobile Responsiveness
    // =========================================================================
    assert.match(htmlBookingA, /dir="rtl"/, 'Must render in RTL');
    assert.match(htmlBookingA, /overflow-x-hidden/, 'Must prevent horizontal overflow');
    assert.match(htmlBookingA, /grid-cols-1/, 'Must support 1-column layout on mobile viewports');
    assert.match(htmlBookingA, /focus-visible:ring-2/, 'Interactive elements must include focus rings');
    assert.match(htmlBookingA, /focus-visible:ring-primary/, 'Focus rings must use primary token');

    // Verify single H1 on each redesigned page
    const countH1 = (html) => (html.match(/<h1[^>]*>/g) || []).length;
    assert.strictEqual(countH1(htmlRootAfterA), 1, 'Home must have exactly one H1');
    assert.strictEqual(countH1(htmlCoursesA), 1, 'Courses listing must have exactly one H1');
    assert.strictEqual(countH1(htmlWorkshopsA), 1, 'Workshops listing must have exactly one H1');
    assert.strictEqual(countH1(htmlBlogA), 1, 'Blog listing must have exactly one H1');
    assert.strictEqual(countH1(htmlArticleA), 1, 'Article detail must have exactly one H1');
    assert.strictEqual(countH1(htmlBookingA), 1, 'Booking must have exactly one H1');

    // =========================================================================
    // SCENARIO 6: Private Account & Appointment Routes Strict no-store Verification
    // =========================================================================
    const checkPrivateNoStore = async (urlPath) => {
      const res = await fetchOnHost(`http://127.0.0.1:${sitePort}${urlPath}`, {
        headers: reqA,
      });
      assert.strictEqual(res.status, 200, `${urlPath} must return 200 OK`);
      const cc = res.headers.get('cache-control') || '';
      assert.ok(cc.includes('no-store'), `${urlPath} must include no-store in Cache-Control, got: ${cc}`);
      assert.ok(cc.includes('private'), `${urlPath} must include private in Cache-Control, got: ${cc}`);
    };

    await checkPrivateNoStore('/account/courses');
    await checkPrivateNoStore('/account/courses/c-demo-1');
    await checkPrivateNoStore('/appointments/appt-demo-1?hash=test-token-hash');
    await checkPrivateNoStore('/workshops/registrations/reg-demo-1?lookup_token=test-lookup');
  } finally {
    child.kill('SIGTERM');
    await new Promise((resolve) => mockApiServer.close(resolve));
  }
});
