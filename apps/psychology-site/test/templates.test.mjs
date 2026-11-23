import test from 'node:test';
import assert from 'node:assert';
import http from 'node:http';
import { spawn } from 'node:child_process';
import { TEMPLATE_CATALOG } from '@psychology/ui';

const siteDir = new URL('..', import.meta.url).pathname;

test('Templates Registry and SSR BaseLayout verification', async () => {
  const port = 4397;
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
      setTimeout(() => reject(new Error('timeout waiting for server to start')), 6000);
    });

    // 1. Verify Templates Registry via /api/templates
    const apiRes = await fetch(`http://127.0.0.1:${port}/api/templates`);
    assert.strictEqual(apiRes.status, 200);
    const registeredTemplates = await apiRes.json();

    const catalogIds = TEMPLATE_CATALOG.map((t) => t.id).sort();
    const registeredIds = registeredTemplates.map((t) => t.id).sort();

    assert.deepStrictEqual(
      registeredIds,
      catalogIds,
      'Registry template IDs must exactly and bidirectionally match TEMPLATE_CATALOG',
    );

    for (const catalogEntry of TEMPLATE_CATALOG) {
      const found = registeredTemplates.find((t) => t.id === catalogEntry.id);
      assert.ok(found, `Template ${catalogEntry.id} must exist in registry`);
      assert.strictEqual(found.meta.id, catalogEntry.id);
      assert.strictEqual(found.meta.version, catalogEntry.version);
      assert.strictEqual(found.meta.daisyTheme, catalogEntry.daisyTheme);
      assert.strictEqual(found.meta.labelFa, catalogEntry.labelFa);

      // Verify all components are defined
      assert.strictEqual(found.components.hasHeader, true, `${catalogEntry.id} must have Header`);
      assert.strictEqual(found.components.hasFooter, true, `${catalogEntry.id} must have Footer`);
      assert.strictEqual(found.components.hasHomeHero, true, `${catalogEntry.id} must have HomeHero`);
      assert.strictEqual(found.components.hasCourseCard, true, `${catalogEntry.id} must have CourseCard`);
      assert.strictEqual(found.components.hasWorkshopCard, true, `${catalogEntry.id} must have WorkshopCard`);
      assert.strictEqual(found.components.hasBlogCard, true, `${catalogEntry.id} must have BlogCard`);
      assert.strictEqual(found.components.hasShell, true, `${catalogEntry.id} must have Shell`);
    }

    // 2. Default SSR request renders minimal template
    const resDefault = await fetch(`http://127.0.0.1:${port}/`);
    assert.strictEqual(resDefault.status, 200);
    const htmlDefault = await resDefault.text();
    assert.match(htmlDefault, /data-theme="minimal"/);
    assert.match(htmlDefault, /data-palette="calm-sage"/);
    assert.match(htmlDefault, /data-radius="md"/);
    assert.match(htmlDefault, /data-scale="md"/);
    assert.match(htmlDefault, /قالب اختصاصی مینیمال/);

    // 3. Clinical SSR request on /preview renders clinical template
    const resClinical = await fetch(`http://127.0.0.1:${port}/preview?template=clinical`);
    assert.strictEqual(resClinical.status, 200);
    const htmlClinical = await resClinical.text();
    assert.match(htmlClinical, /data-theme="clinical"/);
    assert.match(htmlClinical, /مرکز تخصصی روان‌شناسی و مشاوره/);
    assert.match(htmlClinical, /قالب کلینیکال/);

    // 4. Unknown template safely falls back to minimal template
    const resFallback = await fetch(`http://127.0.0.1:${port}/preview?template=unknown_xyz_random`);
    assert.strictEqual(resFallback.status, 200);
    const htmlFallback = await resFallback.text();
    assert.match(htmlFallback, /data-theme="minimal"/);
    assert.match(htmlFallback, /قالب اختصاصی مینیمال/);

    // 5. Empty template safely falls back to minimal template
    const resEmpty = await fetch(`http://127.0.0.1:${port}/preview?template=`);
    assert.strictEqual(resEmpty.status, 200);
    const htmlEmpty = await resEmpty.text();
    assert.match(htmlEmpty, /data-theme="minimal"/);
    assert.match(htmlEmpty, /قالب اختصاصی مینیمال/);

    // 6. Inherited Object prototype properties safely fall back to minimal
    for (const protoKey of ['toString', 'valueOf', 'constructor', '__proto__', 'hasOwnProperty', 'isPrototypeOf']) {
      const resProto = await fetch(`http://127.0.0.1:${port}/preview?template=${protoKey}`);
      assert.strictEqual(
        resProto.status,
        200,
        `Preview with inherited prototype template=${protoKey} must not crash SSR (expected 200 OK)`
      );
      const htmlProto = await resProto.text();
      assert.match(
        htmlProto,
        /data-theme="minimal"/,
        `Preview with inherited prototype template=${protoKey} must safely fall back to minimal`
      );
      assert.match(htmlProto, /قالب اختصاصی مینیمال/);
    }
  } finally {
    child.kill('SIGTERM');
  }
});

test('PSY-5.17: Home page / fallback to minimal for unknown and inherited prototype template IDs', async () => {
  const mockApiPort = 4414;
  const sitePort = 4415;
  const http = await import('node:http');

  let currentTemplateId = 'toString';

  const mockServer = http.createServer((_req, res) => {
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(
      JSON.stringify({
        name: 'کلینیک نمونه آزمایشی',
        title: 'مرکز روان‌شناسی تست',
        description: 'توضیحات تست فال‌بک قالب',
        phone: '۰۲۱-۸۸۸۸۸۸۸۸',
        address: 'تهران، خیابان آزادی',
        primary_color: '#0284c7',
        template_id: currentTemplateId,
        template_version: '1.0.0',
        theme_radius: 'md',
        theme_scale: 'md',
      })
    );
  });

  await new Promise((resolve) => mockServer.listen(mockApiPort, '127.0.0.1', resolve));

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
    await new Promise((resolve, reject) => {
      child.stdout.on('data', (data) => {
        if (data.toString().includes('Server listening')) {
          resolve();
        }
      });
      child.on('error', reject);
      setTimeout(() => reject(new Error('timeout waiting for site server to start')), 6000);
    });

    for (const testId of ['toString', 'valueOf', 'constructor', '__proto__', 'nonexistent_template_xyz']) {
      currentTemplateId = testId;
      const resHome = await fetch(`http://127.0.0.1:${sitePort}/`, {
        headers: { Host: 'rezaei-clinic.ir', 'X-Forwarded-Host': 'rezaei-clinic.ir' },
      });
      assert.strictEqual(
        resHome.status,
        200,
        `Home page with published template_id="${testId}" must return 200 OK`
      );
      const htmlHome = await resHome.text();
      assert.match(
        htmlHome,
        /data-theme="minimal"/,
        `Published template_id="${testId}" must safely fall back to minimal theme`
      );
      assert.match(htmlHome, /قالب اختصاصی مینیمال/);
    }
  } finally {
    child.kill('SIGTERM');
    await new Promise((resolve) => mockServer.close(resolve));
  }
});

test('PSY-6.1: Four Palettes SSR Render, Fixed Template with Changed Palette, and Fixed Palette with Changed Template', async () => {
  const sitePort = 4416;
  const child = spawn('node', ['./dist/server/entry.mjs'], {
    cwd: siteDir,
    env: {
      ...process.env,
      PORT: String(sitePort),
      HOST: '127.0.0.1',
    },
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
      setTimeout(() => reject(new Error('timeout waiting for site server to start')), 6000);
    });

    const host = 'rezaei-clinic.ir';

    // 1. Default SSR render of default palette (calm-sage)
    const resDefault = await fetch(`http://127.0.0.1:${sitePort}/`, {
      headers: { Host: host, 'X-Forwarded-Host': host },
    });
    assert.strictEqual(resDefault.status, 200);
    const htmlDefault = await resDefault.text();
    assert.match(htmlDefault, /data-palette="calm-sage"/, 'Default SSR must render calm-sage palette');

    // 2. Preview with default palette (calm-sage) and non-default palette (ocean-trust)
    const resSage = await fetch(`http://127.0.0.1:${sitePort}/preview?palette=calm-sage`, {
      headers: { Host: host, 'X-Forwarded-Host': host },
    });
    assert.strictEqual(resSage.status, 200);
    const htmlSage = await resSage.text();
    assert.match(htmlSage, /data-palette="calm-sage"/);

    const resOcean = await fetch(`http://127.0.0.1:${sitePort}/preview?palette=ocean-trust`, {
      headers: { Host: host, 'X-Forwarded-Host': host },
    });
    assert.strictEqual(resOcean.status, 200);
    const htmlOcean = await resOcean.text();
    assert.match(htmlOcean, /data-palette="ocean-trust"/);

    // 3. Fixed Template (minimal) with 4 Changed Palettes
    for (const pal of ['calm-sage', 'ocean-trust', 'warm-sand', 'royal-plum']) {
      const resPal = await fetch(`http://127.0.0.1:${sitePort}/preview?template=minimal&palette=${pal}`, {
        headers: { Host: host, 'X-Forwarded-Host': host },
      });
      assert.strictEqual(resPal.status, 200);
      const htmlPal = await resPal.text();
      assert.match(htmlPal, /data-theme="minimal"/, `Template must remain minimal for palette ${pal}`);
      assert.match(htmlPal, new RegExp(`data-palette="${pal}"`), `Must render palette ${pal}`);
    }

    // 4. Fixed Palette (ocean-trust) with Changed Template (minimal vs clinical)
    const resMinOcean = await fetch(
      `http://127.0.0.1:${sitePort}/preview?template=minimal&palette=ocean-trust`,
      { headers: { Host: host, 'X-Forwarded-Host': host } }
    );
    assert.strictEqual(resMinOcean.status, 200);
    const htmlMinOcean = await resMinOcean.text();
    assert.match(htmlMinOcean, /data-theme="minimal"/);
    assert.match(htmlMinOcean, /data-palette="ocean-trust"/);

    const resClinOcean = await fetch(
      `http://127.0.0.1:${sitePort}/preview?template=clinical&palette=ocean-trust`,
      { headers: { Host: host, 'X-Forwarded-Host': host } }
    );
    assert.strictEqual(resClinOcean.status, 200);
    const htmlClinOcean = await resClinOcean.text();
    assert.match(htmlClinOcean, /data-theme="clinical"/);
    assert.match(htmlClinOcean, /data-palette="ocean-trust"/);

    // 5. Safe fallback for unknown and prototype keys: falls back to calm-sage
    for (const invalidPal of ['unknown_pal', 'toString', 'valueOf', 'constructor', '__proto__']) {
      const resFallback = await fetch(`http://127.0.0.1:${sitePort}/preview?palette=${invalidPal}`, {
        headers: { Host: host, 'X-Forwarded-Host': host },
      });
      assert.strictEqual(resFallback.status, 200, `Unknown palette ${invalidPal} must return 200 OK`);
      const htmlFallback = await resFallback.text();
      assert.match(
        htmlFallback,
        /data-palette="calm-sage"/,
        `Invalid palette ${invalidPal} must fall back safely to calm-sage`
      );
    }
  } finally {
    child.kill('SIGTERM');
  }
});

test('PSY-6.2: Optional Shell in Templates Registry, BaseLayout Meta Ownership and Inner Routes Regression', async () => {
  const sitePort = 4417;
  const child = spawn('node', ['./dist/server/entry.mjs'], {
    cwd: siteDir,
    env: {
      ...process.env,
      PORT: String(sitePort),
      HOST: '127.0.0.1',
    },
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
      setTimeout(() => reject(new Error('timeout waiting for site server to start')), 6000);
    });

    const host = 'rezaei-clinic.ir';

    // 1. Home page / renders minimal-shell
    const resHome = await fetch(`http://127.0.0.1:${sitePort}/`, {
      headers: { Host: host, 'X-Forwarded-Host': host },
    });
    assert.strictEqual(resHome.status, 200);
    const htmlHome = await resHome.text();
    assert.match(htmlHome, /class="[^"]*minimal-shell[^"]*"/, 'Home page must use minimal-shell');
    assert.match(htmlHome, /<main class="flex-1 w-full"/, 'Shell must provide main content container');

    // 2. Preview page with minimal vs clinical shell
    const resPrevMin = await fetch(`http://127.0.0.1:${sitePort}/preview?template=minimal`, {
      headers: { Host: host, 'X-Forwarded-Host': host },
    });
    assert.strictEqual(resPrevMin.status, 200);
    const htmlPrevMin = await resPrevMin.text();
    assert.match(htmlPrevMin, /class="[^"]*minimal-shell[^"]*"/, 'Preview minimal must use minimal-shell');

    const resPrevClin = await fetch(`http://127.0.0.1:${sitePort}/preview?template=clinical`, {
      headers: { Host: host, 'X-Forwarded-Host': host },
    });
    assert.strictEqual(resPrevClin.status, 200);
    const htmlPrevClin = await resPrevClin.text();
    assert.match(htmlPrevClin, /class="[^"]*clinical-shell[^"]*"/, 'Preview clinical must use clinical-shell');

    // 3. Meta Ownership in BaseLayout: BaseLayout strictly owns head, meta, charset, viewport and html attributes
    assert.match(htmlHome, /<!doctype html>/i);
    assert.match(htmlHome, /<html\s+lang="fa"\s+dir="rtl"/);
    assert.match(htmlHome, /<meta charset="utf-8"/);
    assert.match(htmlHome, /<meta name="viewport" content="width=device-width/);
    assert.match(htmlHome, /<title>/);
    assert.match(htmlHome, /<meta name="description"/);
    // Ensure Shell does NOT inject duplicate html or head tags
    const htmlTagMatches = htmlHome.match(/<html/g);
    assert.strictEqual(htmlTagMatches?.length, 1, 'Only BaseLayout must render <html>');
    const headTagMatches = htmlHome.match(/<head>/g);
    assert.strictEqual(headTagMatches?.length, 1, 'Only BaseLayout must render <head>');

    // 4. Inner Routes Regression: All inner routes render within the template Shell
    const innerRoutes = [
      { path: '/courses', checkText: 'دوره‌های آموزشی تخصصی' },
      { path: '/workshops', checkText: 'کارگاه‌های تخصصی و آموزشی' },
      { path: '/blog', checkText: 'مقالات و یادداشت‌های تخصصی' },
      { path: '/booking', checkText: 'درخواست نوبت مشاوره' },
      { path: '/account/courses', checkText: 'دوره‌های من' },
    ];

    for (const route of innerRoutes) {
      const res = await fetch(`http://127.0.0.1:${sitePort}${route.path}`, {
        headers: { Host: host, 'X-Forwarded-Host': host },
      });
      assert.strictEqual(res.status, 200, `Route ${route.path} must return 200 OK`);
      const html = await res.text();
      assert.match(html, /class="[^"]*minimal-shell[^"]*"/, `Route ${route.path} must be wrapped in template Shell`);
      assert.match(html, new RegExp(route.checkText), `Route ${route.path} must contain its content`);
      assert.match(html, /dir="rtl"/, `Route ${route.path} must retain RTL layout`);
      assert.match(html, /<meta name="viewport"/, `Route ${route.path} must retain responsive meta`);
    }
  } finally {
    child.kill('SIGTERM');
  }
});

test('PSY-6.3: Mobile-App Template SSR Render, Bottom Navigation Bar (4 Destinations + More Menu), RTL, Course Details and Booking Form in Mobile', async () => {
  const mockApiPort = 4418;
  const sitePort = 4419;
  const http = await import('node:http');

  const mockServer = http.createServer((_req, res) => {
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(
      JSON.stringify({
        name: 'کلینیک روان‌شناسی همراه',
        title: 'مرکز مشاوره و سلامت روان همراه',
        description: 'سامانه تخصصی موبایل‌محور سلامت روان و روان‌درمانی',
        phone: '۰۲۱-۲۲۲۲۳۳۴۴',
        address: 'تهران، میدان ونک، خیابان ملاصدرا',
        primary_color: '#2563eb',
        template_id: 'mobile-app',
        template_version: '1.0.0',
        palette_id: 'ocean-trust',
        theme_radius: 'md',
        theme_scale: 'md',
      })
    );
  });

  await new Promise((resolve) => mockServer.listen(mockApiPort, '127.0.0.1', resolve));

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
    await new Promise((resolve, reject) => {
      child.stdout.on('data', (data) => {
        if (data.toString().includes('Server listening')) {
          resolve();
        }
      });
      child.on('error', reject);
      setTimeout(() => reject(new Error('timeout waiting for site server to start')), 6000);
    });

    const host = 'hamrah-clinic.ir';

    // 1. Home page / with mobile-app template
    const resHome = await fetch(`http://127.0.0.1:${sitePort}/`, {
      headers: { Host: host, 'X-Forwarded-Host': host },
    });
    assert.strictEqual(resHome.status, 200);
    const htmlHome = await resHome.text();

    assert.match(htmlHome, /data-theme="mobile-app"/);
    assert.match(htmlHome, /class="[^"]*mobile-app-shell[^"]*"/);
    assert.match(htmlHome, /class="[^"]*mobile-bottom-nav[^"]*"/);
    assert.match(htmlHome, /style="[^"]*scroll-padding-bottom:\s*5rem;?[^"]*"/);

    // Verify the 4 primary destinations in bottom nav
    assert.match(htmlHome, /href="\/"[^>]*aria-current="page"/, 'Home destination must be marked active on /');
    assert.match(htmlHome, /href="\/courses"/, 'Must contain courses destination');
    assert.match(htmlHome, /href="\/workshops"/, 'Must contain workshops destination');
    assert.match(htmlHome, /href="\/booking"/, 'Must contain booking destination');

    // Verify the "More" menu and its items
    assert.match(htmlHome, /aria-label="منوی بیشتر"/, 'Must contain More menu toggle');
    assert.match(htmlHome, /href="\/blog"/, 'More menu must contain blog destination');
    assert.match(htmlHome, /href="\/account\/courses"/, 'More menu must contain account destination');

    // Verify RTL & document metadata ownership
    assert.match(htmlHome, /<html\s+lang="fa"\s+dir="rtl"/);
    assert.match(htmlHome, /<meta name="viewport" content="width=device-width/);

    // 2. Course Details page (/courses/anxiety-management) in mobile-app shell
    const resCourse = await fetch(`http://127.0.0.1:${sitePort}/courses/anxiety-management`, {
      headers: { Host: host, 'X-Forwarded-Host': host },
    });
    assert.strictEqual(resCourse.status, 200);
    const htmlCourse = await resCourse.text();
    assert.match(htmlCourse, /class="[^"]*mobile-app-shell[^"]*"/);
    assert.match(htmlCourse, /class="[^"]*mobile-bottom-nav[^"]*"/);
    assert.match(htmlCourse, /href="\/courses"[^>]*aria-current="page"/, 'Courses destination must be active on course detail');
    assert.match(htmlCourse, /دوره جامع مدیریت استرس و اضطراب فراگیر/, 'Must contain course details');

    // 3. Booking page (/booking) in mobile-app shell
    const resBooking = await fetch(`http://127.0.0.1:${sitePort}/booking`, {
      headers: { Host: host, 'X-Forwarded-Host': host },
    });
    assert.strictEqual(resBooking.status, 200);
    const htmlBooking = await resBooking.text();
    assert.match(htmlBooking, /class="[^"]*mobile-app-shell[^"]*"/);
    assert.match(htmlBooking, /class="[^"]*mobile-bottom-nav[^"]*"/);
    assert.match(htmlBooking, /href="\/booking"[^>]*aria-current="page"/, 'Booking destination must be active on booking page');
    assert.match(htmlBooking, /درخواست نوبت مشاوره/, 'Must contain booking page content');
    assert.match(htmlBooking, /overflow-x-hidden/, 'Must contain mobile overflow-x guard');

    // 4. Preview endpoint with template=mobile-app
    const resPreview = await fetch(`http://127.0.0.1:${sitePort}/preview?template=mobile-app`, {
      headers: { Host: host, 'X-Forwarded-Host': host },
    });
    assert.strictEqual(resPreview.status, 200);
    const htmlPreview = await resPreview.text();
    assert.match(htmlPreview, /data-theme="mobile-app"/);
    assert.match(htmlPreview, /class="[^"]*mobile-app-shell[^"]*"/);
    assert.match(htmlPreview, /class="[^"]*mobile-bottom-nav[^"]*"/);
  } finally {
    child.kill('SIGTERM');
    await new Promise((resolve) => mockServer.close(resolve));
  }
});

test('PSY-6.6.1: Site routes take published/preview palette and private routes remain no-store', async () => {
  const sitePort = 4402;
  const mockApiPort = 4403;

  const mockServer = http.createServer((_req, res) => {
    res.setHeader('Content-Type', 'application/json; charset=utf-8');
    res.writeHead(200);
    res.end(
      JSON.stringify({
        name: 'کلینیک روان‌شناسی شن‌های گرم',
        title: 'مرکز تخصصی مشاوره شن‌های گرم',
        description: 'روان‌درمانی و رشد فردی با پالت اختصاصی شن‌های گرم',
        phone: '۰۲۱-۵۵۵۵۶۶۷۷',
        address: 'تهران، خیابان ولیعصر',
        template_id: 'minimal',
        template_version: '1.0.0',
        palette_id: 'warm-sand',
        theme_radius: 'md',
        theme_scale: 'md',
      })
    );
  });

  await new Promise((resolve) => mockServer.listen(mockApiPort, '127.0.0.1', resolve));

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
    await new Promise((resolve, reject) => {
      child.stdout.on('data', (data) => {
        if (data.toString().includes('Server listening')) {
          resolve();
        }
      });
      child.on('error', reject);
      setTimeout(() => reject(new Error('timeout waiting for site server to start')), 6000);
    });

    const host = 'warm-clinic.ir';

    // 1. Home page / takes published palette (warm-sand)
    const resHome = await fetch(`http://127.0.0.1:${sitePort}/`, {
      headers: { Host: host, 'X-Forwarded-Host': host },
    });
    assert.strictEqual(resHome.status, 200);
    const htmlHome = await resHome.text();
    assert.match(htmlHome, /data-palette="warm-sand"/);

    // 2. Preview page /preview takes override palette (royal-plum) and has private, no-store
    const resPreview = await fetch(`http://127.0.0.1:${sitePort}/preview?palette=royal-plum`, {
      headers: { Host: host, 'X-Forwarded-Host': host },
    });
    assert.strictEqual(resPreview.status, 200);
    const previewCache = resPreview.headers.get('cache-control') || '';
    assert.match(previewCache, /no-store/);
    assert.match(previewCache, /private/);
    const htmlPreview = await resPreview.text();
    assert.match(htmlPreview, /data-palette="royal-plum"/);

    // 3. Detail pages take published palette (warm-sand)
    const detailPaths = [
      '/courses/anxiety-management',
      '/workshops/f47ac10b-58cc-4372-a567-0e02b2c3d479',
      '/blog/self-awareness-and-emotional-regulation',
      '/booking',
    ];

    for (const dPath of detailPaths) {
      const res = await fetch(`http://127.0.0.1:${sitePort}${dPath}`, {
        headers: { Host: host, 'X-Forwarded-Host': host },
      });
      assert.strictEqual(res.status, 200, `Path ${dPath} must return 200 OK`);
      const html = await res.text();
      assert.match(html, /data-palette="warm-sand"/, `Path ${dPath} must render published warm-sand palette`);
    }

    // 4. Private account and tracking pages take published palette and strictly keep no-store
    const privatePaths = [
      '/account/courses',
      '/account/courses/c1111111-2222-3333-4444-555555555555',
      '/appointments/app-test-123',
      '/workshops/registrations/reg-test-123',
    ];

    for (const pPath of privatePaths) {
      const res = await fetch(`http://127.0.0.1:${sitePort}${pPath}`, {
        headers: { Host: host, 'X-Forwarded-Host': host },
      });
      assert.strictEqual(res.status, 200, `Path ${pPath} must return 200 OK`);
      const cacheHeader = res.headers.get('cache-control') || '';
      assert.match(cacheHeader, /no-store/, `Path ${pPath} must strictly carry no-store`);
      assert.match(cacheHeader, /private/, `Path ${pPath} must strictly carry private`);
      const html = await res.text();
      assert.match(html, /data-palette="warm-sand"/, `Path ${pPath} must render published warm-sand palette`);
    }
  } finally {
    child.kill('SIGTERM');
    await new Promise((resolve) => mockServer.close(resolve));
  }
});
