export const cover = 'b58e9fea-6465-40e2-a459-a9111a19b686';
export const workshopId = 'f47ac10b-58cc-4372-a567-0e02b2c3d479';
export const fullWorkshopId = 'a12bc34d-58cc-4372-a567-0e02b2c3d480';

const firstCourse = {
  id: 'fixture-course-a', title: 'دوره جامع مدیریت استرس و اضطراب فراگیر', slug: 'anxiety-management',
  description: 'توضیحات دورهٔ مطب', instructor_name: 'دکتر مریم علوی', price_rials: 12500001,
  status: 'published', topic: 'اضطراب', cover_media_id: cover, lessons_count: 1,
  lessons: [{
    id: 'fixture-lesson', title: 'جلسه اول: آشنایی با مکانیسم زیستی استرس',
    description: 'توضیح جلسهٔ منتشرشده', display_order: 1, duration_seconds: 1800, is_free_preview: true,
  }],
};
const courses = [firstCourse, {
  ...firstCourse, id: 'fixture-course-b', slug: 'communication-skills', title: 'دوره مهارت‌های ارتباط مؤثر',
  topic: 'روابط', cover_media_id: null, cover_url: 'https://invalid.example/unscreened.png',
}];
const firstWorkshop = {
  id: workshopId, slug: 'analytic-psychotherapy-workshop', title: 'کارگاه روان‌درمانی تحلیلی و تنظیم هیجان',
  description: 'توضیحات کارگاه مطب', instructor_name: 'دکتر مریم علوی', starts_at: '2027-12-10T14:00:00Z',
  ends_at: '2027-12-10T18:00:00Z', registration_deadline: '2027-12-09T23:59:00Z', location: 'تهران، سالن مطب',
  price_rials: 5000001, remaining_capacity: 5, is_registration_open: true, topic: 'اضطراب',
};
const workshops = [firstWorkshop, {
  ...firstWorkshop, id: fullWorkshopId, title: 'کارگاه تکمیل ظرفیت', topic: 'روابط',
  remaining_capacity: 0, is_registration_open: false,
}];

export function educationFixture(path, { empty = false } = {}) {
  if (path === '/v1/public/courses') return empty ? [] : courses;
  if (path === '/v1/public/workshops') return empty ? [] : workshops;
  if (empty) return undefined;
  return courses.find(course => path === '/v1/public/courses/' + course.slug)
    ?? workshops.find(workshop => path === '/v1/public/workshops/' + workshop.id);
}
