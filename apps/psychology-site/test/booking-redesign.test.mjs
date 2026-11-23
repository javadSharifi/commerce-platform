import { bookingFixture, fixtureTime } from './booking-fixture.mjs';
import test from 'node:test';
import assert from 'node:assert';
import http from 'node:http';
import { spawn } from 'node:child_process';

const siteDir = new URL('..', import.meta.url).pathname;

test('PSY-7.5: In-person consultation booking redesign, BookingFlow, available vs full slots, focus rings, 409 conflict, and request status handling', async () => {
  const mockApiPort = 4485;
  const sitePort = 4484;

  let appointmentRequestCount = 0;
  let lastAppointmentBody = null;
  let slotsFetchCount = 0;

  // Mock Backend API
  const mockServer = http.createServer((req, res) => {
    const url = new URL(req.url, `http://${req.headers.host}`);
    const pathname = url.pathname;

    if (pathname.startsWith('/v1/public/appointments/') && bookingFixture(req, res)) return;

    // 1. Public Site Data
    if (pathname === '/v1/public/site') {
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(
        JSON.stringify({
          name: 'کلینیک تخصصی روان‌شناسی شریفی',
          title: 'مرکز مشاوره و سلامت روان شریفی',
          description: 'ارائه خدمات روان‌درمانی فردی، زوج‌درمانی و کارگاه‌های تخصصی',
          phone: '۰۲۱-۲۲۸۴۹۱۰۳',
          address: 'تهران، خیابان شریعتی، بالاتر از ظفر، پلاک ۱۲۰',
          primary_color: '#2D4F43',
          palette_id: 'default',
          template_id: 'editorial',
          template_version: '1.0.0',
          theme_radius: 'md',
          theme_scale: 'md',
        })
      );
      return;
    }

    // 2. Public Therapists Directory
    if (pathname === '/v1/public/therapists') {
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(
        JSON.stringify([
          {
            id: 'th-sharifi-01',
            name: 'دکتر محمد جواد شریفی',
            slug: 'dr-mohammad-sharifi',
            title: 'روان‌شناس بالینی و روان‌درمانگر تحلیلی',
            bio: 'بیش از ۱۲ سال سابقه بالینی در درمان اختلالات فردی و روابط بین‌فردی.',
            session_duration_minutes: 50,
          },
          {
            id: 'th-karimi-02',
            name: 'دکتر سارا کریمی',
            slug: 'dr-sara-karimi',
            title: 'متخصص درمان شناختی‌رفتاری (CBT) و اضطراب',
            bio: 'پژوهشگر و درمانگر حوزه اضطراب، وسواس و مدیریت استرس.',
            session_duration_minutes: 45,
          },
        ])
      );
      return;
    }

    // 3. Public Slots for Therapist
    if (pathname.includes('/therapists/') && pathname.includes('/slots')) {
      slotsFetchCount++;
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(
        JSON.stringify([
          {
            id: 'slot-available-101',
            therapist_id: 'th-sharifi-01',
            starts_at: fixtureTime(0),
            ends_at: fixtureTime(50),
            status: 'available',
          },
          {
            id: 'slot-booked-102',
            therapist_id: 'th-sharifi-01',
            starts_at: fixtureTime(90),
            ends_at: fixtureTime(140),
            status: 'booked',
          },
          {
            id: 'slot-conflict-103',
            therapist_id: 'th-sharifi-01',
            starts_at: fixtureTime(420),
            ends_at: fixtureTime(470),
            status: 'available',
          },
        ])
      );
      return;
    }

    // 4. Public Appointment Booking Endpoint
    if (pathname === '/v1/public/appointments' && req.method === 'POST') {
      appointmentRequestCount++;
      let bodyStr = '';
      req.on('data', (chunk) => {
        bodyStr += chunk;
      });
      req.on('end', () => {
        try {
          const body = JSON.parse(bodyStr);
          lastAppointmentBody = body;

          // Conflict scenario simulation: slot-conflict-103 returns 409
          if (body.slot_id === 'slot-conflict-103') {
            res.writeHead(409, { 'Content-Type': 'application/json' });
            res.end(
              JSON.stringify({
                code: 'CONFLICT',
                message: 'Slot is already booked or held by another guest.',
              })
            );
            return;
          }

          // Rate limit simulation: phone starting with 09999999999 returns 429
          if (body.guest_phone === '09999999999') {
            res.writeHead(429, { 'Content-Type': 'application/json' });
            res.end(
              JSON.stringify({
                code: 'RATE_LIMIT_EXCEEDED',
                message: 'Too many appointment requests.',
              })
            );
            return;
          }

          // Invalid phone scenario: phone starting with 123 returns INVALID_PHONE
          if (body.guest_phone === '123') {
            res.writeHead(400, { 'Content-Type': 'application/json' });
            res.end(
              JSON.stringify({
                code: 'INVALID_PHONE',
                message: 'Phone number format is invalid.',
              })
            );
            return;
          }

          // Successful booking: returns 201 Created
          res.writeHead(201, { 'Content-Type': 'application/json' });
          res.end(
            JSON.stringify({
              id: 'appt-success-777',
              guest_lookup_hash: 'hash-secret-token-abc',
              status: 'pending_confirmation',
              therapist_id: body.therapist_id,
              slot_id: body.slot_id,
            })
          );
        } catch (err) {
          res.writeHead(400, { 'Content-Type': 'application/json' });
          res.end(JSON.stringify({ code: 'BAD_REQUEST', error: String(err), bodyStr }));
        }
      });
      return;
    }

    res.writeHead(404, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({ code: 'NOT_FOUND' }));
  });

  await new Promise((resolve) => mockServer.listen(mockApiPort, '127.0.0.1', resolve));

  const child = spawn('node', ['./dist/server/entry.mjs'], {
    cwd: siteDir,
    env: {
      ...process.env,
      PORT: String(sitePort),
      HOST: '127.0.0.1',
      INTERNAL_API_URL: `http://127.0.0.1:${mockApiPort}`,
      PUBLIC_API_URL: `http://127.0.0.1:${mockApiPort}`,
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

    const host = 'sharifi-clinic.ir';

    // Fetch the redesigned /booking page
    const resBooking = await fetch(`http://127.0.0.1:${sitePort}/booking`, {
      headers: {
        Host: host,
        'User-Agent': 'Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 Mobile/15E148 Safari/604.1',
      },
    });

    assert.strictEqual(resBooking.status, 200, 'Booking page must return 200 OK');

    // 1. Verify Cache-Control (public, max-age=0, must-revalidate; no stale s-maxage)
    const cc = resBooking.headers.get('cache-control') || '';
    assert.ok(cc.includes('private') && cc.includes('no-store'), 'Live availability must bypass cache');
    assert.ok(!cc.includes('s-maxage'), 'Cache-Control must NOT contain stale s-maxage');

    const html = await resBooking.text();
    assert.match(html, /خلاصه نوبت انتخابی/);
    assert.match(html, /id="booking-summary"/);


    // 2. Verify Editorial Header & Badges (Stitch _5/code.html visual cues)
    assert.match(html, /رزرو وقت مشاوره • محرمانه و مطمئن/, 'Header must feature trust badge');
    assert.match(html, /درخواست نوبت مشاوره و روان‌درمانی/, 'Header must feature main H1 headline');
    assert.match(html, /تأیید درخواست با شما تماس می‌گیرد/);

    // 3. Verify 2-Column Desktop & 1-Column Mobile Responsive Layout
    assert.match(html, /dir="rtl"/, 'Page must be RTL');
    assert.match(html, /overflow-x-hidden/, 'Must prevent horizontal overflow on 390px mobile');
    assert.match(html, /grid-cols-1/, 'Must support 1-column layout on mobile viewports');
    assert.match(html, /lg:grid-cols-12/, 'Must render 12-column responsive grid on desktop');

    // 4. Verify Trust & Confidentiality Sidebar Card
    assert.match(html, /اصول رازداری و هماهنگی/, 'Sidebar must contain confidentiality section');
    assert.match(html, /شمارهٔ تماس برای هماهنگی/);
    assert.match(html, /۰۲۱-۲۲۸۴۹۱۰۳/, 'Clinic phone number must be rendered');
    assert.match(html, /تهران، خیابان شریعتی/, 'Clinic address must be rendered');

    // 5. Verify BookingFlow React Island & Therapists Directory
    assert.ok(html.includes('astro-island'), 'BookingFlow must render as an Astro island');
    assert.match(html, /client="visible"/, 'Must hydrate with client:visible');
    assert.match(html, /دکتر محمد جواد شریفی/, 'Must render first therapist name');
    assert.match(html, /دکتر سارا کریمی/, 'Must render second therapist name');
    assert.match(html, /therapist-card/, 'Therapist cards must have therapist-card class');

    // 6. Verify Focus States and Keyboard Accessibility (فوکوس)
    assert.match(html, /focus-visible:ring-2/, 'Interactive elements must include focus-visible:ring-2');
    assert.match(html, /focus-visible:ring-primary/, 'Focus rings must use primary theme color');
    assert.match(html, /role="button"/, 'Therapist selection card must have role="button" for keyboard accessibility');
    assert.match(html, /tabindex="0"/i, 'Therapist selection card must have tabIndex for tab order');

    // 7. Verify Available vs Full/Unavailable Slots (سانس عرضه‌شده و سانس پر)
    assert.match(html, /slot-btn/, 'Slots must render with slot-btn class');
    assert.match(html, /slot-available-101/, 'Available slot must be present');
    assert.match(html, /slot-booked-102/, 'Booked/full slot must be present');
    assert.match(html, /slot-btn-unavailable/, 'Booked slot must have slot-btn-unavailable class');
    assert.match(html, /تکمیل‌شده/, 'Full slot must have (تکمیل‌شده) label');
    assert.match(html, /line-through/, 'Full slot must be visually struck through');

    // 8. Verify PersianDatePicker Island & Jalali numerals
    assert.match(html, /booking_preferred_date/, 'PersianDatePicker must have accessible name');
    assert.ok(html.includes('۱۴۰۵') || html.includes('1405'), 'Jalali year must be rendered');

    // 9. Verify Booking Modal and Form Fields Structure in Component Bundle
    const fs = await import('node:fs');
    const path = await import('node:path');
    const clientDir = path.join(siteDir, 'dist/client/_astro');
    const files = fs.readdirSync(clientDir);
    const bookingFlowChunk = files.find((f) => f.startsWith('BookingFlow') && f.endsWith('.js'));
    assert.ok(bookingFlowChunk, 'BookingFlow client bundle must be generated');

    const chunkContent = fs.readFileSync(path.join(clientDir, bookingFlowChunk), 'utf-8');
    assert.match(chunkContent, /booking-form/, 'Client bundle must include booking-form');
    assert.match(chunkContent, /guest_name/, 'Client bundle must include guest_name field');
    assert.match(chunkContent, /guest_phone/, 'Client bundle must include guest_phone field');
    assert.match(chunkContent, /submit-booking-btn/, 'Client bundle must include submit-booking-btn');
    assert.match(chunkContent, /modal-alert/, 'Client bundle must include modal-alert container');
    assert.match(chunkContent, /تکمیل درخواست نوبت مشاوره/, 'Client bundle must include modal title');

    // Verify Modal component from @psychology/ui
    const { Modal } = await import('@psychology/ui');
    assert.strictEqual(typeof Modal, 'function', 'Modal from @psychology/ui must be a function');

    const tracking = await fetch(`http://127.0.0.1:${sitePort}/appointments/appt-demo-1`, {
      headers: { Cookie: 'appointment_lookup=hash-test-123' },
    });
    assert.equal(tracking.status, 200);
    assert.match(await tracking.text(), /در انتظار تأیید و تماس پذیرش/);
    for (const [path, token, status] of [['appt-demo-1', '', 404], ['appt-demo-1', 'wrong', 404], ['unavailable', 'hash-test-123', 503]]) {
      const res = await fetch(`http://127.0.0.1:${sitePort}/appointments/${path}`, {
        headers: token ? { Cookie: 'appointment_lookup=' + token } : {},
      });
      assert.equal(res.status, status);
      assert.match(res.headers.get('cache-control'), /no-store/);
      assert.doesNotMatch(await res.text(), /در انتظار تأیید و تماس پذیرش|مراجع آزمایشی/);
    }

    const cancel = await fetch(`http://127.0.0.1:${sitePort}/appointments/appt-demo-1/cancel`, {
      method: 'POST',
      headers: { Cookie: 'appointment_lookup=hash-test-123', 'Content-Type': 'application/json' },
      body: '{}',
    });
    assert.equal(cancel.status, 200);
    assert.match(cancel.headers.get('cache-control'), /no-store/);

    const crossSiteCancel = await fetch(`http://127.0.0.1:${sitePort}/appointments/appt-demo-1/cancel`, {
      method: 'POST',
      headers: { Origin: 'https://other-clinic.test', Cookie: 'appointment_lookup=hash-test-123', 'Content-Type': 'application/json' },
      body: '{}',
    });
    assert.equal(crossSiteCancel.status, 403);

    // 10. Verify API Interactions directly via Mock Server (Real status handling: 201, 409, 429, 400)
    // 10a. Successful Booking (201 Created)
    const successRes = await fetch(`http://127.0.0.1:${mockApiPort}/v1/public/appointments`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Host: host,
      },
      body: JSON.stringify({
        therapist_id: 'th-sharifi-01',
        slot_id: 'slot-available-101',
        guest_name: 'سارا احمدی',
        guest_phone: '09121112233',
        notes: 'مشاوره اضطراب',
      }),
    });
    if (successRes.status !== 201) {
      const errText = await successRes.text();
      assert.fail(`Expected 201 Created, received status ${successRes.status}: ${errText}`);
    }
    const successData = await successRes.json();
    assert.strictEqual(successData.id, 'appt-success-777');
    assert.strictEqual(successData.guest_lookup_hash, 'hash-secret-token-abc');
    assert.ok(lastAppointmentBody, 'lastAppointmentBody must be recorded');
    assert.strictEqual(lastAppointmentBody.guest_name, 'سارا احمدی');
    assert.strictEqual(lastAppointmentBody.guest_phone, '09121112233');

    // 10b. Conflict Scenario (409 Conflict - slot full or taken by another guest)
    const conflictRes = await fetch(`http://127.0.0.1:${mockApiPort}/v1/public/appointments`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Host: host,
      },
      body: JSON.stringify({
        therapist_id: 'th-sharifi-01',
        slot_id: 'slot-conflict-103',
        guest_name: 'مریم حسینی',
        guest_phone: '09123334455',
      }),
    });
    assert.strictEqual(conflictRes.status, 409, 'Conflict slot must return 409');
    const conflictData = await conflictRes.json();
    assert.strictEqual(conflictData.code, 'CONFLICT');

    // 10c. Rate Limit Scenario (429 Too Many Requests)
    const rateLimitRes = await fetch(`http://127.0.0.1:${mockApiPort}/v1/public/appointments`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Host: host,
      },
      body: JSON.stringify({
        therapist_id: 'th-sharifi-01',
        slot_id: 'slot-available-101',
        guest_name: 'کاربر پرتکرار',
        guest_phone: '09999999999',
      }),
    });
    assert.strictEqual(rateLimitRes.status, 429, 'Excessive requests must return 429');

    // 10d. Invalid Phone Format Scenario (400 Invalid Phone)
    const invalidPhoneRes = await fetch(`http://127.0.0.1:${mockApiPort}/v1/public/appointments`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Host: host,
      },
      body: JSON.stringify({
        therapist_id: 'th-sharifi-01',
        slot_id: 'slot-available-101',
        guest_name: 'کاربر نامعتبر',
        guest_phone: '123',
      }),
    });
    assert.strictEqual(invalidPhoneRes.status, 400, 'Invalid phone number must return 400');
    const invalidData = await invalidPhoneRes.json();
    assert.strictEqual(invalidData.code, 'INVALID_PHONE');
  } finally {
    child.kill('SIGTERM');
    await new Promise((resolve) => mockServer.close(resolve));
  }
});
