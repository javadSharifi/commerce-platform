import { ApiClientError, type ContentBlock, type PublicBlogPostItem } from '@psychology/api-client';
import { formatDate, publicImagePath } from './home.js';
import { publicSiteClient } from './site.js';

export const publicBlogCache = 'public, max-age=0, s-maxage=3600, stale-while-revalidate=60';

function isPublished(post: PublicBlogPostItem): boolean {
  return !('status' in post) || post.status === 'published';
}

export async function readBlogIndex(request: Request) {
  const api = publicSiteClient(request);
  if (!api) return { posts: [], unavailable: true };
  try {
    const posts = await api.listPublicBlogPosts();
    if (!Array.isArray(posts)) throw new Error('Unexpected blog list response');
    return { posts: posts.filter(isPublished), unavailable: false };
  } catch (err) {
    console.error('Public blog list unavailable', err instanceof Error ? err.name : 'UnknownError');
    return { posts: [], unavailable: true };
  }
}

export async function readBlogPost(request: Request, slug: string) {
  const api = publicSiteClient(request);
  if (!api) return { status: 503 as const };
  try {
    const post = await api.getPublicBlogPost(slug);
    if (!isPublished(post)) return { status: 404 as const };
    if (!Array.isArray(post.content_blocks)) throw new Error('Unexpected blog detail response');
    return { status: 200 as const, post };
  } catch (err) {
    if (err instanceof ApiClientError && err.status === 404) return { status: 404 as const };
    console.error('Public blog post unavailable', err instanceof Error ? err.name : 'UnknownError');
    return { status: 503 as const };
  }
}

export function normalizeBlogSearch(value: string): string {
  return value.normalize('NFKC').toLocaleLowerCase('fa-IR').replace(/[يى]/g, 'ی').replace(/ك/g, 'ک')
    .replace(/\u200c/g, ' ').replace(/\s+/g, ' ').trim();
}

export function blogCard(post: PublicBlogPostItem) {
  return {
    id: post.id, slug: encodeURIComponent(post.slug), title: post.title, excerpt: post.seo_description,
    dateLabel: formatDate(post.published_at), coverUrl: publicImagePath(post.cover_media_id),
    category: post.category,
  };
}

export async function blogBlocks(request: Request, blocks: ContentBlock[]) {
  const api = publicSiteClient(request);
  let hasStreams = false;
  const rendered = await Promise.all(blocks.map(async ({ type, data }) => {
    if (!data) return undefined;
    const text = typeof data.text === 'string' ? data.text : '';
    if (type === 'heading' || type === 'paragraph') {
      return text ? { type, text, level: Number(data.level) } : undefined;
    }
    if (!['image', 'audio', 'video'].includes(type)) return undefined;
    const id = typeof data.media_id === 'string' ? data.media_id : undefined;
    const imagePath = publicImagePath(id);
    if (!imagePath) return undefined;

    const caption = typeof data.caption === 'string' ? data.caption : '';
    if (type === 'image') return { type, src: imagePath, caption };
    if (!api || !id) return undefined;
    hasStreams = true;
    try {
      const { stream_url: src } = await api.getPublicMediaStreamUrl(id);
      return ['https:', 'http:'].includes(new URL(src).protocol) ? { type, src, caption } : undefined;
    } catch (err) {
      console.error('Public blog stream unavailable', err instanceof Error ? err.name : 'UnknownError');
      return undefined;
    }
  }));
  return { blocks: rendered.filter(block => block !== undefined), hasStreams };
}
