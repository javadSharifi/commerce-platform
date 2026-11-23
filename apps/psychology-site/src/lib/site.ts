import { ApiClientError, PsychologyApiClient, type PublicSiteResponse } from '@psychology/api-client';

export const DEFAULT_SITE: PublicSiteResponse = {
  name: 'کلینیک روان‌شناسی و مشاوره مهر',
  title: 'دکتر علوی - روان‌درمانگر و مشاور خانواده',
  description: 'ارائه خدمات مشاوره تخصصی فردی، درمان اضطراب و افسردگی، برگزاری کارگاه‌های مهارت‌های زندگی و رشد فردی.',
  phone: '۰۲۱-۸۸۸۸۸۸۸۸',
  address: 'تهران، خیابان ولیعصر، بالاتر از میدان ونک',
  primary_color: '#0284c7',
  template_id: 'minimal',
  template_version: '1.0.0',
  theme_radius: 'md',
  theme_scale: 'md',
  palette_id: 'calm-sage',
};

export function getApiBaseUrl(): string {
  if (typeof process !== 'undefined' && process.env) {
    if (process.env.INTERNAL_API_URL) return process.env.INTERNAL_API_URL;
    if (process.env.PUBLIC_API_URL) return process.env.PUBLIC_API_URL;
  }
  if (typeof import.meta !== 'undefined' && import.meta.env) {
    if (import.meta.env.INTERNAL_API_URL) return import.meta.env.INTERNAL_API_URL;
    if (import.meta.env.PUBLIC_API_URL) return import.meta.env.PUBLIC_API_URL;
  }
  return '';
}

export type PublicSiteResult =
  | { ok: true; site: PublicSiteResponse }
  | { ok: false; status: 404; code: 'HOST_NOT_FOUND'; message: string }
  | { ok: false; status: 502 | 503; code: 'API_UNAVAILABLE'; message: string };

export async function fetchPublicSiteResult(request: Request): Promise<PublicSiteResult> {
  const apiBase = getApiBaseUrl();
  const host = request.headers.get('x-forwarded-host') || request.headers.get('host') || 'localhost:3000';

  if (!apiBase) {
    // When INTERNAL_API_URL is not configured (offline UI template tests), allow default mock site
    return { ok: true, site: DEFAULT_SITE };
  }

  try {
    const client = new PsychologyApiClient({
      baseUrl: apiBase,
      headers: {
        Host: host,
        'X-Forwarded-Host': host,
      },
    });
    const site = await client.getPublicSite();
    if (site && site.name) {
      return { ok: true, site };
    }
    return {
      ok: false,
      status: 404,
      code: 'HOST_NOT_FOUND',
      message: 'دامنه یا مطب مورد نظر یافت نشد.',
    };
  } catch (err: unknown) {
    if (err instanceof ApiClientError) {
      if (err.status === 404) {
        return {
          ok: false,
          status: 404,
          code: 'HOST_NOT_FOUND',
          message: err.message || 'دامنه یا مطب مورد نظر در سیستم ثبت نشده است.',
        };
      }
      return {
        ok: false,
        status: 502,
        code: 'API_UNAVAILABLE',
        message: 'خطا در ارتباط با سرویس پشتیبان.',
      };
    }

    return {
      ok: false,
      status: 503,
      code: 'API_UNAVAILABLE',
      message: 'سرویس موقتاً در دسترس نیست. لطفاً دقایقی دیگر تلاش کنید.',
    };
  }
}

export async function fetchPublicSite(request: Request): Promise<PublicSiteResponse> {
  const result = await fetchPublicSiteResult(request);
  if (result.ok) {
    return result.site;
  }
  throw new Error(`PublicSiteError (${result.status}): ${result.message}`);
}
