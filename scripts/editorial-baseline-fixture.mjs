export const acceptance = process.env.PSY_EDITORIAL_ACCEPTANCE === '1';
export const hostA = 'editorial-a.test';
export const hostB = 'editorial-b.test';
export const emptyCover = '/_baseline/empty-cover.svg';
export const clock = '2026-11-23T08:00:00Z';

const name = 'محمد جواد شریفی';
const homeTitle = 'فضایی امن برای شناخت عمیق‌تر خود و ساختن روابطی سالم‌تر';
const courseTitle = 'دوره جامع مدیریت اضطراب و نشخوار فکری';
const articleTitle = 'چگونه در روابط عاطفی مرزهای سالم بگذاریم بدون آنکه احساس گناه کنیم؟';
const courseSlug = 'stress-and-rumination';
const articleSlug = 'healthy-boundaries';

const site = {
  name, title: homeTitle,
  description: 'با تکیه بر روان‌درمانی مبتنی بر شواهد و گفت‌وگویی همدلانه، همراه شما هستم تا الگوهای تکراری ذهن و رابطه را بشناسید و با آرامش بیشتری مسیر زندگی خود را بازآفرینی کنید.',
  phone: '', address: 'تهران، خیابان شریعتی، کلینیک تخصصی روانشناسی',
  template_id: 'editorial', template_version: '1.0.0', palette_id: acceptance ? 'stitch-editorial' : 'calm-sage',
  theme_radius: 'md', theme_scale: 'md',
  ...(acceptance && { home_profile: {
    introduction: 'من محمد جواد شریفی هستم. در جلسات حضوری همراه شما هستم تا الگوهای ذهن و رابطه را بهتر بشناسید.',
    specialties: ['اضطراب و استرس', 'روابط بین‌فردی', 'خودآگاهی', 'طرحواره‌درمانی'],
  } }),
};

const courses = [
  [courseSlug, courseTitle, 14800000, 12],
  ['relationship-boundaries', 'هنر مرزبندی سالم در روابط', 8900000, 8],
  ['schema-therapy', 'شناخت و درمان طرحواره‌های ناسازگار', 16500000, 14],
].map(([slug, title, price_rials, lessons_count]) => ({
  id: slug, slug, title, price_rials, lessons_count,
  instructor_name: name, status: 'published', cover_url: emptyCover,
  ...(acceptance && { topic: slug === courseSlug ? 'اضطراب' : 'روابط' }),
}));

const posts = [
  [articleSlug, articleTitle, 'چطور بدون احساس گناه مسئولیت احساسات و انتخاب‌های خودمان را بپذیریم؟'],
  ['guilt-and-criticism', 'تفاوت احساس گناه واقعی با صدای سرزنشگر درونی', 'چگونه میان مسئولیت‌پذیری و سرزنش خود تفاوت بگذاریم؟'],
  ['night-rumination', 'چرا ذهن ما در سکوت شب دچار نشخوار فکری می‌شود؟', 'نگاهی به چرخهٔ فکرهای تکراری و مواجهه با آن.'],
  ['body-and-anxiety', 'بدن ما پیش از ذهن اضطراب را فریاد می‌زند', 'نشانه‌های بدنی و ارتباط آن‌ها با تجربهٔ اضطراب.'],
  ['relationship-conflict', 'چرا در تعارض‌های عاطفی سکوت می‌کنیم؟', 'دربارهٔ واکنش‌های رایج در گفت‌وگوهای دشوار.'],
  ['abandonment-schema', 'طرحواره رهاشدگی در انتخاب‌های عاطفی', 'تأثیر تجربه‌های پیشین بر انتخاب در روابط.'],
  ['seeking-therapy', 'چه زمانی باید به روان‌درمانگر مراجعه کنیم؟', 'نشانه‌هایی برای تصمیم‌گیری دربارهٔ دریافت کمک تخصصی.'],
].map(([slug, title, seo_description]) => ({
  id: slug, slug, title, seo_description,
  published_at: '2026-12-03T08:00:00Z', status: 'published', cover_url: emptyCover,
  ...(acceptance && { category: ['healthy-boundaries', 'relationship-conflict', 'abandonment-schema'].includes(slug) ? 'روابط' : 'سلامت روان',
    is_featured: slug === articleSlug, author_name: name }),
}));

const therapist = { id: 'sharifi-test', name, title: 'روانشناس بالینی و درمانگر', session_duration_minutes: 50 };
const slots = ['10:00', '11:00'].map((time, i) => ({
  id: `sharifi-slot-${i + 1}`, therapist_id: therapist.id,
  starts_at: `2026-11-23T${time}:00Z`, ends_at: `2026-11-23T${time.slice(0, 2)}:50:00Z`,
  status: 'available',
}));

const workshops = [{
  id: 'editorial-workshop-a', title: 'کارگاه حضوری مهارت‌های ارتباطی', instructor_name: name,
  price_rials: 8900000, starts_at: '2026-12-05T08:30:00Z', remaining_capacity: 4,
  is_registration_open: true, topic: 'روابط',
}];

export const routes = [
  ['/blog', articleTitle, 7],
  [`/blog/${articleSlug}`, articleTitle, 1],
  [`/courses/${courseSlug}`, courseTitle, 0],
  ['/courses', courseTitle, 3],
  ['/booking', name, 0],
  ['/', homeTitle, 0],
];

export const otherTitle = 'مطب دوم آزمایشی';
export const publicApi = new Map([
  [hostA, new Map([
    ['/v1/public/site', site],
    ['/v1/public/courses', courses],
    ...(acceptance ? [['/v1/public/workshops', workshops]] : []),
    [`/v1/public/courses/${courseSlug}`, {
      ...courses[0],
      description: 'یادگیری گام‌به‌گام تکنیک‌های علمی برای آرام‌سازی ذهن، کاهش نگرانی‌های مزمن و بازگرداندن تمرکز و آرامش به زندگی روزمره.',
      lessons: ['درک چرخه اضطراب', 'شناسایی نشخوار فکری', 'تمرین‌های آرام‌سازی'].map((title, i) => ({
        id: `lesson-${i + 1}`, title, display_order: i + 1, duration_seconds: 1800, is_free_preview: i === 0,
      })),
    }],
    ['/v1/public/blog', posts],
    [`/v1/public/blog/${articleSlug}`, {
      ...posts[0],
      content_blocks: [
        { type: 'paragraph', data: { text: 'بسیاری از ما در فرهنگ و خانواده‌هایی رشد کرده‌ایم که در آن «خوب بودن» مترادف با همیشه در دسترس بودن، نادیده گرفتن خستگی خود و اولویت دادن به خواسته‌های دیگران تعریف شده است.' } },
        { type: 'heading', data: { level: 2, text: 'چرا مرزبندی را با خودخواهی اشتباه می‌گیریم؟' } },
        { type: 'paragraph', data: { text: 'مرز سالم دیواری برای طرد کردن دیگران نیست؛ بلکه خطی روشن است که نشان می‌دهد مسئولیت احساسات و انتخاب‌های من کجا به پایان می‌رسد و مسئولیت طرف مقابل از کجا آغاز می‌شود.' } },
      ],
    }],
    ['/v1/public/therapists', [therapist]],
    [`/v1/public/therapists/${therapist.id}/slots`, slots],
  ])],
  [hostB, new Map([
    ['/v1/public/site', { ...site, name: otherTitle, title: otherTitle, description: otherTitle, address: '', home_profile: undefined, palette_id: 'warm-sand' }],
    ['/v1/public/blog', []],
    ...(acceptance ? [['/v1/public/workshops', []], ['/v1/public/therapists', []]] : []),
    ['/v1/public/courses', []],
  ])],
]);
