import test from 'node:test';
import assert from 'node:assert';
import { spawn } from 'node:child_process';
import http from 'node:http';

const siteDir = new URL('..', import.meta.url).pathname;

test('PSY-7.2: Home / and /preview Redesign, Shared ViewModels, Reading Order, and Two-Clinic Fixture (Full Data vs Empty/No-Image)', async () => {
  // 1. Mock Backend API server providing two distinct tenant fixtures: Clinic A (Full Data) vs Clinic B (Empty/No-Image)
  const mockApiPort = 4453;
  const mockSiteServer = http.createServer((req, res) => {
    const hostHeader = (req.headers['x-forwarded-host'] || req.headers.host || '').toString();
    res.setHeader('Content-Type', 'application/json');

    // Tenant A (omid-psych.ir): Full profile with phone, address, and published editorial template
    if (hostHeader.includes('omid-psych.ir') || req.url?.includes('omid-psych.ir')) {
      res.writeHead(200);
      res.end(
        JSON.stringify({
          id: 'tenant-a-1111',
          name: 'مرکز مشاوره و روان‌شناسی امید',
          title: 'کلینیک تخصصی روان‌درمانی و مشاوره امید',
          description: 'فضایی امن و آرام برای خودآگاهی، بهبود روابط بین‌فردی و درمان تخصصی اضطراب و افسردگی',
          phone: '02188776655',
          address: 'تهران، خیابان ولیعصر، بالاتر از میدان ونک، پلاک ۱',
          template_id: 'editorial',
          palette_id: 'calm-sage',
          theme_radius: 'md',
          theme_scale: 'md',
        }),
      );
      return;
    }

    // Tenant B (dr-ahmadi.ir): Minimal/Empty state with null phone, null address, no images
    if (hostHeader.includes('dr-ahmadi.ir') || req.url?.includes('dr-ahmadi.ir')) {
      res.writeHead(200);
      res.end(
        JSON.stringify({
          id: 'tenant-b-2222',
          name: 'دکتر علیرضا احمدی',
          title: 'دکتر علیرضا احمدی — متخصص اعصاب و روان',
          description: 'مشاوره فردی و درمان اختلالات خلقی',
          phone: null,
          address: null,
          template_id: 'minimal',
          palette_id: 'ocean-trust',
          theme_radius: 'sm',
          theme_scale: 'md',
        }),
      );
      return;
    }

    // Default fallback
    res.writeHead(200);
    res.end(
      JSON.stringify({
        id: 'tenant-def-3333',
        name: 'کلینیک روان‌شناسی',
        title: 'مرکز خدمات روان‌شناختی',
        description: 'مشاوره و روان‌درمانی',
        phone: null,
        address: null,
        template_id: 'minimal',
        palette_id: 'calm-sage',
      }),
    );
  });

  await new Promise((resolve) => mockSiteServer.listen(mockApiPort, '127.0.0.1', resolve));

  // 2. Start Astro SSR server connected to the mock API
  const sitePort = 4454;
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

    // =========================================================================
    // SCENARIO 1: Clinic A (Full Data, omid-psych.ir) on Public Home /
    // =========================================================================
    const resAHome = await fetch(`http://127.0.0.1:${sitePort}/`, {
      headers: {
        'X-Forwarded-Host': 'omid-psych.ir',
        Host: 'omid-psych.ir',
      },
    });
    assert.strictEqual(resAHome.status, 200);
    const htmlAHome = await resAHome.text();

    // Verify cache header for public homepage
    const cacheCtrlA = resAHome.headers.get('cache-control') || '';
    assert.match(cacheCtrlA, /public/);
    assert.match(cacheCtrlA, /s-maxage=3600/);

    // Verify reading order & heading hierarchy:
    // H1 in Hero -> H2 in Sections -> H3 in Cards
    assert.match(htmlAHome, /<h1[^>]*>[\s\S]*?کلینیک تخصصی روان‌درمانی و مشاوره امید[\s\S]*?<\/h1>/);
    assert.match(htmlAHome, /<h2[^>]*id="specialties-heading"[^>]*>[\s\S]*?حوزه‌های تخصصی و خدمات[\s\S]*?<\/h2>/);
    assert.match(htmlAHome, /<h2[^>]*id="courses-heading"[^>]*>[\s\S]*?دوره‌های آموزشی منتخب[\s\S]*?<\/h2>/);
    assert.match(htmlAHome, /<h2[^>]*id="workshops-heading"[^>]*>[\s\S]*?کارگاه‌های تخصصی[\s\S]*?<\/h2>/);
    assert.match(htmlAHome, /<h2[^>]*id="blog-heading"[^>]*>[\s\S]*?تازه‌ترین مقالات و یادداشت‌ها[\s\S]*?<\/h2>/);
    assert.match(htmlAHome, /<h2[^>]*id="booking-cta-heading"[^>]*>[\s\S]*?جلسه معارفه و رزرو وقت مشاوره حضوری یا آنلاین[\s\S]*?<\/h2>/);

    // Verify card headings are H3
    assert.match(htmlAHome, /<h3[^>]*>مشاوره و درمان فردی<\/h3>/);
    assert.match(htmlAHome, /<h3[^>]*>مدیریت اضطراب و استرس<\/h3>/);
    assert.match(htmlAHome, /<h3[^>]*>زوج‌درمانی و تعارضات<\/h3>/);

    // Verify Hero CTAs
    assert.match(htmlAHome, /href="\/booking"[^>]*>[\s\S]*?رزرو نوبت مشاوره/);
    assert.match(htmlAHome, /href="\/courses"[^>]*>[\s\S]*?مشاهده دوره‌ها/);

    // Verify Contact card displays valid phone and address
    assert.match(htmlAHome, /02188776655/);
    assert.match(htmlAHome, /تهران، خیابان ولیعصر، بالاتر از میدان ونک، پلاک ۱/);

    // =========================================================================
    // SCENARIO 2: Clinic A on /preview with editorial template
    // =========================================================================
    const resAPreview = await fetch(`http://127.0.0.1:${sitePort}/preview?template=editorial&palette=calm-sage`, {
      headers: {
        'X-Forwarded-Host': 'omid-psych.ir',
        Host: 'omid-psych.ir',
      },
    });
    assert.strictEqual(resAPreview.status, 200);
    const htmlAPreview = await resAPreview.text();

    // Verify preview private headers & banner
    const cacheCtrlAPreview = resAPreview.headers.get('cache-control') || '';
    assert.match(cacheCtrlAPreview, /private/);
    assert.match(cacheCtrlAPreview, /no-store/);
    assert.match(htmlAPreview, /حالت پیش‌نمایش ظاهر/);
    assert.match(htmlAPreview, /data-theme="editorial"/);
    assert.match(htmlAPreview, /data-palette="calm-sage"/);

    // Same shared components & reading order
    assert.match(htmlAPreview, /id="specialties-heading"/);
    assert.match(htmlAPreview, /id="courses-heading"/);
    assert.match(htmlAPreview, /id="booking-cta-heading"/);

    // =========================================================================
    // SCENARIO 3: Clinic B (dr-ahmadi.ir) - Empty Data & Missing Image State
    // =========================================================================
    const resBHome = await fetch(`http://127.0.0.1:${sitePort}/`, {
      headers: {
        'X-Forwarded-Host': 'dr-ahmadi.ir',
        Host: 'dr-ahmadi.ir',
      },
    });
    assert.strictEqual(resBHome.status, 200);
    const htmlBHome = await resBHome.text();

    // Clinic B uses minimal template and ocean-trust palette
    assert.match(htmlBHome, /data-theme="minimal"/);
    assert.match(htmlBHome, /data-palette="ocean-trust"/);
    assert.match(htmlBHome, /دکتر علیرضا احمدی/);

    // Verify Contact details with null phone/address:
    // MUST NOT display literal "null" or "undefined" or broken HTML
    assert.doesNotMatch(htmlBHome, />null</);
    assert.doesNotMatch(htmlBHome, />undefined</);
    assert.doesNotMatch(htmlBHome, /تلفن تماس:[\s\S]*?null/);
    assert.match(htmlBHome, /هماهنگی جلسات و نوبت‌دهی به صورت کاملاً برخط از طریق سامانه رزرو سایت انجام می‌شود/);

    // Missing images: cards and hero must use neutral fallback placeholder, not broken img tag
    assert.match(htmlBHome, /بدون تصویر/);
    assert.doesNotMatch(htmlBHome, /<img[^>]*src="null"/);
    assert.doesNotMatch(htmlBHome, /<img[^>]*src="undefined"/);

    // =========================================================================
    // SCENARIO 4: Clinic B on /preview switching to editorial template
    // =========================================================================
    const resBPreview = await fetch(`http://127.0.0.1:${sitePort}/preview?template=editorial`, {
      headers: {
        'X-Forwarded-Host': 'dr-ahmadi.ir',
        Host: 'dr-ahmadi.ir',
      },
    });
    assert.strictEqual(resBPreview.status, 200);
    const htmlBPreview = await resBPreview.text();

    assert.match(htmlBPreview, /data-theme="editorial"/);
    assert.match(htmlBPreview, /دکتر علیرضا احمدی/);
    assert.match(htmlBPreview, /حالت پیش‌نمایش ظاهر/);

    // Missing images in editorial hero: must render neutral fallback placeholder surface
    assert.match(htmlBPreview, /فضای امن و اختصاصی مشاوره/);

    // Strict isolation: Clinic A data (phone, address, title) must never appear in Clinic B
    assert.doesNotMatch(htmlBPreview, /02188776655/);
    assert.doesNotMatch(htmlBPreview, /میدان ونک/);
  } finally {
    child.kill('SIGTERM');
    await new Promise((resolve) => mockSiteServer.close(resolve));
  }
});
