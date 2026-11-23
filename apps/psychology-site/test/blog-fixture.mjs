export const articleSlug = 'self-awareness-and-emotional-regulation';
export const relatedSlug = 'managing-performance-anxiety';
export const coverId = 'b58e9fea-6465-40e2-a459-a9111a19b686';
export const videoId = '67482ad6-921b-433a-99b7-cdfd77c19e43';
export const audioId = 'ebd3e892-f695-4450-a6b3-aa0221608317';

const lead = {
  id: 'article-a', slug: articleSlug, title: 'نقش خودآگاهی و تنظیم هیجان در روابط بین‌فردی',
  seo_description: 'بررسی راهکارهای علمی و روان‌شناختی', author_name: 'نویسندهٔ مقاله، فرزانه امینی',
  category: 'روابط', is_featured: true, cover_media_id: coverId, published_at: '2026-11-15T00:00:00Z',
};
const related = {
  id: 'article-related', slug: relatedSlug, title: 'مقالهٔ هم‌موضوع', seo_description: 'تمرین‌های ارتباطی',
  category: 'روابط', published_at: '2026-11-16T00:00:00Z',
};
const articles = [related, lead, {
  id: 'article-b', slug: 'stress-guide', title: 'کاهش استرس محیط کار', seo_description: 'تکنیک تمرکز در جلسه',
  category: 'اضطراب', published_at: '2026-11-14T00:00:00Z',
}];
const draft = { ...lead, slug: 'hidden-draft', title: 'متن پیش‌نویس محرمانه', status: 'draft', category: 'موضوع پنهان' };

export function blogFixture(path, { empty = false, withMedia = true } = {}) {
  if (path === '/v1/public/blog') return empty ? [] : [...articles, draft];
  if (empty) return undefined;
  if (path === `/v1/public/blog/${articleSlug}`) {
    return {
      ...lead,
      content_blocks: [
        { type: 'heading', data: { level: 2, text: 'مقدمه‌ای بر هوش هیجانی و ارتباط سالم' } },
        { type: 'image', data: { media_id: coverId, caption: 'شرح تصویر منتشرشده' } },
        { type: 'image', data: { url: 'https://invalid.example/unscreened.png' } },
        { type: 'video', data: { media_id: videoId, caption: 'مشاهده ویدئوی تشریح مدل هیجانی' } },
        { type: 'audio', data: { media_id: audioId, caption: 'شنیدن پادکست آموزشی مرتبط' } },
        { type: 'script', data: { text: 'unauthorized-block-marker' } },
      ].filter(block => withMedia || block.type === 'heading'),
    };
  }
  if (path === '/v1/public/blog/hidden-draft') return draft;
  if (path === `/v1/public/blog/${relatedSlug}`) return { ...related, content_blocks: [] };
  if (path === `/v1/public/media/${videoId}/stream-url`) return { stream_url: 'https://media.example.test/fixture-video' };
  if (path === `/v1/public/media/${audioId}/stream-url`) return { stream_url: 'https://media.example.test/fixture-audio' };
}
