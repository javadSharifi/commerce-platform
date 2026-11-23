import test from 'node:test';
import assert from 'node:assert';
import { spawn } from 'node:child_process';
import { TEMPLATE_CATALOG, EDITORIAL_TEMPLATE_META } from '@psychology/ui';

const siteDir = new URL('..', import.meta.url).pathname;

test('PSY-7.1: Editorial Template Shell Contract, Tokens, Local Font, Focus, and Dual Palette Verification', async () => {
  const port = 4452;
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
      setTimeout(() => reject(new Error('timeout waiting for site server to start')), 6000);
    });

    // 1. Guard check: editorial IS in TEMPLATE_CATALOG for publishing after full route coverage (PSY-7.6)
    const inPublishableCatalog = TEMPLATE_CATALOG.some((t) => t.id === 'editorial');
    assert.strictEqual(
      inPublishableCatalog,
      true,
      'Editorial template must be in publishable TEMPLATE_CATALOG now (PSY-7.6 full route coverage complete)',
    );
    assert.strictEqual(EDITORIAL_TEMPLATE_META.id, 'editorial');
    assert.strictEqual(EDITORIAL_TEMPLATE_META.version, '1.0.0');
    assert.strictEqual(EDITORIAL_TEMPLATE_META.daisyTheme, 'editorial');

    // 2. Public /api/templates must include publishable templates including editorial
    const apiRes = await fetch(`http://127.0.0.1:${port}/api/templates`);
    assert.strictEqual(apiRes.status, 200);
    const pubTemplates = await apiRes.json();
    assert.strictEqual(pubTemplates.some((t) => t.id === 'editorial'), true);

    // With ?all=true, editorial must be listed with all standard components
    const apiAllRes = await fetch(`http://127.0.0.1:${port}/api/templates?all=true`);
    assert.strictEqual(apiAllRes.status, 200);
    const allTemplates = await apiAllRes.json();
    const editorialInAll = allTemplates.find((t) => t.id === 'editorial');
    assert.ok(editorialInAll, 'editorial template must be registered in all-templates query');
    assert.strictEqual(editorialInAll.components.hasHeader, true);
    assert.strictEqual(editorialInAll.components.hasFooter, true);
    assert.strictEqual(editorialInAll.components.hasHomeHero, true);
    assert.strictEqual(editorialInAll.components.hasCourseCard, true);
    assert.strictEqual(editorialInAll.components.hasWorkshopCard, true);
    assert.strictEqual(editorialInAll.components.hasBlogCard, true);
    assert.strictEqual(editorialInAll.components.hasShell, true);

    // 3. Render fixture: /preview?template=editorial with Palette 1 (calm-sage)
    const resCalm = await fetch(`http://127.0.0.1:${port}/preview?template=editorial&palette=calm-sage`);
    assert.strictEqual(resCalm.status, 200);
    const htmlCalm = await resCalm.text();

    // Verify RTL document properties and tokens
    assert.match(htmlCalm, /lang="fa"/, 'Document must have lang="fa"');
    assert.match(htmlCalm, /dir="rtl"/, 'Document must have dir="rtl"');
    assert.match(htmlCalm, /data-theme="editorial"/, 'Must render data-theme="editorial"');
    assert.match(htmlCalm, /data-palette="calm-sage"/, 'Must render data-palette="calm-sage"');

    // Verify Shell wrapper
    assert.match(htmlCalm, /template-shell/, 'Must render template-shell wrapper');
    assert.match(htmlCalm, /editorial-shell/, 'Must render editorial-shell class');

    // Verify Header components, navigation, and brand
    assert.match(htmlCalm, /<header[^>]*class="[^"]*sticky top-0/, 'Editorial header must have sticky positioning');
    assert.match(htmlCalm, /دوره‌ها/, 'Navigation must contain courses link');
    assert.match(htmlCalm, /کارگاه‌ها/, 'Navigation must contain workshops link');
    assert.match(htmlCalm, /وبلاگ/, 'Navigation must contain blog link');
    assert.match(htmlCalm, /دریافت نوبت/, 'Must render primary action CTA');

    // Verify focus-visible states
    assert.match(htmlCalm, /focus-visible:ring-primary/, 'Must specify focus-visible ring on interactive elements');
    assert.match(htmlCalm, /focus-visible:ring-2/, 'Must specify focus-visible ring-2');

    // Verify mobile zero-JS menu
    assert.match(htmlCalm, /<details class="dropdown dropdown-end"/, 'Must provide zero-JS details dropdown');
    assert.match(htmlCalm, /aria-label="منوی ناوبری"/, 'Mobile menu must have descriptive aria-label');

    // Verify Footer
    assert.match(htmlCalm, /<footer[^>]*class="[^"]*bg-base-200/, 'Footer must render with base-200 background');
    assert.match(htmlCalm, /دسترسی سریع/, 'Footer must render quick links section');
    assert.match(htmlCalm, /اطلاعات و پشتیبانی/, 'Footer must render trust and support section');

    // 4. Render fixture: /preview?template=editorial with Palette 2 (ocean-trust)
    const resOcean = await fetch(`http://127.0.0.1:${port}/preview?template=editorial&palette=ocean-trust`);
    assert.strictEqual(resOcean.status, 200);
    const htmlOcean = await resOcean.text();

    assert.match(htmlOcean, /data-theme="editorial"/, 'Theme must remain editorial');
    assert.match(htmlOcean, /data-palette="ocean-trust"/, 'Palette must switch to ocean-trust');
    assert.match(htmlOcean, /editorial-shell/, 'Must still use editorial shell');

    // 5. Verify local font asset / Vazirmatn definition in bundled CSS
    // Check bundled CSS on client
    const cssMatch = htmlCalm.match(/href="(\/_astro\/[^"]+\.css)"/);
    assert.ok(cssMatch, 'Page must link to bundled CSS file');
    const cssRes = await fetch(`http://127.0.0.1:${port}${cssMatch[1]}`);
    assert.strictEqual(cssRes.status, 200);
    const cssText = await cssRes.text();
    assert.match(cssText, /Vazirmatn/, 'Bundled CSS must contain Vazirmatn local font family token');
    assert.match(cssText, /\[data-theme=editorial\]|\[data-theme="editorial"\]/, 'Bundled CSS must contain editorial theme tokens');
  } finally {
    child.kill('SIGTERM');
  }
});
