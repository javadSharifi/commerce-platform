import type { APIRoute } from 'astro';

export const GET: APIRoute = async ({ request, cookies }) => {
  const apiBase = import.meta.env.INTERNAL_API_URL || import.meta.env.PUBLIC_API_URL || 'http://localhost:8080';
  const sessionToken = cookies.get('session_token')?.value || '';
  const tenantId = cookies.get('tenant_id')?.value || '';
  const host = request.headers.get('host') || 'localhost:3001';

  try {
    const res = await fetch(`${apiBase}/admin/site`, {
      headers: {
        Cookie: `session_token=${sessionToken}`,
        'X-Tenant-ID': tenantId,
        Host: host,
      },
      signal: AbortSignal.timeout(2000),
    });

    if (res.ok) {
      const data = await res.json();
      return new Response(JSON.stringify(data), {
        status: 200,
        headers: {
          'Content-Type': 'application/json',
          'Cache-Control': 'private, no-store',
        },
      });
    }
  } catch {
    // API offline in dev/preview
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
  const apiBase = import.meta.env.INTERNAL_API_URL || import.meta.env.PUBLIC_API_URL || 'http://localhost:8080';
  const sessionToken = cookies.get('session_token')?.value || '';
  const tenantId = cookies.get('tenant_id')?.value || '';
  const host = request.headers.get('host') || 'localhost:3001';

  let body: any;
  try {
    body = await request.json();
  } catch {
    return new Response(JSON.stringify({ code: 'INVALID_JSON', message: 'فرمت درخواست نامعتبر است' }), {
      status: 400,
      headers: { 'Content-Type': 'application/json' },
    });
  }

  try {
    const res = await fetch(`${apiBase}/admin/site`, {
      method: 'PUT',
      headers: {
        'Content-Type': 'application/json',
        Cookie: `session_token=${sessionToken}`,
        'X-Tenant-ID': tenantId,
        Host: host,
      },
      body: JSON.stringify(body),
      signal: AbortSignal.timeout(3000),
    });

    const data = await res.json();
    return new Response(JSON.stringify(data), {
      status: res.status,
      headers: {
        'Content-Type': 'application/json',
        'Cache-Control': 'private, no-store',
      },
    });
  } catch {
    // In standalone mock mode without running Go server
    return new Response(
      JSON.stringify({
        ...body,
        template_version: '1.0.0',
        message: 'تنظیمات با موفقیت ذخیره شد (حالت شبیه‌سازی)',
      }),
      {
        status: 200,
        headers: {
          'Content-Type': 'application/json',
          'Cache-Control': 'private, no-store',
        },
      },
    );
  }
};
