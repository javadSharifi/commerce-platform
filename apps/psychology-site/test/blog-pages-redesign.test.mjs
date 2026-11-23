import test from 'node:test';
import assert from 'node:assert/strict';
import { spawn } from 'node:child_process';
import { once } from 'node:events';
import http from 'node:http';
import { fetchOnHost } from './http-host.mjs';
import { articleSlug, blogFixture, coverId, relatedSlug } from './blog-fixture.mjs';

test('PSY-7.7.6: published blog search, categories, featured and related posts, and strict Host isolation', async () => {
  const hostA = 'blog-a.test';
  const hostB = 'blog-b.test';
  const errorHost = 'api-error.test';
  const api = http.createServer((req, res) => {
    res.setHeader('Content-Type', 'application/json');
    if (req.url === '/v1/public/site') return res.end(JSON.stringify({
      name: req.headers.host, title: req.headers.host, template_id: 'editorial', palette_id: 'stitch-editorial',
    }));
    const data = blogFixture(req.url, { empty: req.headers.host === hostB });
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

    const listing = await page('/blog');
    assert.equal(listing.status, 200);
    assert.match(listing.cache, /public/);
    assert.match(listing.html, /<h1[^>]*>\s*مقالات و یادداشت‌های تخصصی\s*<\/h1>/);
    assert.match(listing.html, new RegExp('id="featured-article"[\\s\\S]*href="/blog/' + articleSlug + '"'));
    assert.equal(listing.html.match(new RegExp('href="/blog/' + articleSlug + '"', 'g'))?.length, 1);
    assert.match(listing.html, /type="search"[^>]*name="search"/);
    assert.match(listing.html, /href="\/booking"/);
    assert.doesNotMatch(listing.html, /متن پیش‌نویس|موضوع پنهان/);

    const category = await page('/blog?category=' + encodeURIComponent('روابط'));
    assert.match(category.cache, /no-store/);
    assert.match(category.html, new RegExp(articleSlug));
    assert.match(category.html, new RegExp(relatedSlug));
    assert.doesNotMatch(category.html, /stress-guide/);

    const search = await page('/blog?search=' + encodeURIComponent('تمركز') + '&category=' + encodeURIComponent('اضطراب'));
    assert.match(search.cache, /no-store/);
    assert.match(search.html, /stress-guide/);
    assert.doesNotMatch(search.html, new RegExp(articleSlug));
    const noMatch = await page('/blog?search=nonexistent');
    assert.match(noMatch.html, /مقاله‌ای با این جست‌وجو پیدا نشد/);

    const article = await page('/blog/' + articleSlug + '?status=draft');
    assert.equal(article.status, 200);
    assert.match(article.cache, /no-store/, 'signed stream URLs must not be cached in the HTML');
    assert.match(article.html, /بازگشت به فهرست مقالات/);
    assert.match(article.html, /نویسندهٔ مقاله، فرزانه امینی/);
    assert.match(article.html, /بررسی راهکارهای علمی و روان‌شناختی/);
    assert.match(article.html, /مقدمه‌ای بر هوش هیجانی و ارتباط سالم/);
    assert.match(article.html, new RegExp('/v1/public/media/' + coverId + '/image'));
    assert.match(article.html, /<audio[\s\S]*fixture-audio/);
    assert.match(article.html, /<video[\s\S]*fixture-video/);
    assert.match(article.html, /شنیدن پادکست آموزشی مرتبط/);
    assert.match(article.html, /مشاهده ویدئوی تشریح مدل هیجانی/);
    assert.match(article.html, new RegExp('مطالب مرتبط[\\s\\S]*href="/blog/' + relatedSlug + '"'));
    assert.doesNotMatch(article.html, /stress-guide|unauthorized-block-marker|unscreened\.png|src="(?:null)?"/);

    const noCover = await page('/blog/' + relatedSlug);
    assert.equal(noCover.status, 200);
    assert.match(noCover.html, /data-blog-cover-placeholder/);
    assert.doesNotMatch(noCover.html, /نویسندهٔ مقاله، فرزانه امینی[\s\S]*<h1/);
    for (const route of ['/blog/unknown', '/blog/hidden-draft']) {
      const missing = await page(route);
      assert.equal(missing.status, 404);
      assert.match(missing.cache, /no-store/);
    }
    const empty = await page('/blog', hostB);
    assert.equal(empty.status, 200);
    assert.match(empty.html, /هنوز مقاله‌ای منتشر نشده است/);
    assert.doesNotMatch(empty.html, new RegExp(articleSlug + '|فرزانه امینی'));
    assert.equal((await page('/blog/' + articleSlug, hostB)).status, 404);
    assert.doesNotMatch((await page('/sitemap.xml', hostB)).html, new RegExp(articleSlug));
    const sitemap = await page('/sitemap.xml');
    assert.match(sitemap.html, new RegExp(articleSlug));
    assert.doesNotMatch(sitemap.html, /hidden-draft/);
    assert.equal((await page('/blog/' + articleSlug, errorHost)).status, 503);
    const unavailable = await page('/blog', errorHost);
    assert.match(unavailable.cache, /no-store/);
    assert.match(unavailable.html, /نمایش مقالات موقتاً در دسترس نیست/);

    for (const html of [listing.html, article.html]) {
      assert.doesNotMatch(html, /<script[^>]+type="module"|color:\s*#0f172a|background:\s*#ffffff/);
    }
  } finally {
    child.kill('SIGTERM');
    api.closeAllConnections();
    await new Promise(resolve => api.close(resolve));
  }
});
