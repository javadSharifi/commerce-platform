import test from 'node:test';
import assert from 'node:assert';
import React from 'react';
import { renderToString } from 'react-dom/server';
import {
  Modal,
  Select,
  Tabs,
  TabList,
  Tab,
  TabPanel,
  PersianDatePicker,
  toISODateString,
  parseISODate,
  formatPersianDate,
  toPersianCalendarDate,
  getTodayPersian,
  Button,
  Alert,
} from '../dist/index.js';

test('Date Utils - ISO and Persian conversions', () => {
  // Parsing ISO date
  const parsed = parseISODate('2026-11-21');
  assert.ok(parsed);
  assert.strictEqual(parsed.year, 2026);
  assert.strictEqual(parsed.month, 11);
  assert.strictEqual(parsed.day, 21);

  // Conversion to ISO string
  const iso = toISODateString(parsed);
  assert.strictEqual(iso, '2026-11-21');

  // Conversion to Persian Calendar Date
  const persian = toPersianCalendarDate('2026-11-21');
  assert.ok(persian);
  assert.strictEqual(persian.calendar.identifier, 'persian');
  assert.strictEqual(persian.year, 1405);
  assert.strictEqual(persian.month, 8);
  assert.strictEqual(persian.day, 30);

  // Conversion from Persian Calendar Date back to ISO
  const backToIso = toISODateString(persian);
  assert.strictEqual(backToIso, '2026-11-21');

  // Formatting to Persian text
  const formatted = formatPersianDate('2026-11-21', { style: 'long' });
  assert.ok(formatted.includes('۱۴۰۵') || formatted.includes('1405'));
  assert.ok(formatted.includes('آبان'));
  assert.ok(formatted.includes('۳۰') || formatted.includes('30'));

  // Today Persian returns valid Persian date
  const todayP = getTodayPersian();
  assert.ok(todayP);
  assert.strictEqual(todayP.calendar.identifier, 'persian');
  assert.ok(todayP.year >= 1400);
});

test('Enhanced Primitives - Button loading state and Alert with title', () => {
  // Button with isLoading
  const buttonHtml = renderToString(
    React.createElement(Button, { isLoading: true, variant: 'primary' }, 'در حال پردازش')
  );
  assert.match(buttonHtml, /loading-spinner/);
  assert.match(buttonHtml, /disabled/);
  assert.match(buttonHtml, /در حال پردازش/);

  // Alert with title
  const alertHtml = renderToString(
    React.createElement(
      Alert,
      { variant: 'error', title: 'خطای رزرو' },
      'این زمان قبلاً توسط کاربر دیگری رزرو شده است.'
    )
  );
  assert.match(alertHtml, /خطای رزرو/);
  assert.match(alertHtml, /این زمان قبلاً توسط کاربر دیگری رزرو شده است\./);
  assert.match(alertHtml, /alert-error/);
});

test('Interactive SSR - Modal with trigger renders trigger button cleanly', () => {
  const triggerBtn = React.createElement(Button, { variant: 'primary' }, 'مشاهده پرونده');
  const modalElement = React.createElement(
    Modal,
    {
      trigger: triggerBtn,
      title: 'پرونده مراجع',
      description: 'اطلاعات محرمانه پرونده بالینی',
      actions: React.createElement(Button, { variant: 'ghost' }, 'بستن'),
    },
    React.createElement('p', null, 'محتوای جلسات مشاوره')
  );

  const html = renderToString(modalElement);
  assert.match(html, /مشاهده پرونده/);
  assert.match(html, /btn-primary/);
});

test('Interactive SSR - Select renders accessible button and native form select fallback', () => {
  const selectElement = React.createElement(Select, {
    name: 'consultation_type',
    label: 'نوع مشاوره',
    defaultValue: 'online',
    options: [
      { id: 'online', label: 'مشاوره آنلاین' },
      { id: 'in_person', label: 'مشاوره حضوری' },
      { id: 'phone', label: 'مشاوره تلفنی' },
    ],
  });

  const html = renderToString(selectElement);
  // Label exists
  assert.match(html, /نوع مشاوره/);
  // RAC Button exists with aria-haspopup="listbox"
  assert.match(html, /aria-haspopup="listbox"/);
  // Default value text is rendered
  assert.match(html, /مشاوره آنلاین/);
  // Native hidden select for SSR and standard form submissions
  assert.match(html, /<select[^>]*name="consultation_type"/);
  assert.match(html, /<option[^>]*value="online"[^>]*selected=""/);
  assert.match(html, /<option[^>]*value="in_person"/);
  assert.match(html, /<option[^>]*value="phone"/);
});

test('Interactive SSR - Tabs renders DaisyUI tabs and active panel content', () => {
  const tabsElement = React.createElement(Tabs, {
    variant: 'boxed',
    defaultSelectedKey: 'general',
    items: [
      {
        id: 'general',
        label: 'اطلاعات فردی',
        content: React.createElement('div', { id: 'general-content' }, 'فرم اطلاعات روان‌شناس'),
      },
      {
        id: 'schedule',
        label: 'برنامه هفتگی',
        content: React.createElement('div', { id: 'schedule-content' }, 'جدول ساعات کاری'),
      },
    ],
  });

  const html = renderToString(tabsElement);
  assert.match(html, /tabs-boxed/);
  assert.match(html, /role="tablist"/);
  assert.match(html, /اطلاعات فردی/);
  assert.match(html, /برنامه هفتگی/);
  assert.match(html, /tab-active/);
  assert.match(html, /فرم اطلاعات روان‌شناس/);
});

test('Interactive SSR - PersianDatePicker renders Jalali calendar and standard ISO hidden input', () => {
  const pickerElement = React.createElement(PersianDatePicker, {
    name: 'appointment_date',
    label: 'تاریخ نوبت',
    defaultValue: '2026-11-21',
  });

  const html = renderToString(pickerElement);
  // Label
  assert.match(html, /تاریخ نوبت/);
  // Persian numbers rendered in segments (1405 / 8 / 30)
  assert.ok(html.includes('۱۴۰۵') || html.includes('1405'));
  assert.ok(html.includes('۸') || html.includes('8'));
  assert.ok(html.includes('۳۰') || html.includes('30'));
  // Input group styling
  assert.match(html, /input-bordered/);
  // Hidden standard ISO date input for form submission & API compatibility
  assert.match(html, /type="date"/);
  assert.match(html, /value="2026-11-21"/);
});
