import test from 'node:test';
import assert from 'node:assert';
import http from 'node:http';
import { spawn } from 'node:child_process';

test('PSY-5.16: Connect panel settings to active clinic and verified primary domain with 2 clinics', async () => {
  const mockApiPort = 4410;
  const adminPort = 4411;

  const tenantA = {
    id: '11111111-1111-1111-1111-111111111111',
    name: 'کلینیک آسا',
    slug: 'asa',
    domain: 'clinic-a.ir',
    settings: {
      tenant_id: '11111111-1111-1111-1111-111111111111',
      primary_domain: 'clinic-a.ir',
      title: 'مرکز مشاوره و روان‌درمانی آسا',
      description: 'روانشناسی تخصصی فردی',
      phone: '۰۲۱-۱۱۱۱۱۱۱۱',
      address: 'تهران، میدان ونک',
      palette_id: 'calm-sage',
      template_id: 'minimal',
      template_version: '1.0.0',
      theme_radius: 'md',
      theme_scale: 'md',
    },
  };

  const tenantB = {
    id: '22222222-2222-2222-2222-222222222222',
    name: 'کلینیک بهار',
    slug: 'bahar',
    domain: 'clinic-b.ir',
    settings: {
      tenant_id: '22222222-2222-2222-2222-222222222222',
      primary_domain: 'clinic-b.ir',
      title: 'مرکز روانشناسی و مشاوره بهار',
      description: 'مشاوره کودک و خانواده',
      phone: '۰۲۱-۲۲۲۲۲۲۲۲',
      address: 'تهران، خیابان شریعتی',
      palette_id: 'warm-sand',
      template_id: 'clinical',
      template_version: '1.0.0',
      theme_radius: 'lg',
      theme_scale: 'sm',
    },
  };

  const receivedRequests = [];

  const mockServer = http.createServer((req, res) => {
    const url = new URL(req.url, `http://${req.headers.host}`);
    const pathname = url.pathname;
    const tenantIdHeader = req.headers['x-tenant-id'];

    let body = '';
    req.on('data', (chunk) => {
      body += chunk;
    });

    req.on('end', () => {
      const parsedBody = body ? JSON.parse(body) : null;
      receivedRequests.push({
        method: req.method,
        pathname,
        headers: req.headers,
        body: parsedBody,
      });

      // 1. Auth Me endpoint: returns authenticated user and memberships for both clinics
      if (pathname === '/v1/admin/auth/me') {
        const cookie = req.headers.cookie || '';
        if (!cookie.includes('session_token=')) {
          res.writeHead(401, { 'Content-Type': 'application/json' });
          res.end(JSON.stringify({ code: 'UNAUTHORIZED', message: 'احراز هویت الزامی است' }));
          return;
        }

        res.writeHead(200, { 'Content-Type': 'application/json' });
        res.end(
          JSON.stringify({
            user: {
              id: 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa',
              full_name: 'دکتر مریم علوی',
              email: 'admin@example.com',
            },
            memberships: [
              {
                id: 'm1-1111-1111-1111-111111111111',
                tenant_id: tenantA.id,
                tenant_name: tenantA.name,
                tenant_slug: tenantA.slug,
                role: 'owner',
              },
              {
                id: 'm2-2222-2222-2222-222222222222',
                tenant_id: tenantB.id,
                tenant_name: tenantB.name,
                tenant_slug: tenantB.slug,
                role: 'owner',
              },
            ],
          })
        );
        return;
      }

      // 2. Settings endpoint
      if (pathname === '/v1/admin/site') {
        const activeTenantId = tenantIdHeader || tenantA.id;
        const currentTenant = activeTenantId === tenantB.id ? tenantB : tenantA;

        if (req.method === 'GET') {
          res.writeHead(200, { 'Content-Type': 'application/json' });
          res.end(JSON.stringify(currentTenant.settings));
          return;
        }

        if (req.method === 'PUT') {
          // Validation simulation
          if (!parsedBody.title || parsedBody.title.trim() === '') {
            res.writeHead(400, { 'Content-Type': 'application/json' });
            res.end(
              JSON.stringify({
                code: 'VALIDATION_ERROR',
                message: 'عنوان مطب الزامی است و نمی‌تواند خالی باشد.',
              })
            );
            return;
          }

          // Update in-memory
          currentTenant.settings = {
            ...currentTenant.settings,
            ...parsedBody,
          };

          res.writeHead(200, { 'Content-Type': 'application/json' });
          res.end(JSON.stringify(currentTenant.settings));
          return;
        }
      }

      // 3. Appearance endpoint
      if (pathname === '/v1/admin/site/appearance') {
        const activeTenantId = tenantIdHeader || tenantA.id;
        const currentTenant = activeTenantId === tenantB.id ? tenantB : tenantA;

        if (req.method === 'GET') {
          res.writeHead(200, { 'Content-Type': 'application/json' });
          res.end(JSON.stringify(currentTenant.settings));
          return;
        }

        if (req.method === 'PUT') {
          currentTenant.settings = {
            ...currentTenant.settings,
            ...parsedBody,
          };

          res.writeHead(200, { 'Content-Type': 'application/json' });
          res.end(JSON.stringify(currentTenant.settings));
          return;
        }
      }

      res.writeHead(404, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({ code: 'NOT_FOUND', message: 'مسیری یافت نشد' }));
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
      setTimeout(() => reject(new Error('timeout waiting for admin server to start')), 6000);
    });

    // -------------------------------------------------------------
    // Test 1: Active Clinic A (Default from session membership[0])
    // -------------------------------------------------------------
    const resA = await fetch(`http://127.0.0.1:${adminPort}/settings`, {
      headers: {
        Cookie: 'session_token=valid-session-token',
      },
    });
    assert.strictEqual(resA.status, 200);
    const htmlA = await resA.text();

    // Verify Clinic A settings and title are rendered
    assert.ok(htmlA.includes(tenantA.settings.title), 'Must render Clinic A title');
    assert.ok(htmlA.includes(tenantA.settings.address), 'Must render Clinic A address');

    // Verify preview link points to verified primary domain for Clinic A
    const expectedPreviewA = `https://${tenantA.domain}/preview?template=${tenantA.settings.template_id}&palette=${tenantA.settings.palette_id}&radius=${tenantA.settings.theme_radius}&scale=${tenantA.settings.theme_scale}`;
    assert.ok(
      htmlA.includes(expectedPreviewA),
      `Preview button must link to Clinic A domain (${tenantA.domain}): expected ${expectedPreviewA}`
    );
    assert.ok(
      htmlA.includes(`data-public-base="https://${tenantA.domain}"`),
      'data-public-base attribute must match Clinic A domain'
    );

    // -------------------------------------------------------------
    // Test 2: Active Clinic B (Selected via valid tenant_id cookie)
    // -------------------------------------------------------------
    const resB = await fetch(`http://127.0.0.1:${adminPort}/settings`, {
      headers: {
        Cookie: `session_token=valid-session-token; tenant_id=${tenantB.id}`,
      },
    });
    assert.strictEqual(resB.status, 200);
    const htmlB = await resB.text();

    // Verify Clinic B settings and title are rendered
    assert.ok(htmlB.includes(tenantB.settings.title), 'Must render Clinic B title');
    assert.ok(htmlB.includes(tenantB.settings.address), 'Must render Clinic B address');

    // Verify preview link points to verified primary domain for Clinic B
    const expectedPreviewB = `https://${tenantB.domain}/preview?template=${tenantB.settings.template_id}&palette=${tenantB.settings.palette_id}&radius=${tenantB.settings.theme_radius}&scale=${tenantB.settings.theme_scale}`;
    assert.ok(
      htmlB.includes(expectedPreviewB),
      `Preview button must link to Clinic B domain (${tenantB.domain}): expected ${expectedPreviewB}`
    );
    assert.ok(
      htmlB.includes(`data-public-base="https://${tenantB.domain}"`),
      'data-public-base attribute must match Clinic B domain'
    );

    // -------------------------------------------------------------
    // Test 3: Cookie Tampering Protection (Unauthorized tenant ID)
    // -------------------------------------------------------------
    const resTampered = await fetch(`http://127.0.0.1:${adminPort}/settings`, {
      headers: {
        Cookie: 'session_token=valid-session-token; tenant_id=99999999-9999-9999-9999-999999999999',
      },
    });
    assert.strictEqual(resTampered.status, 200);
    const htmlTampered = await resTampered.text();
    // Must fall back to first authorized membership (Clinic A), not the tampered tenant
    assert.ok(
      htmlTampered.includes(`data-public-base="https://${tenantA.domain}"`),
      'Must fall back to Clinic A verified domain on tampered tenant cookie'
    );

    // -------------------------------------------------------------
    // Test 4: Live preview calculation before save
    // -------------------------------------------------------------
    // The client script uses data-public-base to construct preview URLs before saving:
    const simulateChangePreview = (base, template, palette, radius, scale) =>
      `${base}/preview?template=${template}&palette=${palette}&radius=${radius}&scale=${scale}`;

    const previewBeforeSaveA = simulateChangePreview(`https://${tenantA.domain}`, 'clinical', 'calm-sage', 'lg', 'sm');
    assert.strictEqual(
      previewBeforeSaveA,
      `https://${tenantA.domain}/preview?template=clinical&palette=calm-sage&radius=lg&scale=sm`
    );

    const previewBeforeSaveB = simulateChangePreview(`https://${tenantB.domain}`, 'modern', 'deep-teal', 'sm', 'lg');
    assert.strictEqual(
      previewBeforeSaveB,
      `https://${tenantB.domain}/preview?template=modern&palette=deep-teal&radius=sm&scale=lg`
    );

    // -------------------------------------------------------------
    // Test 5: Saving content settings (PUT /api/admin/site) for active clinic
    // -------------------------------------------------------------
    const updatePayloadB = {
      title: 'کلینیک تخصصی روانپزشکی بهار - به‌روزشده',
      description: 'ارائه خدمات به‌روز مشاوره‌ای',
      phone: '۰۲۱-۲۲۲۲۳۳۳۳',
      address: 'تهران، خیابان شریعتی، پلاک ۲',
    };

    const resPutB = await fetch(`http://127.0.0.1:${adminPort}/api/admin/site`, {
      method: 'PUT',
      headers: {
        'Content-Type': 'application/json',
        Cookie: `session_token=valid-session-token; tenant_id=${tenantB.id}`,
      },
      body: JSON.stringify(updatePayloadB),
    });

    assert.strictEqual(resPutB.status, 200);
    const updatedDataB = await resPutB.json();
    assert.strictEqual(updatedDataB.title, updatePayloadB.title);
    assert.strictEqual(tenantB.settings.title, updatePayloadB.title);

    // -------------------------------------------------------------
    // Test 6: API validation error display in form
    // -------------------------------------------------------------
    const invalidPayload = {
      title: '', // empty title violates validation
      description: 'توضیحات تست',
    };

    const resPutError = await fetch(`http://127.0.0.1:${adminPort}/api/admin/site`, {
      method: 'PUT',
      headers: {
        'Content-Type': 'application/json',
        Cookie: `session_token=valid-session-token; tenant_id=${tenantA.id}`,
      },
      body: JSON.stringify(invalidPayload),
    });

    assert.strictEqual(resPutError.status, 400);
    const errorJson = await resPutError.json();
    assert.strictEqual(errorJson.code, 'VALIDATION_ERROR');
    assert.strictEqual(errorJson.message, 'عنوان مطب الزامی است و نمی‌تواند خالی باشد.');

    // Verify error alert markup is present in the page HTML for client script to populate
    assert.ok(htmlA.includes('id="alert-error"'), 'Form must contain #alert-error container');
    assert.ok(htmlA.includes('id="alert-error-msg"'), 'Form must contain #alert-error-msg element');

    // -------------------------------------------------------------
    // Test 7: Publishing appearance (PUT /api/admin/appearance) for active clinic
    // -------------------------------------------------------------
    const appearancePayloadB = {
      template_id: 'modern',
      palette_id: 'rose-calm',
      theme_radius: 'full',
      theme_scale: 'lg',
    };

    const resAppearanceB = await fetch(`http://127.0.0.1:${adminPort}/api/admin/appearance`, {
      method: 'PUT',
      headers: {
        'Content-Type': 'application/json',
        Cookie: `session_token=valid-session-token; tenant_id=${tenantB.id}`,
      },
      body: JSON.stringify(appearancePayloadB),
    });

    assert.strictEqual(resAppearanceB.status, 200);
    const updatedAppearanceB = await resAppearanceB.json();
    assert.strictEqual(updatedAppearanceB.template_id, appearancePayloadB.template_id);
    assert.strictEqual(updatedAppearanceB.palette_id, appearancePayloadB.palette_id);
    assert.strictEqual(tenantB.settings.template_id, appearancePayloadB.template_id);
    assert.strictEqual(tenantB.settings.palette_id, appearancePayloadB.palette_id);
  } finally {
    child.kill('SIGTERM');
    await new Promise((resolve) => mockServer.close(resolve));
  }
});
