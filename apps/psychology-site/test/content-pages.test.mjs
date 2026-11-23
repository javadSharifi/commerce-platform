import { blogFixture } from './blog-fixture.mjs';
import http from 'node:http';
import { fetchOnHost } from './http-host.mjs';
import { educationFixture } from './education-fixture.mjs';
import test from 'node:test';
import assert from 'node:assert';
import { spawn } from 'node:child_process';

test('PSY-5.8.2: Content pages migration (courses, workshops, blog) with SSR and 0 KB client JS', async () => {
  const port = 4392;
  const api = http.createServer((req, res) => {
    const data = req.url === '/v1/public/site'
      ? { name: 'کلینیک آزمایشی', title: 'کلینیک آزمایشی', template_id: 'minimal', palette_id: 'calm-sage' }
      : educationFixture(req.url) ?? blogFixture(req.url);
    res.writeHead(data === undefined ? 404 : 200, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify(data ?? {}));
  });
  await new Promise(resolve => api.listen(0, '127.0.0.1', resolve));
  const child = spawn('node', ['./dist/server/entry.mjs'], {
    env: { ...process.env, PORT: String(port), HOST: '127.0.0.1', INTERNAL_API_URL: 'http://127.0.0.1:' + api.address().port },
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

    const host = 'rezaei-clinic.ir';

    // 1. Courses Listing (/courses)
    const resCourses = await fetchOnHost(`http://127.0.0.1:${port}/courses`, {
      headers: { Host: host },
    });
    assert.strictEqual(resCourses.status, 200);
    assert.ok(resCourses.headers.get('cache-control')?.includes('public'));
    const htmlCourses = await resCourses.text();
    assert.match(htmlCourses, /دوره‌ها و کارگاه‌های آموزشی/);
    assert.match(htmlCourses, /دوره جامع مدیریت استرس و اضطراب فراگیر/);
    assert.match(htmlCourses, /card bg-base-100/);
    assert.match(htmlCourses, /text-primary/);
    assert.match(htmlCourses, /تومان/);
    // Ensure no hardcoded hex color inline styles remain in content
    assert.doesNotMatch(htmlCourses, /color:\s*#0f172a/);
    assert.doesNotMatch(htmlCourses, /background:\s*#ffffff/);

    // 2. Course Detail (/courses/anxiety-management)
    const resCourseDetail = await fetchOnHost(`http://127.0.0.1:${port}/courses/anxiety-management`, {
      headers: { Host: host },
    });
    assert.strictEqual(resCourseDetail.status, 200);
    assert.ok(resCourseDetail.headers.get('cache-control')?.includes('public'));
    const htmlCourseDetail = await resCourseDetail.text();
    assert.match(htmlCourseDetail, /دوره جامع مدیریت استرس و اضطراب فراگیر/);
    assert.match(htmlCourseDetail, /سرفصل‌ها و جلسات دوره/);
    assert.match(htmlCourseDetail, /جلسه اول: آشنایی با مکانیسم زیستی استرس/);
    assert.match(htmlCourseDetail, /پیش‌نمایش رایگان/);
    assert.match(htmlCourseDetail, /card bg-base-100/);
    assert.match(htmlCourseDetail, /input-bordered/);
    assert.match(htmlCourseDetail, /btn-primary/);
    assert.doesNotMatch(htmlCourseDetail, /color:\s*#0f172a/);
    assert.doesNotMatch(htmlCourseDetail, /background:\s*#ffffff/);

    // 3. Workshops Listing (/workshops)
    const resWorkshops = await fetchOnHost(`http://127.0.0.1:${port}/workshops`, {
      headers: { Host: host },
    });
    assert.strictEqual(resWorkshops.status, 200);
    assert.ok(resWorkshops.headers.get('cache-control')?.includes('public'));
    const htmlWorkshops = await resWorkshops.text();
    assert.match(htmlWorkshops, /کارگاه‌های تخصصی و آموزشی/);
    assert.match(htmlWorkshops, /کارگاه روان‌درمانی تحلیلی و تنظیم هیجان/);
    assert.match(htmlWorkshops, /card bg-base-100/);
    assert.match(htmlWorkshops, /badge/);
    assert.match(htmlWorkshops, /تومان/);
    assert.doesNotMatch(htmlWorkshops, /color:\s*#0f172a/);
    assert.doesNotMatch(htmlWorkshops, /background:\s*#ffffff/);

    // 4. Workshop Detail (/workshops/f47ac10b-58cc-4372-a567-0e02b2c3d479)
    const resWorkshopDetail = await fetchOnHost(
      `http://127.0.0.1:${port}/workshops/f47ac10b-58cc-4372-a567-0e02b2c3d479`,
      { headers: { Host: host } }
    );
    assert.strictEqual(resWorkshopDetail.status, 200);
    assert.ok(resWorkshopDetail.headers.get('cache-control')?.includes('public'));
    const htmlWorkshopDetail = await resWorkshopDetail.text();
    assert.match(htmlWorkshopDetail, /کارگاه روان‌درمانی تحلیلی و تنظیم هیجان/);
    assert.match(htmlWorkshopDetail, /فرم ثبت‌نام آنلاین در کارگاه/);
    assert.match(htmlWorkshopDetail, /card bg-base-100/);
    assert.match(htmlWorkshopDetail, /badge-success/);
    assert.match(htmlWorkshopDetail, /input-bordered/);
    assert.match(htmlWorkshopDetail, /btn-primary/);
    assert.doesNotMatch(htmlWorkshopDetail, /color:\s*#0f172a/);

    // 5. Workshop Registration Status (/workshops/registrations/reg-test-123)
    const resRegistration = await fetchOnHost(
      `http://127.0.0.1:${port}/workshops/registrations/reg-test-123`,
      { headers: { Host: host } }
    );
    assert.strictEqual(resRegistration.status, 200);
    assert.strictEqual(resRegistration.headers.get('cache-control'), 'private, no-store');
    const htmlRegistration = await resRegistration.text();
    assert.match(htmlRegistration, /وضعیت ثبت‌نام کارگاه/);
    assert.match(htmlRegistration, /تأییدشده و قطعی/);
    assert.match(htmlRegistration, /badge-success/);
    assert.match(htmlRegistration, /card bg-base-100/);
    assert.doesNotMatch(htmlRegistration, /color:\s*#0f172a/);

    // 6. Blog Listing (/blog)
    const resBlog = await fetchOnHost(`http://127.0.0.1:${port}/blog`, {
      headers: { Host: host },
    });
    assert.strictEqual(resBlog.status, 200);
    assert.ok(resBlog.headers.get('cache-control')?.includes('public'));
    const htmlBlog = await resBlog.text();
    assert.match(htmlBlog, /مقالات و یادداشت‌های تخصصی/);
    assert.match(htmlBlog, /نقش خودآگاهی و تنظیم هیجان در روابط بین‌فردی/);
    assert.match(htmlBlog, /card bg-base-100/);
    assert.match(htmlBlog, /text-primary/);
    assert.doesNotMatch(htmlBlog, /color:\s*#0f172a/);
    assert.doesNotMatch(htmlBlog, /background:\s*#ffffff/);

    // 7. Blog Detail (/blog/self-awareness-and-emotional-regulation)
    const resBlogDetail = await fetchOnHost(
      `http://127.0.0.1:${port}/blog/self-awareness-and-emotional-regulation`,
      { headers: { Host: host } }
    );
    assert.strictEqual(resBlogDetail.status, 200);
    assert.ok(resBlogDetail.headers.get('cache-control')?.includes('no-store'));
    const htmlBlogDetail = await resBlogDetail.text();
    assert.match(htmlBlogDetail, /نقش خودآگاهی و تنظیم هیجان در روابط بین‌فردی/);
    assert.match(htmlBlogDetail, /مقدمه‌ای بر هوش هیجانی و ارتباط سالم/);
    assert.match(htmlBlogDetail, /<audio/);
    assert.match(htmlBlogDetail, /<video/);
    assert.match(htmlBlogDetail, /card bg-base-100/);
    assert.doesNotMatch(htmlBlogDetail, /color:\s*#0f172a/);
    assert.doesNotMatch(htmlBlogDetail, /background:\s*#ffffff/);

    // 0 KB client JS check on blog post content page: no client script tags
    assert.strictEqual(htmlBlogDetail.includes('<script type="module"'), false);
  } finally {
    child.kill('SIGTERM');
    api.closeAllConnections();
    await new Promise(resolve => api.close(resolve));
  }
});
