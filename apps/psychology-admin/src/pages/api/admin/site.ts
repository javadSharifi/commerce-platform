import type { APIRoute } from 'astro';
import { PsychologyApiClient, ApiClientError } from '@psychology/api-client';

export const GET: APIRoute = async ({ request, cookies }) => {
  const apiBase =
    process.env.INTERNAL_API_URL ||
    import.meta.env.INTERNAL_API_URL ||
    process.env.PUBLIC_API_URL ||
    import.meta.env.PUBLIC_API_URL ||
    'http://localhost:8080';
  const sessionToken = cookies.get('session_token')?.value || '';
  const tenantId = cookies.get('tenant_id')?.value || '';
  const host = request.headers.get('host') || 'localhost:3001';

  try {
    const client = new PsychologyApiClient({
      baseUrl: apiBase,
      headers: {
        Cookie: `session_token=${sessionToken}`,
        'X-Tenant-ID': tenantId,
        Host: host,
        'X-Forwarded-Host': host,
      },
    });
    const data = await client.getAdminSite();
    return new Response(JSON.stringify(data), {
      status: 200,
      headers: {
        'Content-Type': 'application/json',
        'Cache-Control': 'private, no-store',
      },
    });
  } catch {
    // API offline in dev/preview - return fallback defaults
  }

  // Fallback defaults
  const fallback = {
    title: 'دکتر مریم علوی - روان‌درمانگر و مشاور خانواده',
    description: 'مرکز تخصصی روان‌درمانی فردی، اضطراب و زوج‌درمانی',
    phone: '۰۲۱-۸۸۸۸۸۸۸۸',
    address: 'تهران، خیابان ولیعصر، بالاتر از میدان ونک',
    primary_color: '#0284c7',
    template_id: 'minimal',
    template_version: '1.0.0',
    theme_radius: 'md',
    theme_scale: 'md',
  };

  return new Response(JSON.stringify(fallback), {
    status: 200,
    headers: {
      'Content-Type': 'application/json',
      'Cache-Control': 'private, no-store',
    },
  });
};

export const PUT: APIRoute = async ({ request, cookies }) => {
  const apiBase =
    process.env.INTERNAL_API_URL ||
    import.meta.env.INTERNAL_API_URL ||
    process.env.PUBLIC_API_URL ||
    import.meta.env.PUBLIC_API_URL ||
    'http://localhost:8080';
  const sessionToken = cookies.get('session_token')?.value || '';
  const tenantId = cookies.get('tenant_id')?.value || '';
  const host = request.headers.get('host') || 'localhost:3001';

  let body: any;
  try {
    body = await request.json();
  } catch {
    return new Response(JSON.stringify({ code: 'INVALID_JSON', message: 'فرمت درخواست نامعتبر است' }), {
      status: 400,
      headers: {
        'Content-Type': 'application/json',
        'Cache-Control': 'private, no-store',
      },
    });
  }

  try {
    const client = new PsychologyApiClient({
      baseUrl: apiBase,
      headers: {
        Cookie: `session_token=${sessionToken}`,
        'X-Tenant-ID': tenantId,
        Host: host,
        'X-Forwarded-Host': host,
      },
    });

    const data = await client.updateAdminSite(body);
    return new Response(JSON.stringify(data), {
      status: 200,
      headers: {
        'Content-Type': 'application/json',
        'Cache-Control': 'private, no-store',
      },
    });
  } catch (err: any) {
    if (err instanceof ApiClientError) {
      return new Response(
        JSON.stringify({
          code: err.code || 'API_ERROR',
          message: err.message,
          details: err.details,
        }),
        {
          status: err.status || 500,
          headers: {
            'Content-Type': 'application/json',
            'Cache-Control': 'private, no-store',
          },
        },
      );
    }

    // Network / connection error or main API service unavailable: return real 502 (AP-04 / AP-11)
    return new Response(
      JSON.stringify({
        code: 'BAD_GATEWAY',
        message: 'عدم دسترسی به سرویس اصلی',
      }),
      {
        status: 502,
        headers: {
          'Content-Type': 'application/json',
          'Cache-Control': 'private, no-store',
        },
      },
    );
  }
};
