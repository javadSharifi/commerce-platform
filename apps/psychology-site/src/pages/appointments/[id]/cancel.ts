import type { APIRoute } from 'astro';
import { ApiClientError } from '@psychology/api-client';
import { publicSiteClient } from '../../../lib/site.js';

function response(status: number, message: string) {
  return new Response(JSON.stringify({ message }), {
    status, headers: { 'Cache-Control': 'private, no-store', 'Content-Type': 'application/json' },
  });
}

export const POST: APIRoute = async ({ params, request, cookies }) => {
  const origin = request.headers.get('Origin');
  const host = request.headers.get('Host');
  const fetchSite = request.headers.get('Sec-Fetch-Site');
  // The adapter URL can contain the upstream port behind a reverse proxy.
  if ((origin && (!URL.canParse(origin) || new URL(origin).host !== host)) ||
      (fetchSite && fetchSite !== 'same-origin') ||
      !request.headers.get('Content-Type')?.startsWith('application/json')) {
    return response(403, 'درخواست لغو نامعتبر است.');
  }
  const hash = cookies.get('appointment_lookup')?.value;
  if (!hash || !params.id) return response(404, 'نوبت یافت نشد.');
  const api = publicSiteClient(request);
  if (!api) return response(503, 'لغو نوبت موقتاً ممکن نیست.');
  try {
    await api.cancelPublicAppointment(encodeURIComponent(params.id), hash);
    return response(200, 'نوبت لغو شد.');
  } catch (err) {
    if (err instanceof ApiClientError && [400, 403, 404].includes(err.status)) {
      return response(err.status, err.message);
    }
    return response(503, 'لغو نوبت موقتاً ممکن نیست.');
  }
};
