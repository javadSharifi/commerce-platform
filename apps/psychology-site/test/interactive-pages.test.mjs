import test from 'node:test';
import assert from 'node:assert';
import { spawn } from 'node:child_process';

test('PSY-5.8.3: Interactive pages migration (account/courses, appointments, booking) with no-store and PersianDatePicker Island', async () => {
  const port = 4395;
  const child = spawn('node', ['./dist/server/entry.mjs'], {
    env: { ...process.env, PORT: String(port), HOST: '127.0.0.1' },
    stdio: ['ignore', 'pipe', 'pipe'],
  });

  try {
    await new Promise((resolve, reject) => {
      child.stdout.on('data', (data) => {
        if (data.toString().includes('Server listening')) {
          resolve();
        }
      });
      child.on('error', reject);
      setTimeout(() => reject(new Error('timeout waiting for server to start')), 6000);
    });

    const host = 'rezaei-clinic.ir';

    // 1. Account Courses Dashboard (/account/courses)
    const resAccountCourses = await fetch(`http://127.0.0.1:${port}/account/courses`, {
      headers: { Host: host },
    });
    assert.strictEqual(resAccountCourses.status, 200);
    const ccAccountCourses = resAccountCourses.headers.get('cache-control') || '';
    assert.ok(ccAccountCourses.includes('no-store'), 'Must include no-store');
    assert.ok(ccAccountCourses.includes('private'), 'Must include private');
    const htmlAccountCourses = await resAccountCourses.text();
    assert.match(htmlAccountCourses, /دوره‌های من/);
    assert.match(htmlAccountCourses, /card bg-base-100/);
    assert.match(htmlAccountCourses, /input-bordered/);
    assert.match(htmlAccountCourses, /btn-primary/);
    assert.doesNotMatch(htmlAccountCourses, /color:\s*#0f172a/);
    assert.doesNotMatch(htmlAccountCourses, /background:\s*#ffffff/);

    // 2. Account Course Playback (/account/courses/c-demo-1)
    const resCourseViewer = await fetch(`http://127.0.0.1:${port}/account/courses/c-demo-1`, {
      headers: { Host: host },
    });
    assert.strictEqual(resCourseViewer.status, 200);
    const ccViewer = resCourseViewer.headers.get('cache-control') || '';
    assert.ok(ccViewer.includes('no-store'), 'Must include no-store');
    assert.ok(ccViewer.includes('private'), 'Must include private');
    const htmlViewer = await resCourseViewer.text();
    assert.match(htmlViewer, /پخش جلسات دوره/);
    assert.match(htmlViewer, /main-video-player/);
    assert.match(htmlViewer, /audio-player-wrapper/);
    assert.match(htmlViewer, /card bg-base-100/);
    assert.doesNotMatch(htmlViewer, /color:\s*#0f172a/);

    // 3. Private Appointment Details (/appointments/appt-demo-1)
    const resAppt = await fetch(`http://127.0.0.1:${port}/appointments/appt-demo-1?lookup_hash=hash-test-123`, {
      headers: { Host: host },
    });
    assert.strictEqual(resAppt.status, 200);
    const ccAppt = resAppt.headers.get('cache-control') || '';
    assert.ok(ccAppt.includes('no-store'), 'Must include no-store');
    assert.ok(ccAppt.includes('private'), 'Must include private');
    const htmlAppt = await resAppt.text();
    assert.match(htmlAppt, /وضعیت نوبت مشاوره/);
    assert.match(htmlAppt, /در انتظار تأیید و تماس پذیرش/);
    assert.match(htmlAppt, /<meta name="robots" content="noindex, nofollow"/);
    assert.match(htmlAppt, /card bg-base-100/);
    assert.doesNotMatch(htmlAppt, /color:\s*#0f172a/);

    // 4. Booking Page (/booking) with PersianDatePicker Island
    const resBooking = await fetch(`http://127.0.0.1:${port}/booking`, {
      headers: { Host: host },
    });
    assert.strictEqual(resBooking.status, 200);
    const ccBooking = resBooking.headers.get('cache-control') || '';
    assert.ok(ccBooking.includes('public'), 'Booking page must have public caching');
    const htmlBooking = await resBooking.text();
    assert.match(htmlBooking, /درخواست نوبت مشاوره و روان‌درمانی/);
    assert.match(htmlBooking, /انتخاب درمانگر/);
    assert.match(htmlBooking, /دکتر مریم علوی/);
    assert.match(htmlBooking, /therapist-card/);

    // Verify PersianDatePicker Astro Island
    assert.ok(htmlBooking.includes('astro-island'), 'Must render as an Astro island');
    assert.match(htmlBooking, /client="visible"/, 'Must be hydrated with client:visible');
    assert.match(htmlBooking, /booking_preferred_date/);
    // Jalali date numerals in rendered HTML segments
    assert.ok(htmlBooking.includes('۱۴۰۵') || htmlBooking.includes('1405'));

    // Verify DaisyUI tokens in booking form and modal
    assert.match(htmlBooking, /card bg-base-100/);
    assert.match(htmlBooking, /btn-primary/);
    assert.match(htmlBooking, /input-bordered/);
    assert.doesNotMatch(htmlBooking, /color:\s*#0f172a/);
    assert.doesNotMatch(htmlBooking, /background:\s*#ffffff/);
  } finally {
    child.kill('SIGTERM');
  }
});
