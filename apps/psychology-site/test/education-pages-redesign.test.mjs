import test from 'node:test';
import assert from 'node:assert/strict';
import { spawn } from 'node:child_process';
import { once } from 'node:events';
import http from 'node:http';
import { fetchOnHost } from './http-host.mjs';
import { cover, educationFixture, fullWorkshopId, workshopId } from './education-fixture.mjs';

test('PSY-7.7.5: Education SSR filters published records by topic, uses each Host, and reflects capacity and archive status', async () => {
  const hostA = 'rezaei-clinic.ir';
  const hostB = 'empty-clinic.test';
  const errorHost = 'api-error.test';
  const api = http.createServer((req, res) => {
    res.setHeader('Content-Type', 'application/json');
    if (req.url === '/v1/public/site') {
      return res.end(JSON.stringify({
        name: req.headers.host, title: req.headers.host, template_id: 'editorial', palette_id: 'stitch-editorial',
      }));
    }
    if (req.url === '/v1/public/workshops/invalid-id') {
      res.statusCode = 400;
      return res.end(JSON.stringify({ code: 'INVALID_ID' }));
    }
    const data = educationFixture(req.url, { empty: req.headers.host === hostB });
    res.statusCode = req.headers.host === errorHost ? 503 : data === undefined ? 404 : 200;
    res.end(JSON.stringify(data ?? {}));
  });
  await new Promise(resolve => api.listen(0, '127.0.0.1', resolve));
  const child = spawn(process.execPath, ['./dist/server/entry.mjs'], {
    cwd: new URL('../', import.meta.url),
    env: { ...process.env, PORT: '0', HOST: '127.0.0.1', INTERNAL_API_URL: 'http://127.0.0.1:' + api.address().port },
    stdio: ['ignore', 'pipe', 'inherit'],
  });

  try {
    const [chunk] = await once(child.stdout, 'data', { signal: AbortSignal.timeout(6000) });
    const port = Number(chunk.toString().match(/127\.0\.0\.1:(\d+)/)?.[1]);
    assert.ok(port);
    const page = async (path, host = hostA) => {
      const res = await fetchOnHost(`http://127.0.0.1:${port}${path}`, {
        headers: { Host: host, 'X-Forwarded-Host': hostA },
      });
      return { status: res.status, cache: res.headers.get('cache-control'), html: await res.text() };
    };

    const list = await page('/courses');
    assert.equal(list.status, 200);
    assert.match(list.cache, /public/);
    assert.match(list.html, /<h1[^>]*>دوره‌ها و کارگاه‌های آموزشی<\/h1>/);
    for (const path of ['/courses/anxiety-management', '/courses/communication-skills', '/workshops/' + workshopId, '/workshops/' + fullWorkshopId]) {
      assert.ok(list.html.includes(`href="${path}"`), path);
    }
    assert.match(list.html, new RegExp('/v1/public/media/' + cover + '/image'));
    assert.doesNotMatch(list.html, /unscreened\.png|<script\b[^>]*type="module"/);
    assert.match(list.html, /card bg-base-100/);

    const filtered = await page('/courses?topic=' + encodeURIComponent('اضطراب'));
    assert.match(filtered.cache, /no-store/);
    assert.match(filtered.html, /aria-current="true"[^>]*>اضطراب</);
    assert.match(filtered.html, /anxiety-management/);
    assert.match(filtered.html, new RegExp(workshopId));
    assert.doesNotMatch(filtered.html, /communication-skills|کارگاه تکمیل ظرفیت/);
    const noMatch = await page('/courses?topic=unknown');
    assert.match(noMatch.html, /آموزشی برای این موضوع پیدا نشد/);

    const empty = await page('/courses', hostB);
    assert.equal(empty.status, 200);
    assert.match(empty.html, /هنوز آموزشی منتشر نشده است/);
    assert.doesNotMatch(empty.html, /anxiety-management|اضطراب|کارگاه روان‌درمانی/);

    const listError = await page('/courses', errorHost);
    assert.match(listError.cache, /no-store/);
    assert.match(listError.html, /نمایش آموزش‌ها موقتاً در دسترس نیست/);
    assert.doesNotMatch(listError.html, /anxiety-management/);

    const course = await page('/courses/anxiety-management?status=archived');
    assert.equal(course.status, 200);
    assert.match(course.cache, /public/);
    assert.match(course.html, /دوره جامع مدیریت استرس و اضطراب فراگیر/);
    assert.match(course.html, /سرفصل‌ها و جلسات دوره[\s\S]*<details[\s\S]*<summary[\s\S]*جلسه اول/);
    assert.match(course.html, /پیش‌نمایش رایگان/);
    assert.match(course.html, /۱٬۲۵۰٬۰۰۰٫۱ تومان/);
    assert.match(course.html, /id="checkout-root" data-slug="anxiety-management"/);
    assert.match(course.html, /id="btn-start-checkout"/);
    assert.match(course.html, new RegExp('/v1/public/media/' + cover + '/image'));
    assert.doesNotMatch(course.html, /۸۴ نظر ثبت‌شده|۷ روزه/);
    for (const result of [await page('/courses/archived-course'), await page('/courses/anxiety-management', hostB)]) {
      assert.equal(result.status, 404);
      assert.match(result.cache, /no-store/);
      assert.doesNotMatch(result.html, /checkout-root|جلسه اول/);
    }
    assert.equal((await page('/courses/anxiety-management', errorHost)).status, 503);

    const workshops = await page('/workshops');
    assert.match(workshops.html, /کارگاه‌های تخصصی و آموزشی/);
    assert.match(workshops.html, /ظرفیت تکمیل است/);
    assert.match(workshops.html, /href="\/courses"/);
    assert.doesNotMatch(workshops.html, /communication-skills/);

    const open = await page('/workshops/' + workshopId + '?sold_out=1');
    assert.equal(open.status, 200);
    assert.match(open.html, /ثبت‌نام فعال/);
    assert.match(open.html, /id="register-form"/);
    assert.match(open.html, /id="guest_phone"/);
    const full = await page('/workshops/' + fullWorkshopId);
    assert.equal(full.status, 200);
    assert.match(full.html, /ظرفیت تکمیل است/);
    assert.doesNotMatch(full.html, /id="register-form"|id="guest_phone"/);
    assert.equal((await page('/workshops/invalid-id')).status, 404);
    assert.equal((await page('/workshops/' + workshopId, hostB)).status, 404);
    assert.equal((await page('/workshops/' + workshopId, errorHost)).status, 503);

    for (const result of [list, course, workshops, open]) {
      assert.doesNotMatch(result.html, /color:\s*#0f172a|background:\s*#ffffff/);
    }
  } finally {
    child.kill('SIGTERM');
    api.closeAllConnections();
    await new Promise(resolve => api.close(resolve));
  }
});
