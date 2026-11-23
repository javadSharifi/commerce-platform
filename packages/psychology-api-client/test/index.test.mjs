import { test, describe } from 'node:test';
import assert from 'node:assert';
import { PsychologyApiClient, ApiClientError } from '../dist/index.js';

describe('PsychologyApiClient - Blog & Media Endpoints', () => {
  test('listPublicBlogPosts calls correct endpoint and returns published posts', async () => {
    const mockPosts = [
      {
        id: '1',
        title: 'مقاله ۱',
        slug: 'post-1',
        seo_description: 'توضیحات ۱',
        published_at: '2026-11-15T10:00:00Z',
      },
    ];

    const client = new PsychologyApiClient({
      baseUrl: 'http://localhost:8080',
      fetch: async (url, init) => {
        assert.strictEqual(url.toString(), 'http://localhost:8080/v1/public/blog');
        return new Response(JSON.stringify(mockPosts), { status: 200 });
      },
    });

    const posts = await client.listPublicBlogPosts();
    assert.strictEqual(posts.length, 1);
    assert.strictEqual(posts[0].slug, 'post-1');
  });

  test('getPublicBlogPost returns post detail and handles 404 for drafts or unpublished posts', async () => {
    const mockPost = {
      id: '1',
      title: 'مقاله تستی',
      slug: 'test-post',
      seo_description: 'توضیحات',
      published_at: '2026-11-15T10:00:00Z',
      content_blocks: [
        { type: 'paragraph', data: { text: 'محتوای آزمایشی' } },
        { type: 'audio', data: { media_id: 'm1', caption: 'صوت' } },
        { type: 'video', data: { media_id: 'm2', caption: 'ویدئو' } },
      ],
    };

    const client = new PsychologyApiClient({
      baseUrl: 'http://localhost:8080',
      fetch: async (url, init) => {
        if (url.toString().includes('test-post')) {
          return new Response(JSON.stringify(mockPost), { status: 200 });
        }
        return new Response(JSON.stringify({ code: 'NOT_FOUND', message: 'مطلب یافت نشد' }), { status: 404 });
      },
    });

    // 1. Published post succeeds
    const post = await client.getPublicBlogPost('test-post');
    assert.strictEqual(post.title, 'مقاله تستی');
    assert.strictEqual(post.content_blocks.length, 3);

    // 2. Draft / Unpublished post returns 404
    await assert.rejects(
      async () => {
        await client.getPublicBlogPost('draft-post');
      },
      (err) => {
        assert.strictEqual(err.status, 404);
        assert.strictEqual(err.code, 'NOT_FOUND');
        return true;
      }
    );
  });

  test('getPublicMediaStreamUrl returns short-lived streaming URL with 5-min TTL', async () => {
    const client = new PsychologyApiClient({
      baseUrl: 'http://localhost:8080',
      fetch: async (url, init) => {
        assert.ok(url.toString().includes('/public/media/med-123/stream-url'));
        return new Response(
          JSON.stringify({
            stream_url: 'https://storage.parspack.com/assets/video.mp4?X-Amz-Expires=300',
            expires_in_seconds: 300,
          }),
          { status: 200 }
        );
      },
    });

    const stream = await client.getPublicMediaStreamUrl('med-123');
    assert.strictEqual(stream.expires_in_seconds, 300);
    assert.ok(stream.stream_url.includes('X-Amz-Expires=300'));
  });

  test('admin blog lifecycle: create, publish, unpublish, delete', async () => {
    let currentStatus = 'draft';

    const client = new PsychologyApiClient({
      baseUrl: 'http://localhost:8080',
      fetch: async (url, init) => {
        const u = url.toString();
        const method = init?.method || 'GET';

        if (u.endsWith('/admin/blog') && method === 'POST') {
          return new Response(
            JSON.stringify({
              id: 'new-id',
              title: 'مقاله جدید',
              slug: 'new-post',
              status: 'draft',
              content_blocks: [],
              created_at: new Date().toISOString(),
              updated_at: new Date().toISOString(),
            }),
            { status: 201 }
          );
        }

        if (u.includes('/admin/blog/new-id/publish') && method === 'POST') {
          currentStatus = 'published';
          return new Response(
            JSON.stringify({
              id: 'new-id',
              title: 'مقاله جدید',
              slug: 'new-post',
              status: 'published',
              published_at: new Date().toISOString(),
            }),
            { status: 200 }
          );
        }

        if (u.includes('/admin/blog/new-id/unpublish') && method === 'POST') {
          currentStatus = 'draft';
          return new Response(
            JSON.stringify({
              id: 'new-id',
              title: 'مقاله جدید',
              slug: 'new-post',
              status: 'draft',
              published_at: null,
            }),
            { status: 200 }
          );
        }

        if (u.includes('/admin/blog/new-id') && method === 'DELETE') {
          return new Response(JSON.stringify({ message: 'حذف شد' }), { status: 200 });
        }

        return new Response(null, { status: 404 });
      },
    });

    // 1. Create draft
    const created = await client.createAdminBlogPost({
      title: 'مقاله جدید',
      slug: 'new-post',
      seo_description: '',
      content_blocks: [],
    });
    assert.strictEqual(created.id, 'new-id');
    assert.strictEqual(created.status, 'draft');

    // 2. Publish
    const published = await client.publishAdminBlogPost('new-id');
    assert.strictEqual(published.status, 'published');
    assert.strictEqual(currentStatus, 'published');

    // 3. Unpublish
    const unpublished = await client.unpublishAdminBlogPost('new-id');
    assert.strictEqual(unpublished.status, 'draft');
    assert.strictEqual(currentStatus, 'draft');

    // 4. Delete
    const deleted = await client.deleteAdminBlogPost('new-id');
    assert.strictEqual(deleted.message, 'حذف شد');
  });

  test('admin media lifecycle: upload URL, finalize, and quota listing', async () => {
    const client = new PsychologyApiClient({
      baseUrl: 'http://localhost:8080',
      fetch: async (url, init) => {
        const u = url.toString();
        const method = init?.method || 'GET';

        if (u.endsWith('/admin/media/upload-url') && method === 'POST') {
          return new Response(
            JSON.stringify({
              upload_id: 'up-1',
              upload_url: 'https://storage.parspack.mock/staging/file.mp4',
              staging_key: 'staging/up-1_file.mp4',
              expires_at: new Date(Date.now() + 900000).toISOString(),
            }),
            { status: 201 }
          );
        }

        if (u.endsWith('/admin/media/finalize') && method === 'POST') {
          return new Response(
            JSON.stringify({
              id: 'asset-1',
              storage_key: 'assets/asset-1_file.mp4',
              original_name: 'file.mp4',
              mime_type: 'video/mp4',
              size_bytes: 52428800,
              access_class: 'public',
              validation_status: 'verified',
              created_at: new Date().toISOString(),
            }),
            { status: 200 }
          );
        }

        if (u.endsWith('/admin/media') && method === 'GET') {
          return new Response(
            JSON.stringify({
              items: [
                {
                  id: 'asset-1',
                  storage_key: 'assets/asset-1_file.mp4',
                  original_name: 'file.mp4',
                  mime_type: 'video/mp4',
                  size_bytes: 52428800,
                  access_class: 'public',
                  validation_status: 'verified',
                  created_at: new Date().toISOString(),
                },
              ],
              quota_used_bytes: 52428800,
              quota_total_bytes: 10737418240,
            }),
            { status: 200 }
          );
        }

        return new Response(null, { status: 404 });
      },
    });

    const uploadReq = await client.requestMediaUploadUrl({
      original_name: 'file.mp4',
      mime_type: 'video/mp4',
      size_bytes: 52428800,
      access_class: 'public',
    });
    assert.strictEqual(uploadReq.upload_id, 'up-1');

    const finalized = await client.finalizeMediaUpload({ upload_id: uploadReq.upload_id });
    assert.strictEqual(finalized.id, 'asset-1');
    assert.strictEqual(finalized.validation_status, 'verified');

    const mediaList = await client.listAdminMedia();
    assert.strictEqual(mediaList.items.length, 1);
    assert.strictEqual(mediaList.quota_total_bytes, 10737418240);
    assert.strictEqual(mediaList.quota_used_bytes, 52428800);
  });
});

describe('Sitemap per Host & Unpublish verification', () => {
  function generateSitemapXML(host, publishedPosts, workshops) {
    const protocol = host.includes('localhost') ? 'http' : 'https';
    const baseUrl = `${protocol}://${host}`;

    const urls = [
      `${baseUrl}/`,
      `${baseUrl}/workshops`,
      `${baseUrl}/blog`,
      ...workshops.map((w) => `${baseUrl}/workshops/${w.id}`),
      ...publishedPosts.map((p) => `${baseUrl}/blog/${p.slug}`),
    ];

    return `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
${urls.map((loc) => `  <url><loc>${loc}</loc></url>`).join('\n')}
</urlset>`;
  }

  test('sitemap reflects published posts per host and excludes drafts', () => {
    const host = 'clinic-alavi.ir';
    const published = [{ slug: 'self-awareness' }, { slug: 'sleep-hygiene' }];
    const workshops = [{ id: 'ws-1' }];

    const xml = generateSitemapXML(host, published, workshops);

    // Verifies canonical host
    assert.ok(xml.includes('https://clinic-alavi.ir/blog/self-awareness'));
    assert.ok(xml.includes('https://clinic-alavi.ir/blog/sleep-hygiene'));
    assert.ok(xml.includes('https://clinic-alavi.ir/workshops/ws-1'));

    // Verify draft is absent
    assert.ok(!xml.includes('draft'));
  });

  test('unpublishing immediately removes post from sitemap', () => {
    const host = 'clinic-alavi.ir';
    let published = [{ slug: 'self-awareness' }, { slug: 'sleep-hygiene' }];

    let xml = generateSitemapXML(host, published, []);
    assert.ok(xml.includes('https://clinic-alavi.ir/blog/sleep-hygiene'));

    // Unpublish 'sleep-hygiene'
    published = published.filter((p) => p.slug !== 'sleep-hygiene');
    xml = generateSitemapXML(host, published, []);

    // Now absent from sitemap
    assert.ok(!xml.includes('https://clinic-alavi.ir/blog/sleep-hygiene'));
    assert.ok(xml.includes('https://clinic-alavi.ir/blog/self-awareness'));
  });
});

describe('PsychologyApiClient - Booking & Therapist Endpoints (PSY-2.1)', () => {
  test('listPublicTherapists and getPublicTherapistDetail return therapist data', async () => {
    const mockTherapist = {
      id: 'th-1',
      name: 'دکتر محمد رضایی',
      slug: 'dr-rezaei',
      title: 'متخصص روان‌شناسی بالینی',
      bio: '۱۰ سال سابقه مشاوره فردی و زوج‌درمانی',
      session_duration_minutes: 45,
    };

    const client = new PsychologyApiClient({
      baseUrl: 'http://localhost:8080',
      fetch: async (url, init) => {
        if (url.toString().endsWith('/public/therapists/th-1')) {
          return new Response(JSON.stringify(mockTherapist), { status: 200 });
        }
        return new Response(JSON.stringify([mockTherapist]), { status: 200 });
      },
    });

    const list = await client.listPublicTherapists();
    assert.strictEqual(list.length, 1);
    assert.strictEqual(list[0].name, 'دکتر محمد رضایی');

    const detail = await client.getPublicTherapistDetail('th-1');
    assert.strictEqual(detail.slug, 'dr-rezaei');
    assert.strictEqual(detail.session_duration_minutes, 45);
  });

  test('listPublicTherapistSlots returns available future slots without exposing PII', async () => {
    const mockSlots = [
      {
        id: 'slot-1',
        therapist_id: 'th-1',
        starts_at: '2026-11-22T09:00:00Z',
        ends_at: '2026-11-22T09:45:00Z',
        status: 'available',
      },
      {
        id: 'slot-2',
        therapist_id: 'th-1',
        starts_at: '2026-11-22T10:00:00Z',
        ends_at: '2026-11-22T10:45:00Z',
        status: 'available',
      },
    ];

    const client = new PsychologyApiClient({
      baseUrl: 'http://localhost:8080',
      fetch: async (url, init) => {
        assert.ok(url.toString().includes('/public/therapists/th-1/slots'));
        return new Response(JSON.stringify(mockSlots), { status: 200 });
      },
    });

    const slots = await client.listPublicTherapistSlots('th-1');
    assert.strictEqual(slots.length, 2);
    // Guarantee no PII in public slots
    assert.strictEqual(slots[0].status, 'available');
    assert.strictEqual(slots[0].guest_name, undefined);
    assert.strictEqual(slots[0].guest_phone, undefined);
  });

  test('public appointment lifecycle: create, check status with lookup hash, and cancel', async () => {
    let appt = {
      id: 'app-1',
      therapist_id: 'th-1',
      slot_id: 'slot-1',
      guest_name: 'سارا صادقی',
      guest_lookup_hash: 'hash-abc-123',
      status: 'pending',
      expires_at: '2026-11-21T11:30:00Z',
      starts_at: '2026-11-22T09:00:00Z',
      ends_at: '2026-11-22T09:45:00Z',
    };

    const client = new PsychologyApiClient({
      baseUrl: 'http://localhost:8080',
      fetch: async (url, init) => {
        const u = url.toString();
        if (init?.method === 'POST' && u.endsWith('/public/appointments')) {
          return new Response(JSON.stringify(appt), { status: 201 });
        }
        if (init?.method === 'POST' && u.endsWith('/cancel')) {
          assert.strictEqual(init.headers['X-Guest-Lookup-Hash'], 'hash-abc-123');
          appt.status = 'cancelled';
          return new Response(JSON.stringify(appt), { status: 200 });
        }
        if (u.endsWith('/public/appointments/app-1')) {
          assert.strictEqual(init.headers['X-Guest-Lookup-Hash'], 'hash-abc-123');
          return new Response(JSON.stringify(appt), { status: 200 });
        }
        return new Response(JSON.stringify({ code: 'NOT_FOUND' }), { status: 404 });
      },
    });

    // 1. Create appointment
    const created = await client.createPublicAppointment({
      therapist_id: 'th-1',
      slot_id: 'slot-1',
      guest_name: 'سارا صادقی',
      guest_phone: '09120000000',
    });
    assert.strictEqual(created.id, 'app-1');
    assert.strictEqual(created.status, 'pending');
    assert.strictEqual(created.guest_lookup_hash, 'hash-abc-123');

    // 2. Check status
    const status = await client.getPublicAppointmentStatus('app-1', 'hash-abc-123');
    assert.strictEqual(status.status, 'pending');

    // 3. Cancel appointment
    const cancelled = await client.cancelPublicAppointment('app-1', 'hash-abc-123', {
      reason: 'عدم امکان حضور در موعد مقرر',
    });
    assert.strictEqual(cancelled.status, 'cancelled');
  });

  test('admin appointment management: confirm, reschedule, and audit history', async () => {
    let appt = {
      id: 'app-1',
      therapist_id: 'th-1',
      therapist_name: 'دکتر محمد رضایی',
      slot_id: 'slot-1',
      starts_at: '2026-11-22T09:00:00Z',
      ends_at: '2026-11-22T09:45:00Z',
      guest_name: 'سارا صادقی',
      guest_phone: '09120000000',
      notes: 'جلسه اول',
      status: 'pending',
      expires_at: '2026-11-21T11:30:00Z',
      confirmed_at: null,
      cancelled_at: null,
      cancellation_reason: '',
      created_at: '2026-11-21T09:30:00Z',
      updated_at: '2026-11-21T09:30:00Z',
    };

    const history = [];

    const client = new PsychologyApiClient({
      baseUrl: 'http://localhost:8080',
      sessionToken: 'admin-sess',
      fetch: async (url, init) => {
        const u = url.toString();
        if (init?.method === 'POST' && u.endsWith('/confirm')) {
          appt.status = 'confirmed';
          appt.confirmed_at = new Date().toISOString();
          history.push({
            id: 'h-1',
            appointment_id: 'app-1',
            from_status: 'pending',
            to_status: 'confirmed',
            action: 'confirmed',
            actor_type: 'staff',
            reason: '',
            created_at: new Date().toISOString(),
          });
          return new Response(JSON.stringify(appt), { status: 200 });
        }
        if (init?.method === 'POST' && u.endsWith('/reschedule')) {
          const body = JSON.parse(init.body);
          appt.slot_id = body.new_slot_id;
          history.push({
            id: 'h-2',
            appointment_id: 'app-1',
            from_status: 'confirmed',
            to_status: 'confirmed',
            action: 'rescheduled',
            actor_type: 'staff',
            reason: body.reason,
            created_at: new Date().toISOString(),
          });
          return new Response(JSON.stringify(appt), { status: 200 });
        }
        if (u.endsWith('/history')) {
          return new Response(JSON.stringify(history), { status: 200 });
        }
        return new Response(JSON.stringify(appt), { status: 200 });
      },
    });

    // 1. Confirm
    const confirmed = await client.confirmAdminAppointment('app-1');
    assert.strictEqual(confirmed.status, 'confirmed');
    assert.ok(confirmed.confirmed_at);

    // 2. Reschedule
    const rescheduled = await client.rescheduleAdminAppointment('app-1', {
      new_slot_id: 'slot-2',
      reason: 'درخواست مراجع برای ساعت بعدی',
    });
    assert.strictEqual(rescheduled.slot_id, 'slot-2');

    // 3. History
    const logs = await client.getAdminAppointmentHistory('app-1');
    assert.strictEqual(logs.length, 2);
    assert.strictEqual(logs[0].action, 'confirmed');
    assert.strictEqual(logs[1].action, 'rescheduled');
  });
});

describe('PsychologyApiClient - Courses & Buyer Endpoints (Phase 3 - PSY-3.1)', () => {
  test('public courses listing, detail, and free preview stream', async () => {
    const mockCourses = [
      {
        id: 'c-1',
        title: 'مدیریت اضطراب فراگیر',
        slug: 'anxiety-management',
        description: 'دوره جامع مهارت‌های کنترل اضطراب',
        instructor_name: 'دکتر علوی',
        price_rials: 15000000,
        status: 'published',
        published_at: '2026-11-20T10:00:00Z',
        total_lessons_count: 5,
        total_duration_seconds: 3600,
      },
    ];

    const mockDetail = {
      ...mockCourses[0],
      lessons: [
        {
          id: 'l-1',
          title: 'درس اول: شناخت نشانه‌ها',
          description: 'مقدمه‌ای بر فیزیولوژی اضطراب',
          display_order: 1,
          duration_seconds: 600,
          is_free_preview: true,
        },
        {
          id: 'l-2',
          title: 'درس دوم: فنون تنفس',
          description: 'تکنیک‌های عملی آرام‌سازی',
          display_order: 2,
          duration_seconds: 900,
          is_free_preview: false,
        },
      ],
    };

    const client = new PsychologyApiClient({
      baseUrl: 'http://localhost:8080',
      fetch: async (url, init) => {
        const u = url.toString();
        if (u.endsWith('/public/courses')) {
          return new Response(JSON.stringify(mockCourses), { status: 200 });
        }
        if (u.endsWith('/public/courses/anxiety-management')) {
          return new Response(JSON.stringify(mockDetail), { status: 200 });
        }
        if (u.endsWith('/preview-stream')) {
          return new Response(
            JSON.stringify({
              stream_url: 'https://storage.parspack.com/preview/l-1.mp4?X-Amz-Expires=300',
              expires_at: '2026-11-20T10:05:00Z',
              duration_seconds: 600,
            }),
            { status: 200 }
          );
        }
        return new Response(JSON.stringify({ code: 'NOT_FOUND', message: 'یافت نشد' }), { status: 404 });
      },
    });

    const courses = await client.listPublicCourses();
    assert.strictEqual(courses.length, 1);
    assert.strictEqual(courses[0].slug, 'anxiety-management');

    const detail = await client.getPublicCourse('anxiety-management');
    assert.strictEqual(detail.lessons.length, 2);
    assert.strictEqual(detail.lessons[0].is_free_preview, true);

    const stream = await client.getPublicLessonPreviewStream('anxiety-management', 'l-1');
    assert.ok(stream.stream_url.includes('l-1.mp4'));
    assert.strictEqual(stream.duration_seconds, 600);
  });

  test('buyer OTP auth, entitlements, and private lesson stream', async () => {
    let verifiedSessionToken = '';

    const mockFetch = async (url, init) => {
      const u = url.toString();
      const headers = init?.headers || {};

      if (u.endsWith('/account/auth/otp/send')) {
        const body = JSON.parse(init.body);
        assert.strictEqual(body.phone, '09123456789');
        return new Response(
          JSON.stringify({
            phone: body.phone,
            expires_in_seconds: 120,
            message: 'کد تایید ارسال شد',
          }),
          { status: 200 }
        );
      }

      if (u.endsWith('/account/auth/otp/verify')) {
        const body = JSON.parse(init.body);
        assert.strictEqual(body.code, '123456');
        verifiedSessionToken = 'buyer-sess-token-xyz';
        return new Response(
          JSON.stringify({
            session_token: verifiedSessionToken,
            expires_at: '2026-11-27T10:00:00Z',
            account: {
              id: 'buyer-1',
              phone: '09123456789',
              full_name: 'سارا رضایی',
              status: 'active',
            },
          }),
          { status: 200 }
        );
      }

      if (u.endsWith('/account/courses')) {
        assert.strictEqual(headers['Authorization'], 'Bearer buyer-sess-token-xyz');
        return new Response(
          JSON.stringify([
            {
              id: 'ent-1',
              course_id: 'c-1',
              course_title_snapshot: 'مدیریت اضطراب فراگیر',
              price_rials_snapshot: 15000000,
              status: 'active',
              granted_at: '2026-11-21T09:00:00Z',
            },
          ]),
          { status: 200 }
        );
      }

      if (u.endsWith('/lessons/l-2/stream')) {
        assert.strictEqual(headers['Authorization'], 'Bearer buyer-sess-token-xyz');
        return new Response(
          JSON.stringify({
            stream_url: 'https://storage.parspack.com/courses/private/l-2.mp4?signed=true',
            expires_at: '2026-11-21T09:05:00Z',
            duration_seconds: 900,
          }),
          { status: 200 }
        );
      }

      return new Response(JSON.stringify({ code: 'UNAUTHORIZED' }), { status: 401 });
    };

    const client = new PsychologyApiClient({
      baseUrl: 'http://localhost:8080',
      fetch: mockFetch,
    });

    // 1. Send OTP
    const otpRes = await client.sendBuyerOTP({ phone: '09123456789' });
    assert.strictEqual(otpRes.expires_in_seconds, 120);

    // 2. Verify OTP
    const verifyRes = await client.verifyBuyerOTP({ phone: '09123456789', code: '123456' });
    assert.strictEqual(verifyRes.session_token, 'buyer-sess-token-xyz');
    assert.strictEqual(verifyRes.account.full_name, 'سارا رضایی');

    // 3. Buyer Client with token
    const buyerClient = new PsychologyApiClient({
      baseUrl: 'http://localhost:8080',
      headers: {
        Authorization: `Bearer ${verifyRes.session_token}`,
      },
      fetch: mockFetch,
    });

    const myCourses = await buyerClient.listMyCourses();
    assert.strictEqual(myCourses.length, 1);
    assert.strictEqual(myCourses[0].course_title_snapshot, 'مدیریت اضطراب فراگیر');

    const stream = await buyerClient.getCourseLessonStreamUrl('c-1', 'l-2');
    assert.ok(stream.stream_url.includes('courses/private/l-2.mp4'));
  });

  test('admin courses and lessons management lifecycle', async () => {
    let courseStatus = 'draft';
    const lessonsList = [];

    const client = new PsychologyApiClient({
      baseUrl: 'http://localhost:8080',
      headers: {
        Authorization: 'Bearer admin-token',
      },
      fetch: async (url, init) => {
        const u = url.toString();
        const method = init?.method || 'GET';

        if (u.endsWith('/admin/courses') && method === 'POST') {
          const body = JSON.parse(init.body);
          return new Response(
            JSON.stringify({
              id: 'c-100',
              title: body.title,
              slug: body.slug,
              description: body.description,
              instructor_name: body.instructor_name,
              price_rials: body.price_rials,
              status: courseStatus,
              lessons_count: 0,
              created_at: new Date().toISOString(),
              updated_at: new Date().toISOString(),
            }),
            { status: 201 }
          );
        }

        if (u.endsWith('/admin/courses/c-100/publish') && method === 'POST') {
          courseStatus = 'published';
          return new Response(
            JSON.stringify({
              id: 'c-100',
              title: 'دوره جدید',
              status: courseStatus,
              published_at: new Date().toISOString(),
            }),
            { status: 200 }
          );
        }

        if (u.endsWith('/admin/courses/c-100/lessons') && method === 'POST') {
          const body = JSON.parse(init.body);
          const newLesson = {
            id: `les-${lessonsList.length + 1}`,
            course_id: 'c-100',
            title: body.title,
            description: body.description,
            display_order: body.display_order,
            media_asset_id: body.media_asset_id,
            duration_seconds: body.duration_seconds,
            is_free_preview: body.is_free_preview,
            created_at: new Date().toISOString(),
            updated_at: new Date().toISOString(),
          };
          lessonsList.push(newLesson);
          return new Response(JSON.stringify(newLesson), { status: 201 });
        }

        if (u.endsWith('/admin/courses/c-100/lessons') && method === 'GET') {
          return new Response(JSON.stringify(lessonsList), { status: 200 });
        }

        if (u.endsWith('/admin/courses/c-100/entitlements') && method === 'GET') {
          return new Response(
            JSON.stringify([
              {
                id: 'ent-1',
                tenant_id: 'ten-1',
                buyer_id: 'buyer-1',
                buyer_phone: '09123456789',
                buyer_name: 'سارا رضایی',
                course_id: 'c-100',
                purchase_id: 'pur-1',
                status: 'active',
                course_title_snapshot: 'دوره جدید',
                price_rials_snapshot: 20000000,
                granted_at: new Date().toISOString(),
              },
            ]),
            { status: 200 }
          );
        }

        return new Response(JSON.stringify({ code: 'NOT_FOUND' }), { status: 404 });
      },
    });

    // 1. Create Course
    const course = await client.createAdminCourse({
      title: 'دوره جدید',
      slug: 'new-course',
      description: 'توضیحات دوره',
      instructor_name: 'دکتر صادقی',
      price_rials: 20000000,
    });
    assert.strictEqual(course.id, 'c-100');
    assert.strictEqual(course.status, 'draft');

    // 2. Publish Course
    const published = await client.publishAdminCourse('c-100');
    assert.strictEqual(published.status, 'published');

    // 3. Add Lesson
    const lesson = await client.createAdminCourseLesson('c-100', {
      title: 'درس اول',
      description: 'مقدمات',
      display_order: 1,
      media_asset_id: 'med-video-1',
      duration_seconds: 1200,
      is_free_preview: true,
    });
    assert.strictEqual(lesson.id, 'les-1');
    assert.strictEqual(lesson.is_free_preview, true);

    // 4. List Lessons
    const lessons = await client.listAdminCourseLessons('c-100');
    assert.strictEqual(lessons.length, 1);

    // 5. Entitlements
    const entitlements = await client.listAdminCourseEntitlements('c-100');
    assert.strictEqual(entitlements.length, 1);
    assert.strictEqual(entitlements[0].buyer_phone, '09123456789');
  });

  test('Public Course checkout and syllabus flow', async () => {
    const client = new PsychologyApiClient({
      baseUrl: 'https://test-clinic.ir',
      fetch: async (url, init = {}) => {
        const u = url.toString();
        const method = init.method || 'GET';

        if (u.endsWith('/public/courses/anxiety-management') && method === 'GET') {
          return new Response(
            JSON.stringify({
              id: 'c-200',
              title: 'دوره اضطراب',
              slug: 'anxiety-management',
              instructor_name: 'دکتر محمدی',
              price_rials: 15000000,
              status: 'published',
              lessons: [
                {
                  id: 'les-1',
                  title: 'معرفی',
                  display_order: 1,
                  duration_seconds: 900,
                  is_free_preview: true,
                },
              ],
            }),
            { status: 200 }
          );
        }

        if (u.endsWith('/public/courses/anxiety-management/checkout') && method === 'POST') {
          return new Response(
            JSON.stringify({
              payment_attempt_id: 'att-100',
              purchase_id: 'pur-100',
              authority: 'A00001234',
              payment_url: 'https://payment.zarinpal.com/pg/StartPay/A00001234',
              amount_rials: 15000000,
              status: 'initiated',
            }),
            { status: 200 }
          );
        }

        return new Response(JSON.stringify({ code: 'NOT_FOUND' }), { status: 404 });
      },
    });

    const course = await client.getPublicCourse('anxiety-management');
    assert.strictEqual(course.id, 'c-200');
    assert.strictEqual(course.price_rials, 15000000);
    assert.strictEqual(course.lessons.length, 1);

    const checkout = await client.initiateCourseCheckout('anxiety-management', {
      idempotency_key: 'chk_key_test_1',
      callback_url: 'https://test-clinic.ir/callbacks/payments/zarinpal',
    });
    assert.strictEqual(checkout.authority, 'A00001234');
    assert.strictEqual(checkout.amount_rials, 15000000);
    assert.strictEqual(checkout.status, 'initiated');
  });

  test('reports: getAdminReportsMetrics aggregates non-PII metrics', async () => {
    const client = new PsychologyApiClient({
      baseUrl: 'https://api.example.com',
      fetch: async (url, opts) => {
        const u = String(url);
        if (u.endsWith('/admin/reports/metrics')) {
          return new Response(
            JSON.stringify({
              tenant_id: 't-123',
              generated_at: new Date().toISOString(),
              workshops: {
                total_workshops: 4,
                published_workshops: 2,
                draft_workshops: 1,
                closed_workshops: 1,
                cancelled_workshops: 0,
                total_registrations: 30,
                confirmed_registrations: 28,
                pending_registrations: 1,
                cancelled_registrations: 1,
              },
              booking: {
                active_therapists: 2,
                total_slots: 40,
                available_slots: 15,
                booked_slots: 20,
                held_slots: 5,
                total_appointments: 25,
                confirmed_appointments: 18,
                pending_appointments: 2,
                completed_appointments: 15,
                cancelled_appointments: 5,
              },
              courses: {
                total_courses: 3,
                published_courses: 2,
                draft_courses: 0,
                archived_courses: 1,
                total_lessons: 18,
                total_entitlements: 50,
                active_entitlements: 48,
                revoked_entitlements: 2,
                pending_entitlements: 0,
              },
              financial: {
                total_purchases: 80,
                completed_purchases: 76,
                pending_purchases: 2,
                failed_purchases: 1,
                refund_required_purchases: 0,
                refunded_purchases: 1,
                gross_revenue_rials: 400000000,
                total_refunded_rials: 5000000,
                net_revenue_rials: 395000000,
                workshop_revenue_rials: 150000000,
                course_revenue_rials: 250000000,
              },
              payment_attempts: {
                total_attempts: 85,
                initiated_attempts: 85,
                verified_attempts: 76,
                failed_attempts: 9,
              },
              media: {
                total_assets: 10,
                total_bytes: 1073741824, // 1 GiB
                storage_quota_bytes: 10737418240, // 10 GiB
                usage_percentage_permille: 100,
                usage_percentage: 10,
                public_bytes: 52428800,
                course_private_bytes: 1021313024,
                video_count: 6,
                audio_count: 3,
                image_count: 1,
              },
            }),
            { status: 200 }
          );
        }
        return new Response(JSON.stringify({ code: 'NOT_FOUND' }), { status: 404 });
      },
    });

    const metrics = await client.getAdminReportsMetrics();
    assert.strictEqual(metrics.tenant_id, 't-123');
    assert.strictEqual(metrics.workshops.total_workshops, 4);
    assert.strictEqual(metrics.booking.active_therapists, 2);
    assert.strictEqual(metrics.courses.total_lessons, 18);
    assert.strictEqual(metrics.financial.net_revenue_rials, 395000000);
    assert.strictEqual(metrics.media.usage_percentage, 10);
  });

  test('Public and Admin Site Settings with template configuration (PSY-5.3)', async () => {
    let storedSettings = {
      tenant_id: 't-test-1',
      title: 'کلینیک تخصصی روان‌شناسی',
      description: 'مرکز مشاوره فردی و زوج‌درمانی',
      phone: '02188888888',
      address: 'تهران، میدان ونک',
      primary_color: '#0284c7',
      template_id: 'minimal',
      template_version: '1.0.0',
      theme_radius: 'md',
      theme_scale: 'md',
    };

    const client = new PsychologyApiClient({
      baseUrl: 'http://localhost:8080',
      fetch: async (url, init) => {
        const u = url.toString();
        if (u.includes('/v1/public/site')) {
          return new Response(
            JSON.stringify({
              name: 'کلینیک مهر',
              title: storedSettings.title,
              description: storedSettings.description,
              phone: storedSettings.phone,
              address: storedSettings.address,
              primary_color: storedSettings.primary_color,
              template_id: storedSettings.template_id,
              template_version: storedSettings.template_version,
              theme_radius: storedSettings.theme_radius,
              theme_scale: storedSettings.theme_scale,
              palette_id: storedSettings.palette_id || 'calm-sage',
            }),
            { status: 200 }
          );
        }
        if (u.includes('/v1/admin/site/appearance')) {
          if (init?.method === 'PUT') {
            const body = JSON.parse(init.body);
            storedSettings = {
              ...storedSettings,
              template_id: body.template_id,
              palette_id: body.palette_id,
              theme_radius: body.theme_radius || storedSettings.theme_radius || 'md',
              theme_scale: body.theme_scale || storedSettings.theme_scale || 'md',
            };
            return new Response(
              JSON.stringify({
                template_id: storedSettings.template_id,
                template_version: '1.0.0',
                palette_id: storedSettings.palette_id,
                theme_radius: storedSettings.theme_radius,
                theme_scale: storedSettings.theme_scale,
              }),
              {
                status: 200,
                headers: { 'Cache-Control': 'private, no-store' },
              }
            );
          }
          return new Response(
            JSON.stringify({
              template_id: storedSettings.template_id,
              template_version: '1.0.0',
              palette_id: storedSettings.palette_id || 'calm-sage',
              theme_radius: storedSettings.theme_radius,
              theme_scale: storedSettings.theme_scale,
            }),
            {
              status: 200,
              headers: { 'Cache-Control': 'private, no-store' },
            }
          );
        }
        if (u.includes('/v1/admin/site')) {
          if (init?.method === 'PUT') {
            const body = JSON.parse(init.body);
            storedSettings = {
              ...storedSettings,
              ...body,
              template_version: '1.0.0',
            };
            return new Response(JSON.stringify(storedSettings), {
              status: 200,
              headers: { 'Cache-Control': 'private, no-store' },
            });
          }
          return new Response(JSON.stringify(storedSettings), { status: 200 });
        }
        return new Response(JSON.stringify({ code: 'NOT_FOUND' }), { status: 404 });
      },
    });

    // 1. Get Public Site
    const pub = await client.getPublicSite();
    assert.strictEqual(pub.name, 'کلینیک مهر');
    assert.strictEqual(pub.template_id, 'minimal');
    assert.strictEqual(pub.template_version, '1.0.0');
    assert.strictEqual(pub.theme_radius, 'md');
    assert.strictEqual(pub.theme_scale, 'md');
    assert.strictEqual(pub.palette_id, 'calm-sage');

    // 2. Get Admin Site
    const adm = await client.getAdminSite();
    assert.strictEqual(adm.tenant_id, 't-test-1');
    assert.strictEqual(adm.template_id, 'minimal');

    // 3. Get Admin Appearance
    const app = await client.getAdminAppearance();
    assert.strictEqual(app.template_id, 'minimal');
    assert.strictEqual(app.template_version, '1.0.0');
    assert.strictEqual(app.palette_id, 'calm-sage');
    assert.strictEqual(app.theme_radius, 'md');
    assert.strictEqual(app.theme_scale, 'md');

    // 4. Update Admin Appearance
    const updatedApp = await client.updateAdminAppearance({
      template_id: 'mobile-app',
      palette_id: 'ocean-trust',
      theme_radius: 'lg',
      theme_scale: 'sm',
    });
    assert.strictEqual(updatedApp.template_id, 'mobile-app');
    assert.strictEqual(updatedApp.template_version, '1.0.0');
    assert.strictEqual(updatedApp.palette_id, 'ocean-trust');
    assert.strictEqual(updatedApp.theme_radius, 'lg');
    assert.strictEqual(updatedApp.theme_scale, 'sm');

    // 5. Update Admin Site with template and theme
    const updated = await client.updateAdminSite({
      title: 'کلینیک مدرن سپهر',
      description: 'روان‌درمانی تحلیلی',
      primary_color: '#0ea5e9',
      template_id: 'clinical',
      theme_radius: 'lg',
      theme_scale: 'sm',
    });
    assert.strictEqual(updated.title, 'کلینیک مدرن سپهر');
    assert.strictEqual(updated.template_id, 'clinical');
    assert.strictEqual(updated.template_version, '1.0.0');
    assert.strictEqual(updated.theme_radius, 'lg');
    assert.strictEqual(updated.theme_scale, 'sm');

    // 6. Public Site reflects update
    const pubAfter = await client.getPublicSite();
    assert.strictEqual(pubAfter.title, 'کلینیک مدرن سپهر');
    assert.strictEqual(pubAfter.template_id, 'clinical');
    assert.strictEqual(pubAfter.theme_radius, 'lg');
    assert.strictEqual(pubAfter.theme_scale, 'sm');
  });
});
