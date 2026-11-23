import { defineMiddleware } from 'astro:middleware';
import { fetchPublicSiteResult } from './lib/site.js';

export const onRequest = defineMiddleware(async (context, next) => {
  const { url, request } = context;
  const pathname = url.pathname;

  // 1. Allow static assets and internal paths
  if (
    pathname.startsWith('/_astro') ||
    pathname.startsWith('/assets') ||
    pathname === '/favicon.ico' ||
    pathname === '/robots.txt' ||
    pathname === '/sitemap.xml'
  ) {
    return next();
  }

  // 2. Resolve public site for the incoming Host
  const result = await fetchPublicSiteResult(request);

  if (!result.ok) {
    const isNotFound = result.status === 404;
    const title = isNotFound ? '۴۰۴ - مطب یا دامنه یافت نشد' : 'سرویس موقتاً در دسترس نیست';
    const message = result.message;

    return new Response(
      `<!DOCTYPE html>
<html lang="fa" dir="rtl">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1.0" />
  <title>${title}</title>
  <style>
    body { font-family: system-ui, -apple-system, sans-serif; display: flex; align-items: center; justify-content: center; min-height: 100vh; margin: 0; background: #f8fafc; color: #1e293b; text-align: center; }
    .card { background: white; padding: 2.5rem; border-radius: 1rem; box-shadow: 0 4px 6px -1px rgb(0 0 0 / 0.1); max-width: 480px; margin: 1rem; }
    h1 { font-size: 1.5rem; margin-bottom: 0.5rem; color: ${isNotFound ? '#0f172a' : '#dc2626'}; }
    p { color: #64748b; line-height: 1.6; }
  </style>
</head>
<body>
  <div class="card">
    <h1>${title}</h1>
    <p>${message}</p>
  </div>
</body>
</html>`,
      {
        status: result.status,
        headers: {
          'Content-Type': 'text/html; charset=utf-8',
          'Cache-Control': 'private, no-store',
        },
      }
    );
  }

  // 3. Attach site to locals so pages don't need duplicate network calls
  context.locals.site = result.site;

  return next();
});
