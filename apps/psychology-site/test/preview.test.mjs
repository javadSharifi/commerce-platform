import test from 'node:test';
import assert from 'node:assert';
import { spawn } from 'node:child_process';

const siteDir = new URL('..', import.meta.url).pathname;

test('PSY-5.5: Home page / and Preview page /preview verification', async () => {
  const port = 4393;
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

    // 1. Home page / renders published appearance and public cache headers
    const resHome = await fetch(`http://127.0.0.1:${port}/`);
    assert.strictEqual(resHome.status, 200);
    const homeCacheControl = resHome.headers.get('cache-control') || '';
    assert.match(homeCacheControl, /public/);
    assert.match(homeCacheControl, /s-maxage=3600/);

    const htmlHome = await resHome.text();
    assert.match(htmlHome, /data-theme="minimal"/);
    assert.match(htmlHome, /data-palette="calm-sage"/);
    assert.match(htmlHome, /data-radius="md"/);
    assert.match(htmlHome, /data-scale="md"/);
    assert.match(htmlHome, /قالب اختصاصی مینیمال/);
    // Home must NOT have noindex, nofollow
    assert.doesNotMatch(htmlHome, /content="noindex,\s*nofollow"/);

    // 2. Home page / ignores appearance query params and strictly renders published appearance
    const resHomeWithQuery = await fetch(`http://127.0.0.1:${port}/?template=clinical&palette=royal-plum&radius=lg&scale=sm`);
    assert.strictEqual(resHomeWithQuery.status, 200);
    const htmlHomeWithQuery = await resHomeWithQuery.text();
    // Still minimal, still calm-sage palette and radius
    assert.match(htmlHomeWithQuery, /data-theme="minimal"/);
    assert.match(htmlHomeWithQuery, /data-palette="calm-sage"/);
    assert.doesNotMatch(htmlHomeWithQuery, /data-theme="clinical"/);
    assert.doesNotMatch(htmlHomeWithQuery, /data-palette="royal-plum"/);

    // 3. Preview page /preview requires private, no-store and noindex, nofollow
    const resPreview = await fetch(`http://127.0.0.1:${port}/preview`);
    assert.strictEqual(resPreview.status, 200);
    const previewCacheControl = resPreview.headers.get('cache-control') || '';
    assert.strictEqual(previewCacheControl, 'private, no-store');

    const htmlPreview = await resPreview.text();
    assert.match(htmlPreview, /<meta name="robots" content="noindex, nofollow"/);
    assert.match(htmlPreview, /حالت پیش‌نمایش ظاهر/);

    // 4. Preview page with valid template=clinical override
    const resPreviewClinical = await fetch(`http://127.0.0.1:${port}/preview?template=clinical`);
    assert.strictEqual(resPreviewClinical.status, 200);
    const htmlPreviewClinical = await resPreviewClinical.text();
    assert.match(htmlPreviewClinical, /data-theme="clinical"/);
    assert.match(htmlPreviewClinical, /مرکز تخصصی روان‌شناسی و مشاوره/);
    assert.match(htmlPreviewClinical, /قالب کلینیکال/);

    // 5. Preview page with valid visual overrides: palette, radius, scale
    const resPreviewCustom = await fetch(
      `http://127.0.0.1:${port}/preview?template=clinical&palette=ocean-trust&radius=lg&scale=sm`,
    );
    assert.strictEqual(resPreviewCustom.status, 200);
    const htmlPreviewCustom = await resPreviewCustom.text();
    assert.match(htmlPreviewCustom, /data-theme="clinical"/);
    assert.match(htmlPreviewCustom, /data-palette="ocean-trust"/);
    assert.match(htmlPreviewCustom, /data-radius="lg"/);
    assert.match(htmlPreviewCustom, /data-scale="sm"/);

    // 6. Preview page with invalid inputs safely falls back
    const resPreviewInvalid = await fetch(
      `http://127.0.0.1:${port}/preview?template=non_existent_template&palette=badpalette&radius=giant&scale=tiny`,
    );
    assert.strictEqual(resPreviewInvalid.status, 200);
    const htmlPreviewInvalid = await resPreviewInvalid.text();
    // Template falls back to minimal, palette falls back to calm-sage
    assert.match(htmlPreviewInvalid, /data-theme="minimal"/);
    assert.match(htmlPreviewInvalid, /data-palette="calm-sage"/);
    assert.match(htmlPreviewInvalid, /قالب اختصاصی مینیمال/);
    // Invalid radius, scale fall back to published/default
    assert.match(htmlPreviewInvalid, /data-radius="md"/);
    assert.match(htmlPreviewInvalid, /data-scale="md"/);

    // 7. Sitemap does NOT include /preview, includes /courses, and excludes fake placeholder slugs
    const resSitemap = await fetch(`http://127.0.0.1:${port}/sitemap.xml`);
    assert.strictEqual(resSitemap.status, 200);
    const xmlSitemap = await resSitemap.text();
    assert.doesNotMatch(xmlSitemap, /\/preview<\/loc>/);
    assert.match(xmlSitemap, /\/courses<\/loc>/);
    assert.doesNotMatch(xmlSitemap, /self-awareness-and-emotional-regulation/);
    assert.doesNotMatch(xmlSitemap, /managing-performance-anxiety/);
  } finally {
    child.kill('SIGTERM');
  }
});
