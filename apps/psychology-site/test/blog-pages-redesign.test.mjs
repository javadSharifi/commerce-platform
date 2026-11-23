import test from 'node:test';
import assert from 'node:assert';
import { spawn } from 'node:child_process';

const siteDir = new URL('..', import.meta.url).pathname;

test('PSY-7.4: Blog routes redesign (/blog, /blog/[slug]), allowed content blocks, empty list, missing image and invalid route regression', async () => {
  const port = 4472;
  const child = spawn('node', ['./dist/server/entry.mjs'], {
    cwd: siteDir,
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
      setTimeout(() => reject(new Error('timeout waiting for server to start')), 8000);
    });

    const host = 'rezaei-clinic.ir';

    // ---------------------------------------------------------------------------------
    // 1. /blog listing
    // ---------------------------------------------------------------------------------
    const resBlog = await fetch(`http://127.0.0.1:${port}/blog`, {
      headers: { 'X-Forwarded-Host': host, Host: host },
    });
    assert.strictEqual(resBlog.status, 200);
    assert.ok(resBlog.headers.get('cache-control')?.includes('public'));
    const htmlBlog = await resBlog.text();

    assert.match(htmlBlog, /یادداشت‌ها و مقالات تخصصی/);
    assert.match(htmlBlog, /<h1[^>]*>[\s\S]*?مقالات و یادداشت‌های تخصصی[\s\S]*?<\/h1>/);
    assert.match(htmlBlog, /همه نوشته‌ها/);
    assert.match(htmlBlog, /نقش خودآگاهی و تنظیم هیجان در روابط بین‌فردی/);
    // Every CTA in the blog list points to its matching /blog/[slug]
    assert.match(htmlBlog, /href="\/blog\/self-awareness-and-emotional-regulation"/);
    assert.match(htmlBlog, /href="\/blog\/managing-performance-anxiety"/);
    // Cross-link to booking present
    assert.match(htmlBlog, /href="\/booking"/);
    // Semantic tokens used, no hardcoded colors
    assert.match(htmlBlog, /card bg-base-100/);
    assert.match(htmlBlog, /text-primary/);
    assert.doesNotMatch(htmlBlog, /color:\s*#0f172a/);
    assert.doesNotMatch(htmlBlog, /background:\s*#ffffff/);

    // ---------------------------------------------------------------------------------
    // 2. /blog empty list scenario regression
    // ---------------------------------------------------------------------------------
    const resBlogEmpty = await fetch(`http://127.0.0.1:${port}/blog?empty=1`, {
      headers: { 'X-Forwarded-Host': host, Host: host },
    });
    assert.strictEqual(resBlogEmpty.status, 200);
    const htmlBlogEmpty = await resBlogEmpty.text();
    assert.match(htmlBlogEmpty, /هنوز مقاله‌ای منتشر نشده است/);
    assert.match(htmlBlogEmpty, /به زودی یادداشت‌ها و مطالب آموزشی جدید در این بخش قرار خواهد گرفت/);
    assert.match(htmlBlogEmpty, /href="\/courses"/);
    assert.doesNotMatch(htmlBlogEmpty, /self-awareness-and-emotional-regulation/);

    // ---------------------------------------------------------------------------------
    // 3. /blog/[slug] published detail, missing image, allowed blocks, 0 KB client JS
    // ---------------------------------------------------------------------------------
    const resBlogDetail = await fetch(
      `http://127.0.0.1:${port}/blog/self-awareness-and-emotional-regulation`,
      { headers: { 'X-Forwarded-Host': host, Host: host } }
    );
    assert.strictEqual(resBlogDetail.status, 200);
    assert.ok(resBlogDetail.headers.get('cache-control')?.includes('public'));
    const htmlBlogDetail = await resBlogDetail.text();

    // Breadcrumb / back link
    assert.match(htmlBlogDetail, /href="\/blog"/);
    assert.match(htmlBlogDetail, /بازگشت به فهرست مقالات/);
    // Title, author & SEO lead
    assert.match(htmlBlogDetail, /<h1[^>]*>[\s\S]*?نقش خودآگاهی و تنظیم هیجان در روابط بین‌فردی[\s\S]*?<\/h1>/);
    assert.match(htmlBlogDetail, /روان‌شناس و مشاور بالینی/);
    assert.match(htmlBlogDetail, /بررسی راهکارهای علمی و روان‌شناختی/);

    // Allowed content blocks rendered
    assert.match(htmlBlogDetail, /مقدمه‌ای بر هوش هیجانی و ارتباط سالم/);
    assert.match(htmlBlogDetail, /<audio/);
    assert.match(htmlBlogDetail, /<video/);
    assert.match(htmlBlogDetail, /شنیدن پادکست آموزشی مرتبط/);
    assert.match(htmlBlogDetail, /مشاهده ویدئوی تشریح مدل هیجانی/);

    // Missing image scenario: cover_url is null, so no broken figure with empty/null src is rendered
    assert.doesNotMatch(htmlBlogDetail, /src="null"/);
    assert.doesNotMatch(htmlBlogDetail, /src=""/);

    // Contextual booking CTA card
    assert.match(htmlBlogDetail, /نیاز به گفت‌وگوی تخصصی درباره این موضوع دارید؟/);
    assert.match(htmlBlogDetail, /href="\/booking"/);

    // 0 KB client JS check on blog post content page: no client script tags
    assert.strictEqual(htmlBlogDetail.includes('<script type="module"'), false);
    assert.doesNotMatch(htmlBlogDetail, /color:\s*#0f172a/);
    assert.doesNotMatch(htmlBlogDetail, /background:\s*#ffffff/);

    // ---------------------------------------------------------------------------------
    // 4. /blog/[slug] invalid route scenario regression
    // ---------------------------------------------------------------------------------
    const resBlogNotFound = await fetch(
      `http://127.0.0.1:${port}/blog/non-existent-article-slug`,
      { headers: { 'X-Forwarded-Host': host, Host: host } }
    );
    assert.strictEqual(resBlogNotFound.status, 404);
    const notFoundText = await resBlogNotFound.text();
    assert.match(notFoundText, /مطلب مورد نظر یافت نشد/);

    // ---------------------------------------------------------------------------------
    // 5. /blog/[slug] non-published / draft content scenario regression
    // ---------------------------------------------------------------------------------
    const resBlogDraft = await fetch(
      `http://127.0.0.1:${port}/blog/self-awareness-and-emotional-regulation?status=draft`,
      { headers: { 'X-Forwarded-Host': host, Host: host } }
    );
    assert.strictEqual(resBlogDraft.status, 404);
  } finally {
    child.kill('SIGTERM');
  }
});
