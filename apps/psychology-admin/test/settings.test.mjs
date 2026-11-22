import test from 'node:test';
import assert from 'node:assert';
import { spawn } from 'node:child_process';
import { TEMPLATE_CATALOG, PRESET_PRIMARY_COLORS } from '@psychology/ui';

test('PSY-5.6: Admin settings page and API endpoints verification', async () => {
  const port = 4402;
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

    // 1. GET /settings renders UI components, catalog templates, tokens, and preview button
    const resPage = await fetch(`http://127.0.0.1:${port}/settings`);
    assert.strictEqual(resPage.status, 200);
    const html = await resPage.text();

    // Verify template options from catalog are rendered
    for (const tpl of TEMPLATE_CATALOG) {
      assert.ok(html.includes(tpl.labelFa), `Missing template label: ${tpl.labelFa}`);
      assert.match(html, new RegExp(`value="${tpl.id}"`));
    }

    // Verify preset color buttons are rendered
    for (const preset of PRESET_PRIMARY_COLORS.slice(0, 3)) {
      assert.ok(html.includes(preset.labelFa), `Missing preset color label: ${preset.labelFa}`);
      assert.match(html, new RegExp(`data-color="${preset.value}"`));
    }

    // Verify radius and scale options exist
    assert.match(html, /name="theme_radius"/);
    assert.match(html, /name="theme_scale"/);

    // Verify live preview link with /preview target
    assert.match(html, /\/preview\?template=/);
    assert.match(html, /پیش‌نمایش زنده در وب‌سایت/);
    assert.match(html, /ذخیره و انتشار تغییرات/);

    // 2. GET /api/admin/site returns private no-store settings JSON
    const resGetApi = await fetch(`http://127.0.0.1:${port}/api/admin/site`);
    assert.strictEqual(resGetApi.status, 200);
    assert.strictEqual(resGetApi.headers.get('cache-control'), 'private, no-store');
    const getData = await resGetApi.json();
    assert.ok(getData.template_id);
    assert.ok(getData.primary_color);
    assert.ok(getData.theme_radius);
    assert.ok(getData.theme_scale);

    // 3. PUT /api/admin/site saves settings with private no-store
    const payload = {
      title: 'کلینیک روان‌شناسی بالینی دکتر مریم علوی',
      description: 'ارائه خدمات روان‌درمانی و مشاوره‌های تخصصی خانواده و فردی',
      phone: '۰۲۱-۸۸۸۸۸۸۸۸',
      address: 'تهران، خیابان ولیعصر',
      template_id: 'clinical',
      primary_color: '#0f766e',
      theme_radius: 'lg',
      theme_scale: 'sm',
    };

    const resPutApi = await fetch(`http://127.0.0.1:${port}/api/admin/site`, {
      method: 'PUT',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(payload),
    });

    assert.strictEqual(resPutApi.status, 200);
    assert.strictEqual(resPutApi.headers.get('cache-control'), 'private, no-store');
    const putData = await resPutApi.json();
    assert.strictEqual(putData.template_id, 'clinical');
    assert.strictEqual(putData.primary_color, '#0f766e');
    assert.strictEqual(putData.theme_radius, 'lg');
    assert.strictEqual(putData.theme_scale, 'sm');
  } finally {
    child.kill('SIGTERM');
  }
});
