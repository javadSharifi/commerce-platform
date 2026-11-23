import test from 'node:test';
import assert from 'node:assert';
import http from 'node:http';
import { spawn } from 'node:child_process';

test('PSY-5.15: SSR runtime with INTERNAL_API_URL, unknown host 404, unreachable 503, no-store and no sample data leak', async () => {
  // 1. Setup mock backend API on an ephemeral port
  let apiCallCount = 0;
  const mockApi = http.createServer((req, res) => {
    apiCallCount++;
    const host = (req.headers.host || '').split(':')[0];

    if (['clinic-alpha.ir', 'clinic-beta.ir'].includes(host) &&
        ['/v1/public/courses', '/v1/public/workshops', '/v1/public/blog'].includes(req.url)) {
      return res.writeHead(200, { 'Content-Type': 'application/json' }).end('[]');
    }

    if (req.url === '/v1/public/site') {
      if (host === 'clinic-alpha.ir') {
        res.writeHead(200, { 'Content-Type': 'application/json' });
        return res.end(
          JSON.stringify({
            name: 'کلینیک روان‌شناسی آلفا',
            title: 'دکتر آلفایی - متخصص اعصاب و روان',
            description: 'مشاوره تخصصی و درمان‌های شناختی رفتاری در کلینیک آلفا.',
            phone: '۰۲۱-۱۱۱۱۱۱۱۱',
            address: 'تهران، میدان آرژانتین',
            primary_color: '#0ea5e9',
            template_id: 'minimal',
            template_version: '1.0.0',
            theme_radius: 'md',
            theme_scale: 'md',
          }),
        );
      }

      if (host === 'clinic-beta.ir') {
        res.writeHead(200, { 'Content-Type': 'application/json' });
        return res.end(
          JSON.stringify({
            name: 'مرکز مشاوره بتا',
            title: 'دکتر بتایی - روان‌درمانگر خانواده',
            description: 'خدمات تخصصی زوج‌درمانی و مشاوره پیش از ازدواج در مرکز بتا.',
            phone: '۰۲۱-۲۲۲۲۲۲۲۲',
            address: 'تهران، خیابان نیاوران',
            primary_color: '#10b981',
            template_id: 'clinical',
            template_version: '1.0.0',
            theme_radius: 'lg',
            theme_scale: 'lg',
          }),
        );
      }

      // Unknown host
      res.writeHead(404, { 'Content-Type': 'application/json' });
      return res.end(
        JSON.stringify({
          code: 'HOST_NOT_FOUND',
          message: 'دامنه یا مطب مورد نظر در سیستم ثبت نشده است.',
        }),
      );
    }

    res.writeHead(404, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({ code: 'NOT_FOUND', message: 'مسیر یافت نشد.' }));
  });

  await new Promise((resolve) => mockApi.listen(0, '127.0.0.1', resolve));
  const apiAddress = mockApi.address();
  const apiPort = typeof apiAddress === 'object' && apiAddress ? apiAddress.port : 0;
  const internalApiUrl = `http://127.0.0.1:${apiPort}`;

  // 2. Spawn psychology-site production build with runtime INTERNAL_API_URL
  const sitePort = 4396;
  const getAsHost = (host, forwardedHost) => new Promise((resolve, reject) => {
    http.get({
      hostname: '127.0.0.1', port: sitePort, path: '/',
      headers: { Host: host, ...(forwardedHost && { 'X-Forwarded-Host': forwardedHost }) },
    }, async response => {
      try {
        const body = Buffer.concat(await Array.fromAsync(response));
        resolve(new Response(body, { status: response.statusCode, headers: response.headers }));
      } catch (err) {
        reject(err);
      }
    }).on('error', reject);
  });
  const siteDir = new URL('..', import.meta.url).pathname;
  const child = spawn('node', ['./dist/server/entry.mjs'], {
    cwd: siteDir,
    env: {
      ...process.env,
      PORT: String(sitePort),
      HOST: '127.0.0.1',
      INTERNAL_API_URL: internalApiUrl,
      NODE_ENV: 'production',
    },
    stdio: ['ignore', 'pipe', 'pipe'],
  });

  try {
    let stderrOutput = '';
    child.stderr.on('data', (d) => {
      stderrOutput += d.toString();
    });

    await new Promise((resolve, reject) => {
      child.stdout.on('data', (data) => {
        if (data.toString().includes('Server listening')) {
          resolve();
        }
      });
      child.on('error', reject);
      setTimeout(() => reject(new Error(`timeout waiting for site server: ${stderrOutput}`)), 6000);
    });

    // -------------------------------------------------------------------------
    // Scenario 1: Host Alpha receives its own profile at runtime via INTERNAL_API_URL
    // -------------------------------------------------------------------------
    const resAlpha = await getAsHost('clinic-alpha.ir', 'clinic-beta.ir');
    assert.strictEqual(resAlpha.status, 200);
    assert.ok(resAlpha.headers.get('cache-control')?.includes('public'));
    const htmlAlpha = await resAlpha.text();
    assert.match(htmlAlpha, /کلینیک روان‌شناسی آلفا/);
    assert.match(htmlAlpha, /دکتر آلفایی/);
    assert.match(htmlAlpha, /۰۲۱-۱۱۱۱۱۱۱۱/);
    assert.doesNotMatch(htmlAlpha, /کلینیک روان‌شناسی و مشاوره مهر/);

    // -------------------------------------------------------------------------
    // Scenario 2: Host Beta receives its distinct profile at runtime via INTERNAL_API_URL
    // -------------------------------------------------------------------------
    const resBeta = await getAsHost('clinic-beta.ir');
    assert.strictEqual(resBeta.status, 200);
    assert.ok(resBeta.headers.get('cache-control')?.includes('public'));
    const htmlBeta = await resBeta.text();
    assert.match(htmlBeta, /مرکز مشاوره بتا/);
    assert.match(htmlBeta, /دکتر بتایی/);
    assert.match(htmlBeta, /data-theme="clinical"/);
    assert.doesNotMatch(htmlBeta, /کلینیک روان‌شناسی و مشاوره مهر/);

    // -------------------------------------------------------------------------
    // Scenario 3: Unknown Host receives explicit 404 without shared cache (no-store)
    // -------------------------------------------------------------------------
    const resUnknown = await getAsHost('unknown-random-clinic.ir', 'clinic-alpha.ir');
    assert.strictEqual(resUnknown.status, 404);
    assert.strictEqual(resUnknown.headers.get('cache-control'), 'private, no-store');
    const htmlUnknown = await resUnknown.text();
    assert.match(htmlUnknown, /۴۰۴ - مطب یا دامنه یافت نشد/);
    assert.doesNotMatch(htmlUnknown, /کلینیک روان‌شناسی و مشاوره مهر/);
    assert.doesNotMatch(htmlUnknown, /دکتر علوی/);

    // -------------------------------------------------------------------------
    // Scenario 4: Unreachable API receives explicit 503 without shared cache (no-store)
    // -------------------------------------------------------------------------
    await new Promise((resolve) => mockApi.close(resolve));

    const resUnreachable = await getAsHost('clinic-alpha.ir');
    assert.strictEqual(resUnreachable.status, 503);
    assert.strictEqual(resUnreachable.headers.get('cache-control'), 'private, no-store');
    const htmlUnreachable = await resUnreachable.text();
    assert.match(htmlUnreachable, /سرویس موقتاً در دسترس نیست/);
    assert.doesNotMatch(htmlUnreachable, /کلینیک روان‌شناسی و مشاوره مهر/);
    assert.doesNotMatch(htmlUnreachable, /دکتر علوی/);
  } finally {
    child.kill('SIGTERM');
    try {
      mockApi.close();
    } catch {
      // Mock server might already be closed
    }
  }
});
