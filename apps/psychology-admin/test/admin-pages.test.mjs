import test from 'node:test';
import assert from 'node:assert';
import { spawn } from 'node:child_process';

test('PSY-5.8.4: Admin panel pages migration to shared UI and DaisyUI with no-store and noindex', async () => {
  const port = 4403;
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

    const routes = [
      { path: '/', label: 'داشبورد و شاخص‌های کلینیک', check: /شاخص‌های کلان کلینیک/ },
      { path: '/workshops', label: 'مدیریت کارگاه‌ها', check: /فهرست کارگاه‌های آموزشی کلینیک/ },
      { path: '/courses', label: 'مدیریت دوره‌ها', check: /مدیریت دوره‌های ضبط‌شده/ },
      { path: '/blog', label: 'مدیریت بلاگ', check: /مدیریت مقالات و بلاگ/ },
      { path: '/media', label: 'مدیریت رسانه‌ها', check: /سهمیه فضای ابری کلینیک/ },
      { path: '/payments', label: 'پیگیری بازپرداخت‌ها', check: /صف سفارش‌های نیازمند استرداد وجه/ },
      { path: '/appointments', label: 'مدیریت نوبت‌ها', check: /مدیریت نوبت‌ها و تقویم مشاوره/ },
      { path: '/settings', label: 'تنظیمات مطب', check: /تنظیمات قالب و هویت بصری/ },
    ];

    for (const route of routes) {
      const res = await fetch(`http://127.0.0.1:${port}${route.path}`);
      assert.strictEqual(res.status, 200, `Route ${route.path} must return 200`);

      const cacheControl = res.headers.get('cache-control') || '';
      assert.ok(cacheControl.includes('no-store'), `${route.path} must have no-store in Cache-Control`);
      assert.ok(cacheControl.includes('private'), `${route.path} must have private in Cache-Control`);

      const html = await res.text();
      assert.match(html, route.check, `${route.path} must match check regex`);
      assert.match(html, /<meta name="robots" content="noindex, nofollow"/, `${route.path} must have robots noindex`);

      // Verify DaisyUI tokens and clean semantic structure
      assert.match(html, /bg-base-200/, `${route.path} must use bg-base-200 layout`);
      assert.match(html, /text-base-content/, `${route.path} must use text-base-content`);
      assert.doesNotMatch(html, /color:\s*#0f172a/, `${route.path} must not contain hardcoded #0f172a color`);
      assert.doesNotMatch(html, /background-color:\s*#f1f5f9/, `${route.path} must not contain legacy #f1f5f9 background style`);
    }
  } finally {
    child.kill('SIGTERM');
  }
});
