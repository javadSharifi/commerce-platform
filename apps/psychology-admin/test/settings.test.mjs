import test from 'node:test';
import assert from 'node:assert';
import http from 'node:http';
import { spawn } from 'node:child_process';
import { TEMPLATE_CATALOG, PALETTE_CATALOG } from '@psychology/ui';

test('PSY-5.6: Admin settings page and API endpoints verification with 502 on offline API', async () => {
  const port = 4402;
  // Point to a guaranteed unused port so API is offline
  const child = spawn('node', ['./dist/server/entry.mjs'], {
    env: { ...process.env, PORT: String(port), HOST: '127.0.0.1', INTERNAL_API_URL: 'http://127.0.0.1:4499' },
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
    const resPage = await fetch(`http://127.0.0.1:${port}/settings`, {
      headers: {
        Cookie: 'session_token=test-token',
      },
    });
    assert.strictEqual(resPage.status, 200);
    const html = await resPage.text();

    // Verify template options from catalog are rendered
    for (const tpl of TEMPLATE_CATALOG) {
      assert.ok(html.includes(tpl.labelFa), `Missing template label: ${tpl.labelFa}`);
      assert.match(html, new RegExp(`value="${tpl.id}"`));
    }

    // Verify palette options from catalog are rendered
    for (const pal of PALETTE_CATALOG) {
      assert.ok(html.includes(pal.labelFa), `Missing palette label: ${pal.labelFa}`);
      assert.match(html, new RegExp(`value="${pal.id}"`));
    }

    // Verify radius and scale options exist
    assert.match(html, /name="theme_radius"/);
    assert.match(html, /name="theme_scale"/);

    // Verify live preview link with /preview target
    assert.match(html, /\/preview\?template=/);
    assert.match(html, /پیش‌نمایش زنده در وب‌سایت/);
    assert.match(html, /ذخیره و انتشار تغییرات/);

    // 2. GET /api/admin/site returns fallback defaults with private no-store settings JSON
    const resGetApi = await fetch(`http://127.0.0.1:${port}/api/admin/site`);
    assert.strictEqual(resGetApi.status, 200);
    assert.strictEqual(resGetApi.headers.get('cache-control'), 'private, no-store');
    const getData = await resGetApi.json();
    assert.ok(getData.template_id);
    assert.ok(getData.theme_radius);
    assert.ok(getData.theme_scale);

    // 3. PUT /api/admin/site returns real 502 Bad Gateway when API backend is offline (no mock 200)
    const payload = {
      title: 'کلینیک روان‌شناسی بالینی دکتر مریم علوی',
      description: 'ارائه خدمات روان‌درمانی و مشاوره‌های تخصصی خانواده و فردی',
      phone: '۰۲۱-۸۸۸۸۸۸۸۸',
      address: 'تهران، خیابان ولیعصر',
    };

    const resPutApi = await fetch(`http://127.0.0.1:${port}/api/admin/site`, {
      method: 'PUT',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(payload),
    });

    assert.strictEqual(resPutApi.status, 502, 'Must return 502 Bad Gateway instead of mock 200');
    assert.strictEqual(resPutApi.headers.get('cache-control'), 'private, no-store');
    const errData = await resPutApi.json();
    assert.strictEqual(errData.code, 'BAD_GATEWAY');
  } finally {
    child.kill('SIGTERM');
  }
});

test('PSY-5.6: PUT /api/admin/site successfully calls /v1/admin/site when backend is available', async () => {
  const mockApiPort = 4404;
  const adminPort = 4405;
  let receivedPath = '';
  let receivedBody = null;

  const mockServer = http.createServer((req, res) => {
    receivedPath = req.url || '';
    let body = '';
    req.on('data', (chunk) => {
      body += chunk;
    });
    req.on('end', () => {
      if (body) {
        receivedBody = JSON.parse(body);
      }
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(
        JSON.stringify({
          ...receivedBody,
          template_version: '1.0.0',
        })
      );
    });
  });

  await new Promise((resolve) => mockServer.listen(mockApiPort, '127.0.0.1', resolve));

  const child = spawn('node', ['./dist/server/entry.mjs'], {
    env: {
      ...process.env,
      PORT: String(adminPort),
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
      setTimeout(() => reject(new Error('timeout waiting for server to start')), 6000);
    });

    const payload = {
      title: 'کلینیک روان‌شناسی بالینی دکتر مریم علوی',
      description: 'ارائه خدمات روان‌درمانی و مشاوره‌های تخصصی خانواده و فردی',
      phone: '۰۲۱-۸۸۸۸۸۸۸۸',
      address: 'تهران، خیابان ولیعصر',
    };

    const resPut = await fetch(`http://127.0.0.1:${adminPort}/api/admin/site`, {
      method: 'PUT',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(payload),
    });

    assert.strictEqual(resPut.status, 200);
    assert.strictEqual(resPut.headers.get('cache-control'), 'private, no-store');
    const data = await resPut.json();
    assert.strictEqual(receivedPath, '/v1/admin/site', 'Must call /v1/admin/site with exact /v1 prefix');
    assert.strictEqual(receivedBody.title, payload.title);

    // Verify PUT /api/admin/appearance calls /v1/admin/site/appearance
    const appearancePayload = {
      template_id: 'clinical',
      palette_id: 'ocean-trust',
      theme_radius: 'lg',
      theme_scale: 'sm',
    };

    const resPutApp = await fetch(`http://127.0.0.1:${adminPort}/api/admin/appearance`, {
      method: 'PUT',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(appearancePayload),
    });

    assert.strictEqual(resPutApp.status, 200);
    assert.strictEqual(resPutApp.headers.get('cache-control'), 'private, no-store');
    assert.strictEqual(receivedPath, '/v1/admin/site/appearance', 'Must call /v1/admin/site/appearance');
    assert.strictEqual(receivedBody.palette_id, appearancePayload.palette_id);
  } finally {
    child.kill('SIGTERM');
    await new Promise((resolve) => mockServer.close(resolve));
  }
});
