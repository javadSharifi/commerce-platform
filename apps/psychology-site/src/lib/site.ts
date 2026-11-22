import { PsychologyApiClient, type PublicSiteResponse } from '@psychology/api-client';

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
};

export async function fetchPublicSite(request: Request): Promise<PublicSiteResponse> {
  const apiBase = import.meta.env.INTERNAL_API_URL || import.meta.env.PUBLIC_API_URL || 'http://localhost:8080';
  const host = request.headers.get('host') || 'localhost:3000';

  try {
    const client = new PsychologyApiClient({
      baseUrl: apiBase,
      headers: { Host: host },
    });
    const site = await client.getPublicSite();
    if (site && site.name) {
      return site;
    }
  } catch {
    // Graceful fallback to DEFAULT_SITE when API is unreachable
  }

  return DEFAULT_SITE;
}
