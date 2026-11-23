import type { APIRoute } from 'astro';
import { readBlogIndex } from '../lib/blog.js';

export const GET: APIRoute = async ({ request }) => {
  const host = request.headers.get('host') || 'localhost:3000';
  const protocol = host.includes('localhost') ? 'http' : 'https';
  const baseUrl = `${protocol}://${host}`;
  const apiBase =
    process.env.INTERNAL_API_URL ||
    import.meta.env.INTERNAL_API_URL ||
    import.meta.env.PUBLIC_API_URL ||
    'http://localhost:8080';

  let courses: Array<{ slug: string; published_at?: string }> = [];
  let workshops: Array<{ id: string }> = [];
  const blogIndex = readBlogIndex(request);

  // 1. Fetch published courses
  try {
    const coursesRes = await fetch(`${apiBase}/v1/public/courses`, {
      headers: { Host: host },
      signal: AbortSignal.timeout(1200),
    });
    if (coursesRes.ok) {
      const data = await coursesRes.json();
      if (Array.isArray(data)) {
        courses = data;
      }
    }
  } catch {
    // API offline in dev/preview
  }

  // 2. Fetch active workshops
  try {
    const workshopRes = await fetch(`${apiBase}/v1/public/workshops`, {
      headers: { Host: host },
      signal: AbortSignal.timeout(1200),
    });
    if (workshopRes.ok) {
      const data = await workshopRes.json();
      if (Array.isArray(data)) {
        workshops = data;
      }
    }
  } catch {
    // API offline in dev/preview
  }

  const { posts: blogPosts, unavailable: blogUnavailable } = await blogIndex;

  interface SitemapUrl {
    loc: string;
    priority: string;
    changefreq: string;
    lastmod?: string;
  }

  const staticUrls: SitemapUrl[] = [
    { loc: `${baseUrl}/`, priority: '1.0', changefreq: 'daily' },
    { loc: `${baseUrl}/courses`, priority: '0.9', changefreq: 'daily' },
    { loc: `${baseUrl}/workshops`, priority: '0.9', changefreq: 'daily' },
    { loc: `${baseUrl}/blog`, priority: '0.9', changefreq: 'daily' },
    { loc: `${baseUrl}/booking`, priority: '0.9', changefreq: 'daily' },
  ];

  const courseUrls: SitemapUrl[] = courses.map((course) => ({
    loc: `${baseUrl}/courses/${encodeURIComponent(course.slug)}`,
    lastmod: course.published_at ? new Date(course.published_at).toISOString().split('T')[0] : undefined,
    priority: '0.8',
    changefreq: 'weekly',
  }));

  const workshopUrls: SitemapUrl[] = workshops.map((ws) => ({
    loc: `${baseUrl}/workshops/${encodeURIComponent(ws.id)}`,
    priority: '0.8',
    changefreq: 'weekly',
  }));

  const blogUrls: SitemapUrl[] = blogPosts.map((post) => ({
    loc: `${baseUrl}/blog/${encodeURIComponent(post.slug)}`,
    lastmod: post.published_at ? new Date(post.published_at).toISOString().split('T')[0] : undefined,
    priority: '0.8',
    changefreq: 'weekly',
  }));

  const allUrls: SitemapUrl[] = [
    ...staticUrls,
    ...courseUrls,
    ...workshopUrls,
    ...blogUrls,
  ];

  const xml = `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
${allUrls
  .map(
    (u) => `  <url>
    <loc>${u.loc}</loc>
    <changefreq>${u.changefreq}</changefreq>
    <priority>${u.priority}</priority>${u.lastmod ? `\n    <lastmod>${u.lastmod}</lastmod>` : ''}
  </url>`
  )
  .join('\n')}
</urlset>`.trim();

  return new Response(xml, {
    status: 200,
    headers: {
      'Content-Type': 'application/xml; charset=utf-8',
      'Cache-Control': blogUnavailable ? 'private, no-store' : 'public, max-age=3600',
    },
  });
};
