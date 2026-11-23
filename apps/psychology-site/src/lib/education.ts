import { ApiClientError } from '@psychology/api-client';
import { publicSiteClient } from './site.js';

export const publicEducationCache = 'public, max-age=0, s-maxage=3600, stale-while-revalidate=60';

export async function readEducation<T>(label: string, load?: () => Promise<T>) {
  if (!load) return { status: 503 as const };
  try {
    return { status: 200 as const, data: await load() };
  } catch (err) {
    if (err instanceof ApiClientError &&
        (err.status === 404 || (err.status === 400 && err.code === 'INVALID_ID'))) {
      return { status: 404 as const };
    }
    console.error('Public education request failed', label, err instanceof Error ? err.name : 'UnknownError');
    return { status: 503 as const };
  }
}

async function requireList<T>(load: Promise<T[]>): Promise<T[]> {
  const items = await load;
  if (!Array.isArray(items)) throw new Error('Unexpected education list response');
  return items;
}

export async function getEducation(request: Request, workshopsOnly: boolean) {
  const api = publicSiteClient(request);
  const [courses, workshops] = await Promise.all([
    workshopsOnly
      ? Promise.resolve({ status: 200 as const, data: [] })
      : readEducation('courses', api && (() => requireList(api.listPublicCourses()))),
    readEducation('workshops', api && (() => requireList(api.listPublicWorkshops()))),
  ]);

  const unavailable = courses.status !== 200 || workshops.status !== 200;
  const cache = new URL(request.url).search || unavailable ? 'private, no-store' : publicEducationCache;
  return {
    cache,
    courses: courses.data?.filter(item => item.status === 'published') ?? [],
    workshops: workshops.data ?? [],
    unavailable,
  };
}
