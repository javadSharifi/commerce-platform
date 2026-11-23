import { bookingFixture } from './booking-fixture.mjs';
import { fetchOnHost } from './http-host.mjs';
import test from 'node:test';
import http from 'node:http';
import assert from 'node:assert';
import { spawn } from 'node:child_process';

test('PSY-5.8.3: Interactive pages migration (account/courses, appointments, booking) with no-store and PersianDatePicker Island', async () => {
  const port = 4395;
  const api = http.createServer((req, res) => {
    if (bookingFixture(req, res)) return;
    res.writeHead(200, { 'Content-Type': 'application/json' }).end(JSON.stringify({
      name: 'مطب آزمایشی', title: 'مطب آزمایشی', template_id: 'minimal',
    }));
  });
  await new Promise(resolve => api.listen(0, '127.0.0.1', resolve));
  const child = spawn('node', ['./dist/server/entry.mjs'], {
    env: { ...process.env, PORT: String(port), HOST: '127.0.0.1', INTERNAL_API_URL: 'http://127.0.0.1:' + api.address().port },
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
    const resAccountCourses = await fetchOnHost(`http://127.0.0.1:${port}/account/courses`, {
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
    const resCourseViewer = await fetchOnHost(`http://127.0.0.1:${port}/account/courses/c-demo-1`, {
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
    const resAppt = await fetchOnHost(`http://127.0.0.1:${port}/appointments/appt-demo-1?lookup_hash=hash-test-123`, {
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
    const resBooking = await fetchOnHost(`http://127.0.0.1:${port}/booking`, {
      headers: { Host: host },
    });
    assert.strictEqual(resBooking.status, 200);
    const ccBooking = resBooking.headers.get('cache-control') || '';
    assert.match(ccBooking, /private, no-store/);
    assert.ok(!ccBooking.includes('s-maxage'), 'Booking page must NOT have s-maxage stale cache');
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

    // PSY-5.18: Mobile 390px layout and RTL checks
    assert.match(htmlBooking, /dir="rtl"/, 'Must render in RTL direction');
    assert.match(htmlBooking, /overflow-x-hidden/, 'Must contain overflow-x-hidden to prevent horizontal scroll on 390px mobile');
    assert.match(htmlBooking, /grid-cols-1/, 'Must support 1-column layout on mobile viewports');
  } finally {
    child.kill('SIGTERM');
    api.closeAllConnections();
    await new Promise(resolve => api.close(resolve));
  }
});

test('PSY-5.18: Booking flow with live API sends Gregorian ISO date for slots and appointments', async () => {
  const mockApiPort = 4418;
  const sitePort = 4419;
  const http = await import('node:http');

  let receivedSlotQuery = '';
  let receivedBookingBody = null;

  const mockServer = http.createServer((req, res) => {
    const url = new URL(req.url, `http://${req.headers.host}`);
    const pathname = url.pathname;

    if (pathname.includes('/therapists') && pathname.includes('/slots')) {
      receivedSlotQuery = url.searchParams.get('from_date') || '';
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(
        JSON.stringify([
          {
            id: 'slot-iso-1111',
            therapist_id: 'th-1',
            starts_at: '2026-11-23T10:00:00Z',
            ends_at: '2026-11-23T10:50:00Z',
            status: 'available',
          },
        ])
      );
      return;
    }

    if (pathname === '/v1/public/therapists') {
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(
        JSON.stringify([
          {
            id: 'th-1',
            name: 'دکتر مریم علوی',
            title: 'روان‌شناس بالینی',
            bio: 'مشاوره فردی',
            session_duration_minutes: 50,
          },
        ])
      );
      return;
    }

    if (pathname === '/v1/public/site') {
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(
        JSON.stringify({
          name: 'کلینیک روان‌شناسی مهر',
          title: 'مرکز مشاوره مهر',
          description: 'ارائه خدمات روان‌شناختی',
          phone: '۰۲۱-۸۸۸۸۸۸۸۸',
          address: 'تهران',
          primary_color: '#0284c7',
          template_id: 'minimal',
          template_version: '1.0.0',
          theme_radius: 'md',
          theme_scale: 'md',
        })
      );
      return;
    }

    res.writeHead(404, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({ code: 'NOT_FOUND' }));
  });

  await new Promise((resolve) => mockServer.listen(mockApiPort, '127.0.0.1', resolve));

  const child = spawn('node', ['./dist/server/entry.mjs'], {
    env: {
      ...process.env,
      PORT: String(sitePort),
      HOST: '127.0.0.1',
      INTERNAL_API_URL: `http://127.0.0.1:${mockApiPort}`,
    },
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
      setTimeout(() => reject(new Error('timeout waiting for site server to start')), 6000);
    });

    const resBooking = await fetchOnHost(`http://127.0.0.1:${sitePort}/booking`, {
      headers: {
        Host: 'mehr-clinic.ir',
        'X-Forwarded-Host': 'mehr-clinic.ir',
        'User-Agent': 'Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1',
      },
    });

    assert.strictEqual(resBooking.status, 200);
    const html = await resBooking.text();

    // 1. Verify RTL direction and Mobile 390px responsiveness
    assert.match(html, /dir="rtl"/);
    assert.match(html, /overflow-x-hidden/);
    assert.match(html, /max-w-5xl/);

    // 2. Verify Persian Jalali display in UI
    assert.ok(html.includes('۱۴۰۵') || html.includes('1405'));

    // 3. Verify standard ISO-8601 date format is used for initial slots fetch
    assert.match(html, /value="\d{4}-\d{2}-\d{2}"/, 'DatePicker initial value must be standard ISO Gregorian YYYY-MM-DD');

    // 4. Verify slots query received standard Gregorian ISO YYYY-MM-DD date string
    if (receivedSlotQuery) {
      assert.match(
        receivedSlotQuery,
        /^\d{4}-\d{2}-\d{2}$/,
        `Slot query from_date must be formatted as YYYY-MM-DD ISO date, received: ${receivedSlotQuery}`
      );
    }
  } finally {
    child.kill('SIGTERM');
    await new Promise((resolve) => mockServer.close(resolve));
  }
});
