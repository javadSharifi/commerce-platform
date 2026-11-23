import { bookingFixture } from './booking-fixture.mjs';
import { blogFixture } from './blog-fixture.mjs';
import { educationFixture } from './education-fixture.mjs';
import { fetchOnHost } from './http-host.mjs';
import test from 'node:test';
import assert from 'node:assert';
import http from 'node:http';
import { spawn } from 'node:child_process';

test('PSY-A12 Acceptance Test: Comprehensive Regression, RTL, Mobile, Zero Client JS, Persian Calendar and Multi-Tenant Isolation', async () => {
  const sitePort = 4394;
  const mockApiPort = 4398;
  const cacheProxyPort = 4397;

  const hostA = 'rezaei-clinic.ir';
  const hostB = 'niloofar-psych.ir';

  // 1. Setup mock API for multi-tenant settings
  let siteA = {
    name: 'مطب تخصصی دکتر علی رضایی',
    title: 'مطب تخصصی دکتر علی رضایی',
    description: 'خدمات روان‌درمانی و مشاوره فردی تخصصی',
    phone: '۰۲۱-۸۸۸۸۱۱۱۱',
    address: 'تهران، میدان ونک',
    palette_id: 'calm-sage',
    template_id: 'minimal',
    template_version: '1.0.0',
    theme_radius: 'md',
    theme_scale: 'md',
  };

  const siteB = {
    name: 'مرکز مشاوره نیلوفر',
    title: 'مرکز مشاوره و سلامت روان نیلوفر',
    description: 'روان‌درمانی و مشاوره‌های تخصصی خانواده',
    phone: '۰۲۱-۸۸۸۸۲۲۲۲',
    address: 'تهران، خیابان شریعتی',
    palette_id: 'warm-sand',
    template_id: 'minimal',
    template_version: '1.0.0',
    theme_radius: 'md',
    theme_scale: 'md',
  };

  const mockApiServer = http.createServer((req, res) => {
    const rawHost = req.headers['x-forwarded-host'] || req.headers['host'] || '';
    const cleanHost = String(rawHost).split(':')[0].trim();
    res.setHeader('Content-Type', 'application/json; charset=utf-8');
    if (bookingFixture(req, res, cleanHost !== hostA)) return;

    if (cleanHost === hostA || cleanHost === hostB) {
      const education = educationFixture(req.url, { empty: cleanHost === hostB });
      if (education !== undefined) return res.end(JSON.stringify(education));
      const blog = blogFixture(req.url, { empty: cleanHost === hostB, withMedia: false });
      if (blog !== undefined) return res.end(JSON.stringify(blog));
    }

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
    res.end(JSON.stringify({ code: 'HOST_NOT_FOUND', message: 'دامنه یافت نشد' }));
  });

  await new Promise((resolve) => mockApiServer.listen(mockApiPort, '127.0.0.1', resolve));

  // 2. Setup Edge Cache Proxy simulating reverse-proxy edge cache
  const cacheMap = new Map();
  const cacheProxyServer = http.createServer(async (req, res) => {
    const rawHost = req.headers['x-forwarded-host'] || req.headers['host'] || '';
    const cleanHost = String(rawHost).split(':')[0].trim();
    const url = req.url || '/';
    const path = url.split('?')[0];
    const key = `${cleanHost}${path}`;

    const isBypass = url.includes('?') ||
      path.startsWith('/preview') ||
      path.startsWith('/admin') ||
      path.startsWith('/account') ||
      path.startsWith('/appointments');

    if (!isBypass && cacheMap.has(key)) {
      const entry = cacheMap.get(key);
      res.writeHead(entry.status, {
        ...entry.headers,
        'X-Cache': 'HIT',
      });
      res.end(entry.body);
      return;
    }

    try {
      const upstreamRes = await fetchOnHost(`http://127.0.0.1:${sitePort}${url}`, {
        method: req.method,
        headers: {
          ...req.headers,
          host: cleanHost,
          'x-forwarded-host': cleanHost,
        },
      });

      const body = await upstreamRes.text();
      const resHeaders = {};
      for (const [k, v] of upstreamRes.headers.entries()) {
        resHeaders[k] = v;
      }

      const cc = (resHeaders['cache-control'] || '').toLowerCase();
      const isNoStore = cc.includes('no-store') || cc.includes('private');

      if (!isBypass && upstreamRes.status === 200 && !isNoStore) {
        cacheMap.set(key, {
          status: upstreamRes.status,
          headers: resHeaders,
          body,
        });
        res.writeHead(upstreamRes.status, {
          ...resHeaders,
          'X-Cache': 'MISS',
        });
      } else {
        res.writeHead(upstreamRes.status, {
          ...resHeaders,
          'X-Cache': isBypass ? 'BYPASS' : 'MISS',
        });
      }
      res.end(body);
    } catch (err) {
      res.writeHead(502, { 'Content-Type': 'text/plain' });
      res.end(`Bad Gateway: ${err.message}`);
    }
  });

  const purgeHostCache = (purgeHost) => {
    for (const key of cacheMap.keys()) {
      if (key.startsWith(purgeHost)) {
        cacheMap.delete(key);
      }
    }
  };

  await new Promise((resolve) => cacheProxyServer.listen(cacheProxyPort, '127.0.0.1', resolve));

  // 3. Spawn Astro SSR child process
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

  let stderrOutput = '';
  child.stderr.on('data', (d) => {
    stderrOutput += d.toString();
  });

  try {

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
    // Step 1: Initial Cache Warmup for both hosts on Edge Cache Proxy
    // =========================================================================
    // Host A first request: MISS -> cached as minimal
    const resHomeA1 = await fetchOnHost(`http://127.0.0.1:${cacheProxyPort}/`, {
      headers: { Host: hostA, 'X-Forwarded-Host': hostA },
    });
    assert.strictEqual(resHomeA1.status, 200);
    assert.strictEqual(resHomeA1.headers.get('x-cache'), 'MISS');
    const htmlHomeA1 = await resHomeA1.text();
    assert.match(htmlHomeA1, /data-theme="minimal"/);
    assert.match(htmlHomeA1, /dir="rtl"/);

    // Host A second request: HIT
    const resHomeA2 = await fetchOnHost(`http://127.0.0.1:${cacheProxyPort}/`, {
      headers: { Host: hostA, 'X-Forwarded-Host': hostA },
    });
    assert.strictEqual(resHomeA2.status, 200);
    assert.strictEqual(resHomeA2.headers.get('x-cache'), 'HIT');
    const htmlHomeA2 = await resHomeA2.text();
    assert.match(htmlHomeA2, /data-theme="minimal"/);

    // Host B first request: MISS -> cached as minimal
    const resHomeB1 = await fetchOnHost(`http://127.0.0.1:${cacheProxyPort}/`, {
      headers: { Host: hostB, 'X-Forwarded-Host': hostB },
    });
    assert.strictEqual(resHomeB1.status, 200);
    assert.strictEqual(resHomeB1.headers.get('x-cache'), 'MISS');
    const htmlHomeB1 = await resHomeB1.text();
    assert.match(htmlHomeB1, /data-theme="minimal"/);

    // Host B second request: HIT
    const resHomeB2 = await fetchOnHost(`http://127.0.0.1:${cacheProxyPort}/`, {
      headers: { Host: hostB, 'X-Forwarded-Host': hostB },
    });
    assert.strictEqual(resHomeB2.status, 200);
    assert.strictEqual(resHomeB2.headers.get('x-cache'), 'HIT');
    const htmlHomeB2 = await resHomeB2.text();
    assert.match(htmlHomeB2, /data-theme="minimal"/);

    // =========================================================================
    // Step 2: Preview Verification (Two consecutive /preview requests must strictly BYPASS cache)
    // =========================================================================
    const previewUrl = `http://127.0.0.1:${cacheProxyPort}/preview?template=clinical&palette=ocean-trust&radius=lg&scale=sm`;

    // Consecutive request 1:
    const resPrev1 = await fetchOnHost(previewUrl, {
      headers: { Host: hostA, 'X-Forwarded-Host': hostA },
    });
    assert.strictEqual(resPrev1.status, 200);
    assert.strictEqual(resPrev1.headers.get('x-cache'), 'BYPASS');
    assert.strictEqual(resPrev1.headers.get('cache-control'), 'private, no-store');
    const htmlPrev1 = await resPrev1.text();
    assert.match(htmlPrev1, /data-theme="clinical"/);
    assert.match(htmlPrev1, /data-palette="ocean-trust"/);
    assert.match(htmlPrev1, /data-radius="lg"/);
    assert.match(htmlPrev1, /data-scale="sm"/);
    assert.match(htmlPrev1, /<meta name="robots" content="noindex, nofollow"/);
    assert.match(htmlPrev1, /حالت پیش‌نمایش ظاهر/);
    assert.match(htmlPrev1, /dir="rtl"/);

    // Consecutive request 2 (must also strictly BYPASS shared cache and carry private, no-store):
    const resPrev2 = await fetchOnHost(previewUrl, {
      headers: { Host: hostA, 'X-Forwarded-Host': hostA },
    });
    assert.strictEqual(resPrev2.status, 200);
    assert.strictEqual(resPrev2.headers.get('x-cache'), 'BYPASS');
    assert.strictEqual(resPrev2.headers.get('cache-control'), 'private, no-store');
    const htmlPrev2 = await resPrev2.text();
    assert.match(htmlPrev2, /data-theme="clinical"/);
    assert.match(htmlPrev2, /data-radius="lg"/);

    // Verify preview did NOT pollute or overwrite cached Home page /
    const resHomeCheck = await fetchOnHost(`http://127.0.0.1:${cacheProxyPort}/`, {
      headers: { Host: hostA, 'X-Forwarded-Host': hostA },
    });
    assert.strictEqual(resHomeCheck.headers.get('x-cache'), 'HIT');
    const htmlHomeCheck = await resHomeCheck.text();
    assert.match(htmlHomeCheck, /data-theme="minimal"/);

    // =========================================================================
    // Step 3: Fallback & Security on Preview
    // =========================================================================
    // Fallback to minimal when unknown template ID is requested
    const resPrevFallback = await fetchOnHost(`http://127.0.0.1:${sitePort}/preview?template=unregistered_template_999`, {
      headers: { Host: hostA, 'X-Forwarded-Host': hostA },
    });
    assert.strictEqual(resPrevFallback.status, 200);
    const htmlPrevFallback = await resPrevFallback.text();
    assert.match(htmlPrevFallback, /data-theme="minimal"/);
    assert.match(htmlPrevFallback, /قالب اختصاصی مینیمال/);

    // Fallback to calm-sage when invalid palette ID is requested
    const resPrevInvalidPal = await fetchOnHost(`http://127.0.0.1:${sitePort}/preview?palette=unregistered_palette_xyz`, {
      headers: { Host: hostA, 'X-Forwarded-Host': hostA },
    });
    assert.strictEqual(resPrevInvalidPal.status, 200);
    const htmlPrevInvalidPal = await resPrevInvalidPal.text();
    assert.match(htmlPrevInvalidPal, /data-palette="calm-sage"/);

    // =========================================================================
    // Step 4: Appearance Save and Outbox Processing Simulation
    // =========================================================================
    // Owner A saves appearance changes (template: clinical, palette: ocean-trust, radius: lg, scale: sm)
    siteA = {
      ...siteA,
      template_id: 'clinical',
      palette_id: 'ocean-trust',
      theme_radius: 'lg',
      theme_scale: 'sm',
    };

    // Outbox worker processes event: purges Host A cache only
    purgeHostCache(hostA);

    // =========================================================================
    // Step 5: Post-Outbox Verification (Host A renders fresh appearance, Host B remains HIT)
    // =========================================================================
    // Host A: First request after outbox is MISS, rendering fresh clinical appearance immediately
    const resHomeAPostPurge1 = await fetchOnHost(`http://127.0.0.1:${cacheProxyPort}/`, {
      headers: { Host: hostA, 'X-Forwarded-Host': hostA },
    });
    assert.strictEqual(resHomeAPostPurge1.status, 200);
    assert.strictEqual(resHomeAPostPurge1.headers.get('x-cache'), 'MISS');
    const htmlHomeAPostPurge1 = await resHomeAPostPurge1.text();
    assert.match(htmlHomeAPostPurge1, /data-theme="clinical"/);
    assert.match(htmlHomeAPostPurge1, /data-palette="ocean-trust"/);
    assert.match(htmlHomeAPostPurge1, /data-radius="lg"/);
    assert.match(htmlHomeAPostPurge1, /data-scale="sm"/);

    // Host A: Second request is re-cached as HIT
    const resHomeAPostPurge2 = await fetchOnHost(`http://127.0.0.1:${cacheProxyPort}/`, {
      headers: { Host: hostA, 'X-Forwarded-Host': hostA },
    });
    assert.strictEqual(resHomeAPostPurge2.status, 200);
    assert.strictEqual(resHomeAPostPurge2.headers.get('x-cache'), 'HIT');
    const htmlHomeAPostPurge2 = await resHomeAPostPurge2.text();
    assert.match(htmlHomeAPostPurge2, /data-theme="clinical"/);

    // Host B: Cache MUST remain strictly HIT with previous minimal appearance and warm-sand palette (zero invalidation leak)
    const resHomeBPostPurge = await fetchOnHost(`http://127.0.0.1:${cacheProxyPort}/`, {
      headers: { Host: hostB, 'X-Forwarded-Host': hostB },
    });
    assert.strictEqual(resHomeBPostPurge.status, 200);
    assert.strictEqual(resHomeBPostPurge.headers.get('x-cache'), 'HIT');
    const htmlHomeBPostPurge = await resHomeBPostPurge.text();
    assert.match(htmlHomeBPostPurge, /data-theme="minimal"/);
    assert.match(htmlHomeBPostPurge, /data-palette="warm-sand"/);

    // =========================================================================
    // Step 6: Content Pages Regression: Courses, Workshops, Blog (RTL, Viewport, 0 KB client JS)
    // =========================================================================
    const contentPages = [
      { path: '/courses', title: 'دوره‌ها و کارگاه‌های آموزشی', zeroClientJs: true },
      { path: '/courses/anxiety-management', title: 'دوره جامع مدیریت استرس و اضطراب فراگیر', zeroClientJs: false },
      { path: '/workshops', title: 'کارگاه‌های تخصصی و آموزشی', zeroClientJs: true },
      { path: '/workshops/f47ac10b-58cc-4372-a567-0e02b2c3d479', title: 'کارگاه روان‌درمانی تحلیلی و تنظیم هیجان', zeroClientJs: false },
      { path: '/blog', title: 'مقالات و یادداشت‌های تخصصی', zeroClientJs: true },
      { path: '/blog/self-awareness-and-emotional-regulation', title: 'نقش خودآگاهی و تنظیم هیجان در روابط بین‌فردی', zeroClientJs: true },
    ];

    for (const page of contentPages) {
      const res = await fetchOnHost(`http://127.0.0.1:${sitePort}${page.path}`, {
        headers: { Host: hostA, 'X-Forwarded-Host': hostA },
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
    // Step 7: Interactive Pages & Booking Flow (PersianDatePicker Island & Jalali Calendar)
    // =========================================================================
    const resBooking = await fetchOnHost(`http://127.0.0.1:${sitePort}/booking`, {
      headers: { Host: hostA, 'X-Forwarded-Host': hostA },
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
    // Step 8: Strict Private Caching Verification (Account & Private Appointments)
    // =========================================================================
    const privatePages = [
      { path: '/account/courses', matchText: 'دوره‌های من' },
      { path: '/account/courses/c-demo-1', matchText: 'پخش جلسات دوره' },
      { path: '/appointments/appt-demo-1?lookup_hash=hash-test-123', matchText: 'وضعیت نوبت مشاوره' },
      { path: '/workshops/registrations/reg-test-123', matchText: 'وضعیت ثبت‌نام کارگاه' },
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
      assert.match(html, /dir="rtl"/, `Private page ${priv.path} must be RTL`);
      assert.match(html, /<meta name="viewport" content="width=device-width/, `Private page ${priv.path} must have responsive viewport`);
      assert.match(html, new RegExp(priv.matchText), `Private page ${priv.path} must contain expected content`);
      assert.doesNotMatch(html, /color:\s*#0f172a/);
    }
  } finally {
    if (stderrOutput) {
      process.stderr.write(`\n[child stderr]:\n${stderrOutput}\n`);
    }
    child.kill('SIGKILL');
    cacheProxyServer.closeAllConnections?.();
    mockApiServer.closeAllConnections?.();
    await new Promise((resolve) => cacheProxyServer.close(resolve));
    await new Promise((resolve) => mockApiServer.close(resolve));
  }
});
