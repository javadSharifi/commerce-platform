import type { APIRoute } from 'astro';

export const GET: APIRoute = async ({ request }) => {
  const host = request.headers.get('host') || 'localhost:3000';
  const protocol = host.includes('localhost') ? 'http' : 'https';
  const baseUrl = `${protocol}://${host}`;
  const apiBase = import.meta.env.PUBLIC_API_URL || 'http://localhost:8080';

  let blogPosts: Array<{ slug: string; published_at?: string }> = [];
  let workshops: Array<{ id: string }> = [];

  try {
    const blogRes = await fetch(`${apiBase}/public/blog`, {
      headers: { 'Host': host },
      signal: AbortSignal.timeout(1200),
    });
    if (blogRes.ok) {
      const data = await blogRes.json();
      if (Array.isArray(data)) {
        blogPosts = data;
      }
    }
  } catch {
    // API offline in dev/preview
  }

  try {
    const workshopRes = await fetch(`${apiBase}/public/workshops`, {
      headers: { 'Host': host },
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

  // If no blog posts returned from API, include default published posts for preview
  if (blogPosts.length === 0) {
    blogPosts = [
      { slug: 'self-awareness-and-emotional-regulation', published_at: '2026-11-15T10:00:00Z' },
      { slug: 'managing-performance-anxiety', published_at: '2026-11-18T14:30:00Z' },
    ];
  }

  interface SitemapUrl {
    loc: string;
    priority: string;
    changefreq: string;
    lastmod?: string;
  }

  const staticUrls: SitemapUrl[] = [
    { loc: `${baseUrl}/`, priority: '1.0', changefreq: 'daily' },
    { loc: `${baseUrl}/workshops`, priority: '0.9', changefreq: 'daily' },
    { loc: `${baseUrl}/blog`, priority: '0.9', changefreq: 'daily' },
  ];

  const blogUrls: SitemapUrl[] = blogPosts.map((post) => ({
    loc: `${baseUrl}/blog/${encodeURIComponent(post.slug)}`,
    lastmod: post.published_at ? new Date(post.published_at).toISOString().split('T')[0] : undefined,
    priority: '0.8',
    changefreq: 'weekly',
  }));

  const workshopUrls: SitemapUrl[] = workshops.map((ws) => ({
    loc: `${baseUrl}/workshops/${encodeURIComponent(ws.id)}`,
    priority: '0.8',
    changefreq: 'weekly',
  }));

  const allUrls: SitemapUrl[] = [...staticUrls, ...workshopUrls, ...blogUrls];

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
      'Cache-Control': 'public, max-age=3600',
    },
  });
};
