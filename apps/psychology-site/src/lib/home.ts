import type { BlogCardVM, CourseCardVM, WorkshopCardVM } from '../templates/view-models.js';
import { publicSiteClient } from './site.js';

const dateFormatter = new Intl.DateTimeFormat('fa-IR', { dateStyle: 'medium', timeZone: 'Asia/Tehran' });
const numberFormatter = new Intl.NumberFormat('fa-IR');

export function publicImagePath(id?: string | null): string | undefined {
  if (!id || !/^[a-f0-9]{8}(?:-[a-f0-9]{4}){3}-[a-f0-9]{12}$/i.test(id)) return undefined;
  return `/v1/public/media/${id}/image`;
}

export function formatDate(value?: string | null): string {
  if (!value) return '';
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? '' : dateFormatter.format(date);
}

export function formatPrice(value: number): string {
  if (!Number.isSafeInteger(value) || value < 0) return 'قیمت نامشخص';
  if (value === 0) return 'رایگان';
  const rials = BigInt(value);
  const remainder = rials % 10n;
  const fraction = remainder ? '٫' + numberFormatter.format(remainder) : '';
  return numberFormatter.format(rials / 10n) + fraction + ' تومان';
}

async function readList<T>(label: string, load: () => Promise<T[]>): Promise<{ items: T[]; failed: boolean }> {
  try {
    const items = await load();
    if (!Array.isArray(items)) throw new Error('Unexpected API list shape');
    return { items, failed: false };
  } catch (err) {
    console.error('Public home list unavailable', label, err instanceof Error ? err.name : 'UnknownError');
    return { items: [], failed: true };
  }
}

export async function getHomeContent(request: Request) {
  const api = publicSiteClient(request);
  if (!api) return { courses: [], workshops: [], articles: [], unavailable: false };

  const [courses, workshops, articles] = await Promise.all([
    readList('courses', () => api.listPublicCourses()),
    readList('workshops', () => api.listPublicWorkshops()),
    readList('blog', () => api.listPublicBlogPosts()),
  ]);

  const courseCards: CourseCardVM[] = courses.items.filter(item => item.status === 'published').slice(0, 3).map(item => {
    const lessons = item.lessons_count ?? item.total_lessons_count;
    return {
      id: item.id, slug: encodeURIComponent(item.slug), title: item.title,
      instructorName: item.instructor_name, priceLabel: formatPrice(item.price_rials),
      lessonsCountLabel: lessons ? numberFormatter.format(lessons) + ' جلسه' : undefined,
      coverUrl: publicImagePath(item.cover_media_id),
    };
  });
  const workshopCards: WorkshopCardVM[] = workshops.items.filter(item => Date.parse(item.starts_at) > Date.now()).slice(0, 3).map(item => ({
    id: encodeURIComponent(item.id), title: item.title, instructorName: item.instructor_name,
    priceLabel: formatPrice(item.price_rials), dateLabel: formatDate(item.starts_at),
    registrationOpen: item.is_registration_open,
    capacityLabel: item.is_registration_open ? `${numberFormatter.format(item.remaining_capacity)} جای خالی` : 'ثبت‌نام بسته است',
  }));
  const articleCards: BlogCardVM[] = articles.items
    .filter(item => !('status' in item) || item.status === 'published').slice(0, 3).map(item => ({
      id: item.id, slug: encodeURIComponent(item.slug), title: item.title,
      excerpt: item.seo_description, dateLabel: formatDate(item.published_at),
      coverUrl: publicImagePath(item.cover_media_id), category: item.category,
    }));
  return {
    courses: courseCards,
    workshops: workshopCards,
    articles: articleCards,
    unavailable: courses.failed || workshops.failed || articles.failed,
  };
}
