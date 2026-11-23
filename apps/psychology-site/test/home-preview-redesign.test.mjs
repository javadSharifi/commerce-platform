import test from 'node:test';
import assert from 'node:assert/strict';
import { spawn } from 'node:child_process';
import { once } from 'node:events';
import http from 'node:http';

const hostA = 'omid-psych.ir';
const hostB = 'ahmadi.test';
const portrait = 'b58e9fea-6465-40e2-a459-a9111a19b686';
const workshop = '7e854cb2-29d8-4283-8a97-3dc7140601bb';
const site = {
  name: 'مرکز مشاوره امید',
  title: 'کلینیک تخصصی امید',
  description: 'مشاورهٔ حضوری',
  phone: '02188776655',
  address: 'تهران، میدان ونک',
  template_id: 'editorial',
  palette_id: 'stitch-editorial',
  theme_radius: 'md',
  theme_scale: 'md',
  home_profile: {
    introduction: 'معرفی عمومی مطب امید',
    specialties: ['خدمات اضطراب امید'],
    credentials: ['مدرک عمومی مطب امید'],
    office_hours: 'یکشنبه ساعت ۱۰',
    portrait_media_id: portrait,
  },
};
const published = {
  '/v1/public/courses': [
    { id: 'c-1', slug: 'published-course-a', title: 'دورهٔ عمومی امید', status: 'published',
      price_rials: 12000001, lessons_count: 4, cover_media_id: portrait },
    { id: 'c-2', slug: 'draft-course-a', title: 'پیش‌نویس دوره', status: 'draft' },
  ],
  '/v1/public/workshops': [
    { id: workshop, title: 'کارگاه عمومی امید', instructor_name: 'مدرس امید', price_rials: 0,
      is_registration_open: true, remaining_capacity: 2, starts_at: '2027-01-10T09:30:00Z' },
  ],
  '/v1/public/blog': [
    { id: 'b-1', slug: 'published-post-a', title: 'یادداشت عمومی امید', seo_description: 'خلاصهٔ عمومی' },
    { id: 'b-2', slug: 'draft-post-a', title: 'پیش‌نویس مقاله', status: 'draft' },
  ],
};

test('Home and preview render each host’s published profile and catalog', async () => {
  const api = http.createServer((req, res) => {
    const host = req.headers.host?.split(':')[0];
    const respond = (body, status = 200) => res.writeHead(status, { 'Content-Type': 'application/json' }).end(JSON.stringify(body));
    if (req.url === '/v1/public/site') {
      if (host === hostA) return respond(site);
      if (host === hostB) return respond({
        ...site, name: 'مطب احمدی', title: 'سایت مطب احمدی', home_profile: undefined,
        phone: '', address: '', palette_id: 'ocean-trust',
      });
      return respond({}, 404);
    }
    if (req.url in published) {
      if (host === hostA) return respond(published[req.url]);
      if (host === hostB) return req.url.endsWith('/blog') ? respond({}, 503) : respond([]);
    }
    return respond({}, 404);
  });
  api.listen(0, '127.0.0.1');
  await once(api, 'listening');
  const child = spawn(process.execPath, ['./dist/server/entry.mjs'], {
    cwd: new URL('../', import.meta.url),
    env: { ...process.env, PORT: '0', HOST: '127.0.0.1', INTERNAL_API_URL: 'http://127.0.0.1:' + api.address().port },
    stdio: ['ignore', 'pipe', 'inherit'],
  });

  try {
    const [[chunk]] = await Promise.all([once(child.stdout, 'data', { signal: AbortSignal.timeout(6000) })]);
    const port = Number(chunk.toString().match(/127\.0\.0\.1:(\d+)/)?.[1]);
    assert.ok(port);
    const page = (host, path) => new Promise((resolve, reject) => {
      http.get({ host: '127.0.0.1', port, path, headers: { Host: host } }, async res => {
        try {
          const html = Buffer.concat(await Array.fromAsync(res)).toString();
          resolve({ status: res.statusCode, cache: res.headers['cache-control'], html });
        } catch (err) { reject(err); }
      }).on('error', reject);
    });

    for (const route of ['/', '/preview?template=editorial&palette=stitch-editorial']) {
      const a = await page(hostA, route);
      assert.equal(a.status, 200);
      assert.match(a.cache, route === '/' ? /s-maxage=3600/ : /private, no-store/);
      for (const value of ['معرفی عمومی مطب امید', 'خدمات اضطراب امید', 'مدرک عمومی مطب امید',
        'یکشنبه ساعت ۱۰', '02188776655', 'تهران، میدان ونک', 'دورهٔ عمومی امید',
        'کارگاه عمومی امید', 'یادداشت عمومی امید', '/courses/published-course-a',
        '/workshops/' + workshop, '/blog/published-post-a', '/v1/public/media/' + portrait + '/image']) {
        assert.ok(a.html.includes(value), route + ' omitted ' + value);
      }
      assert.match(a.html, /id="specialties-heading"[\s\S]*id="courses-heading"[\s\S]*id="workshops-heading"[\s\S]*id="about-heading"[\s\S]*id="blog-heading"[\s\S]*id="booking-cta-heading"/);
      assert.match(a.html, /href="\/booking"/);
      assert.doesNotMatch(a.html, /پیش‌نویس|مشاوره آنلاین|کارگاه عملی خودآگاهی/);
      assert.match(a.html, /۱٬۲۰۰٬۰۰۰٫۱ تومان/);

      const b = await page(hostB, route);
      assert.equal(b.status, 200);
      assert.match(b.html, /سایت مطب احمدی/);
      assert.match(b.html, /نمایش برخی محتواها موقتاً در دسترس نیست/);
      assert.match(b.html, /data-portrait-placeholder/);
      assert.doesNotMatch(b.cache, /s-maxage/);
      assert.doesNotMatch(b.html, /مطب امید|published-course-a|یادداشت عمومی امید|کارگاه عملی خودآگاهی|<img[^>]*src="(?:null|undefined)"/);
      assert.doesNotMatch(b.html, /id="(?:specialties|courses|workshops|about|blog)-heading"/);
    }
    assert.equal((await page('unverified.test', '/')).status, 404);
  } finally {
    child.kill('SIGTERM');
    api.closeAllConnections();
    await new Promise(resolve => api.close(resolve));
  }
});
