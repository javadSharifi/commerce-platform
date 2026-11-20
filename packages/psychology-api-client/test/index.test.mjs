import { test, describe } from 'node:test';
import assert from 'node:assert';
import { PsychologyApiClient, ApiClientError } from '../dist/index.js';

describe('PsychologyApiClient - Blog & Media Endpoints', () => {
  test('listPublicBlogPosts calls correct endpoint and returns published posts', async () => {
    const mockPosts = [
      {
        id: '1',
        title: 'مقاله ۱',
        slug: 'post-1',
        seo_description: 'توضیحات ۱',
        published_at: '2026-11-15T10:00:00Z',
      },
    ];

    const client = new PsychologyApiClient({
      baseUrl: 'http://localhost:8080',
      fetch: async (url, init) => {
        assert.strictEqual(url.toString(), 'http://localhost:8080/public/blog');
        return new Response(JSON.stringify(mockPosts), { status: 200 });
      },
    });

    const posts = await client.listPublicBlogPosts();
    assert.strictEqual(posts.length, 1);
    assert.strictEqual(posts[0].slug, 'post-1');
  });

  test('getPublicBlogPost returns post detail and handles 404 for drafts or unpublished posts', async () => {
    const mockPost = {
      id: '1',
      title: 'مقاله تستی',
      slug: 'test-post',
      seo_description: 'توضیحات',
      published_at: '2026-11-15T10:00:00Z',
      content_blocks: [
        { type: 'paragraph', data: { text: 'محتوای آزمایشی' } },
        { type: 'audio', data: { media_id: 'm1', caption: 'صوت' } },
        { type: 'video', data: { media_id: 'm2', caption: 'ویدئو' } },
      ],
    };

    const client = new PsychologyApiClient({
      baseUrl: 'http://localhost:8080',
      fetch: async (url, init) => {
        if (url.toString().includes('test-post')) {
          return new Response(JSON.stringify(mockPost), { status: 200 });
        }
        return new Response(JSON.stringify({ code: 'NOT_FOUND', message: 'مطلب یافت نشد' }), { status: 404 });
      },
    });

    // 1. Published post succeeds
    const post = await client.getPublicBlogPost('test-post');
    assert.strictEqual(post.title, 'مقاله تستی');
    assert.strictEqual(post.content_blocks.length, 3);

    // 2. Draft / Unpublished post returns 404
    await assert.rejects(
      async () => {
        await client.getPublicBlogPost('draft-post');
      },
      (err) => {
        assert.strictEqual(err.status, 404);
        assert.strictEqual(err.code, 'NOT_FOUND');
        return true;
      }
    );
  });

  test('getPublicMediaStreamUrl returns short-lived streaming URL with 5-min TTL', async () => {
    const client = new PsychologyApiClient({
      baseUrl: 'http://localhost:8080',
      fetch: async (url, init) => {
        assert.ok(url.toString().includes('/public/media/med-123/stream-url'));
        return new Response(
          JSON.stringify({
            stream_url: 'https://storage.parspack.com/assets/video.mp4?X-Amz-Expires=300',
            expires_in_seconds: 300,
          }),
          { status: 200 }
        );
      },
    });

    const stream = await client.getPublicMediaStreamUrl('med-123');
    assert.strictEqual(stream.expires_in_seconds, 300);
    assert.ok(stream.stream_url.includes('X-Amz-Expires=300'));
  });

  test('admin blog lifecycle: create, publish, unpublish, delete', async () => {
    let currentStatus = 'draft';

    const client = new PsychologyApiClient({
      baseUrl: 'http://localhost:8080',
      fetch: async (url, init) => {
        const u = url.toString();
        const method = init?.method || 'GET';

        if (u.endsWith('/admin/blog') && method === 'POST') {
          return new Response(
            JSON.stringify({
              id: 'new-id',
              title: 'مقاله جدید',
              slug: 'new-post',
              status: 'draft',
              content_blocks: [],
              created_at: new Date().toISOString(),
              updated_at: new Date().toISOString(),
            }),
            { status: 201 }
          );
        }

        if (u.includes('/admin/blog/new-id/publish') && method === 'POST') {
          currentStatus = 'published';
          return new Response(
            JSON.stringify({
              id: 'new-id',
              title: 'مقاله جدید',
              slug: 'new-post',
              status: 'published',
              published_at: new Date().toISOString(),
            }),
            { status: 200 }
          );
        }

        if (u.includes('/admin/blog/new-id/unpublish') && method === 'POST') {
          currentStatus = 'draft';
          return new Response(
            JSON.stringify({
              id: 'new-id',
              title: 'مقاله جدید',
              slug: 'new-post',
              status: 'draft',
              published_at: null,
            }),
            { status: 200 }
          );
        }

        if (u.includes('/admin/blog/new-id') && method === 'DELETE') {
          return new Response(JSON.stringify({ message: 'حذف شد' }), { status: 200 });
        }

        return new Response(null, { status: 404 });
      },
    });

    // 1. Create draft
    const created = await client.createAdminBlogPost({
      title: 'مقاله جدید',
      slug: 'new-post',
      seo_description: '',
      content_blocks: [],
    });
    assert.strictEqual(created.id, 'new-id');
    assert.strictEqual(created.status, 'draft');

    // 2. Publish
    const published = await client.publishAdminBlogPost('new-id');
    assert.strictEqual(published.status, 'published');
    assert.strictEqual(currentStatus, 'published');

    // 3. Unpublish
    const unpublished = await client.unpublishAdminBlogPost('new-id');
    assert.strictEqual(unpublished.status, 'draft');
    assert.strictEqual(currentStatus, 'draft');

    // 4. Delete
    const deleted = await client.deleteAdminBlogPost('new-id');
    assert.strictEqual(deleted.message, 'حذف شد');
  });

  test('admin media lifecycle: upload URL, finalize, and quota listing', async () => {
    const client = new PsychologyApiClient({
      baseUrl: 'http://localhost:8080',
      fetch: async (url, init) => {
        const u = url.toString();
        const method = init?.method || 'GET';

        if (u.endsWith('/admin/media/upload-url') && method === 'POST') {
          return new Response(
            JSON.stringify({
              upload_id: 'up-1',
              upload_url: 'https://storage.parspack.mock/staging/file.mp4',
              staging_key: 'staging/up-1_file.mp4',
              expires_at: new Date(Date.now() + 900000).toISOString(),
            }),
            { status: 201 }
          );
        }

        if (u.endsWith('/admin/media/finalize') && method === 'POST') {
          return new Response(
            JSON.stringify({
              id: 'asset-1',
              storage_key: 'assets/asset-1_file.mp4',
              original_name: 'file.mp4',
              mime_type: 'video/mp4',
              size_bytes: 52428800,
              access_class: 'public',
              validation_status: 'verified',
              created_at: new Date().toISOString(),
            }),
            { status: 200 }
          );
        }

        if (u.endsWith('/admin/media') && method === 'GET') {
          return new Response(
            JSON.stringify({
              items: [
                {
                  id: 'asset-1',
                  storage_key: 'assets/asset-1_file.mp4',
                  original_name: 'file.mp4',
                  mime_type: 'video/mp4',
                  size_bytes: 52428800,
                  access_class: 'public',
                  validation_status: 'verified',
                  created_at: new Date().toISOString(),
                },
              ],
              quota_used_bytes: 52428800,
              quota_total_bytes: 10737418240,
            }),
            { status: 200 }
          );
        }

        return new Response(null, { status: 404 });
      },
    });

    const uploadReq = await client.requestMediaUploadUrl({
      original_name: 'file.mp4',
      mime_type: 'video/mp4',
      size_bytes: 52428800,
      access_class: 'public',
    });
    assert.strictEqual(uploadReq.upload_id, 'up-1');

    const finalized = await client.finalizeMediaUpload({ upload_id: uploadReq.upload_id });
    assert.strictEqual(finalized.id, 'asset-1');
    assert.strictEqual(finalized.validation_status, 'verified');

    const mediaList = await client.listAdminMedia();
    assert.strictEqual(mediaList.items.length, 1);
    assert.strictEqual(mediaList.quota_total_bytes, 10737418240);
    assert.strictEqual(mediaList.quota_used_bytes, 52428800);
  });
});

describe('Sitemap per Host & Unpublish verification', () => {
  function generateSitemapXML(host, publishedPosts, workshops) {
    const protocol = host.includes('localhost') ? 'http' : 'https';
    const baseUrl = `${protocol}://${host}`;

    const urls = [
      `${baseUrl}/`,
      `${baseUrl}/workshops`,
      `${baseUrl}/blog`,
      ...workshops.map((w) => `${baseUrl}/workshops/${w.id}`),
      ...publishedPosts.map((p) => `${baseUrl}/blog/${p.slug}`),
    ];

    return `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
${urls.map((loc) => `  <url><loc>${loc}</loc></url>`).join('\n')}
</urlset>`;
  }

  test('sitemap reflects published posts per host and excludes drafts', () => {
    const host = 'clinic-alavi.ir';
    const published = [{ slug: 'self-awareness' }, { slug: 'sleep-hygiene' }];
    const workshops = [{ id: 'ws-1' }];

    const xml = generateSitemapXML(host, published, workshops);

    // Verifies canonical host
    assert.ok(xml.includes('https://clinic-alavi.ir/blog/self-awareness'));
    assert.ok(xml.includes('https://clinic-alavi.ir/blog/sleep-hygiene'));
    assert.ok(xml.includes('https://clinic-alavi.ir/workshops/ws-1'));

    // Verify draft is absent
    assert.ok(!xml.includes('draft'));
  });

  test('unpublishing immediately removes post from sitemap', () => {
    const host = 'clinic-alavi.ir';
    let published = [{ slug: 'self-awareness' }, { slug: 'sleep-hygiene' }];

    let xml = generateSitemapXML(host, published, []);
    assert.ok(xml.includes('https://clinic-alavi.ir/blog/sleep-hygiene'));

    // Unpublish 'sleep-hygiene'
    published = published.filter((p) => p.slug !== 'sleep-hygiene');
    xml = generateSitemapXML(host, published, []);

    // Now absent from sitemap
    assert.ok(!xml.includes('https://clinic-alavi.ir/blog/sleep-hygiene'));
    assert.ok(xml.includes('https://clinic-alavi.ir/blog/self-awareness'));
  });
});
