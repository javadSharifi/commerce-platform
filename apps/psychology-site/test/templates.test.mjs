import test from 'node:test';
import assert from 'node:assert';
import { spawn } from 'node:child_process';
import { TEMPLATE_CATALOG } from '@psychology/ui';

test('Templates Registry and SSR BaseLayout verification', async () => {
  const port = 4397;
  const child = spawn('node', ['./dist/server/entry.mjs'], {
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

      // Verify all 6 components are defined
      assert.strictEqual(found.components.hasHeader, true, `${catalogEntry.id} must have Header`);
      assert.strictEqual(found.components.hasFooter, true, `${catalogEntry.id} must have Footer`);
      assert.strictEqual(found.components.hasHomeHero, true, `${catalogEntry.id} must have HomeHero`);
      assert.strictEqual(found.components.hasCourseCard, true, `${catalogEntry.id} must have CourseCard`);
      assert.strictEqual(found.components.hasWorkshopCard, true, `${catalogEntry.id} must have WorkshopCard`);
      assert.strictEqual(found.components.hasBlogCard, true, `${catalogEntry.id} must have BlogCard`);
    }

    // 2. Default SSR request renders minimal template
    const resDefault = await fetch(`http://127.0.0.1:${port}/`);
    assert.strictEqual(resDefault.status, 200);
    const htmlDefault = await resDefault.text();
    assert.match(htmlDefault, /data-theme="minimal"/);
    assert.match(htmlDefault, /data-radius="md"/);
    assert.match(htmlDefault, /data-scale="md"/);
    assert.match(htmlDefault, /--color-primary:\s*#0284c7/);
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
  } finally {
    child.kill('SIGTERM');
  }
});
