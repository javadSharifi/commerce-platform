import test from 'node:test';
import assert from 'node:assert';
import React from 'react';
import { renderToString } from 'react-dom/server';
import {
  Button,
  Input,
  Textarea,
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
  CardFooter,
  Badge,
  Alert,
  EmptyState,
  isValidHexColor,
  isValidRadius,
  isValidScale,
  DEFAULT_PRIMARY_COLOR,
  DEFAULT_RADIUS,
  DEFAULT_SCALE,
  PRESET_PRIMARY_COLORS,
  RADIUS_OPTIONS,
  SCALE_OPTIONS,
  TEMPLATE_CATALOG,
  DEFAULT_TEMPLATE_ID,
  isValidTemplateId,
  getTemplateMeta,
  resolveTemplateMeta,
  PALETTE_CATALOG,
  DEFAULT_PALETTE_ID,
  isValidPaletteId,
  getPaletteMeta,
  resolvePaletteMeta,
} from '../dist/index.js';

test('Template Catalog - Metadata & Safe Fallback', () => {
  assert.strictEqual(DEFAULT_TEMPLATE_ID, 'minimal');
  assert.strictEqual(TEMPLATE_CATALOG.length >= 2, true);

  const minimal = TEMPLATE_CATALOG.find((t) => t.id === 'minimal');
  assert.ok(minimal);
  assert.strictEqual(minimal.version, '1.0.0');
  assert.strictEqual(minimal.labelFa, 'مینیمال');
  assert.strictEqual(minimal.daisyTheme, 'minimal');
  assert.ok(minimal.descriptionFa.length > 0);

  const clinical = TEMPLATE_CATALOG.find((t) => t.id === 'clinical');
  assert.ok(clinical);
  assert.strictEqual(clinical.version, '1.0.0');
  assert.strictEqual(clinical.labelFa, 'کلینیکال');
  assert.strictEqual(clinical.daisyTheme, 'clinical');
  assert.ok(clinical.descriptionFa.length > 0);

  const mobileApp = TEMPLATE_CATALOG.find((t) => t.id === 'mobile-app');
  assert.ok(mobileApp);
  assert.strictEqual(mobileApp.version, '1.0.0');
  assert.strictEqual(mobileApp.labelFa, 'اپلیکیشن موبایل');
  assert.strictEqual(mobileApp.daisyTheme, 'mobile-app');
  assert.ok(mobileApp.descriptionFa.length > 0);

  const editorial = TEMPLATE_CATALOG.find((t) => t.id === 'editorial');
  assert.ok(editorial);
  assert.strictEqual(editorial.version, '1.0.0');
  assert.strictEqual(editorial.labelFa, 'ادیتوریال');
  assert.strictEqual(editorial.daisyTheme, 'editorial');
  assert.ok(editorial.descriptionFa.length > 0);

  // Validation
  assert.strictEqual(isValidTemplateId('minimal'), true);
  assert.strictEqual(isValidTemplateId('clinical'), true);
  assert.strictEqual(isValidTemplateId('mobile-app'), true);
  assert.strictEqual(isValidTemplateId('editorial'), true);
  assert.strictEqual(isValidTemplateId('non-existent'), false);
  assert.strictEqual(isValidTemplateId(''), false);

  // Getter
  assert.strictEqual(getTemplateMeta('minimal')?.id, 'minimal');
  assert.strictEqual(getTemplateMeta('clinical')?.id, 'clinical');
  assert.strictEqual(getTemplateMeta('mobile-app')?.id, 'mobile-app');
  assert.strictEqual(getTemplateMeta('editorial')?.id, 'editorial');
  assert.strictEqual(getTemplateMeta('unknown'), undefined);

  // Safe Fallback
  assert.strictEqual(resolveTemplateMeta('clinical').id, 'clinical');
  assert.strictEqual(resolveTemplateMeta('mobile-app').id, 'mobile-app');
  assert.strictEqual(resolveTemplateMeta('unknown-id').id, 'minimal');
  assert.strictEqual(resolveTemplateMeta(null).id, 'minimal');
  assert.strictEqual(resolveTemplateMeta(undefined).id, 'minimal');
});

test('PSY-6.1: Palette Catalog - Four Palettes, Semantic DaisyUI Tokens & Prototype-Safe Fallback', () => {
  assert.strictEqual(DEFAULT_PALETTE_ID, 'calm-sage');
  assert.strictEqual(PALETTE_CATALOG.length, 4);

  const expectedIds = ['calm-sage', 'ocean-trust', 'warm-sand', 'royal-plum'];
  assert.deepStrictEqual(
    PALETTE_CATALOG.map((p) => p.id),
    expectedIds
  );

  for (const p of PALETTE_CATALOG) {
    assert.ok(p.id.length > 0);
    assert.ok(p.labelFa.length > 0);
    assert.ok(p.descriptionFa.length > 0);
    assert.match(p.primaryColor, /^#[0-9a-fA-F]{6}$/);

    // Verify all semantic tokens are defined with valid hex codes
    const { tokens } = p;
    assert.match(tokens.colorBase100, /^#[0-9a-fA-F]{6}$/);
    assert.match(tokens.colorBase200, /^#[0-9a-fA-F]{6}$/);
    assert.match(tokens.colorBase300, /^#[0-9a-fA-F]{6}$/);
    assert.match(tokens.colorBaseContent, /^#[0-9a-fA-F]{6}$/);
    assert.match(tokens.colorPrimary, /^#[0-9a-fA-F]{6}$/);
    assert.match(tokens.colorPrimaryContent, /^#[0-9a-fA-F]{6}$/);
    assert.match(tokens.colorSecondary, /^#[0-9a-fA-F]{6}$/);
    assert.match(tokens.colorSecondaryContent, /^#[0-9a-fA-F]{6}$/);
    assert.match(tokens.colorAccent, /^#[0-9a-fA-F]{6}$/);
    assert.match(tokens.colorAccentContent, /^#[0-9a-fA-F]{6}$/);
    assert.match(tokens.colorNeutral, /^#[0-9a-fA-F]{6}$/);
    assert.match(tokens.colorNeutralContent, /^#[0-9a-fA-F]{6}$/);
  }

  // Validation
  for (const id of expectedIds) {
    assert.strictEqual(isValidPaletteId(id), true);
  }
  assert.strictEqual(isValidPaletteId('invalid_palette'), false);
  assert.strictEqual(isValidPaletteId(''), false);
  assert.strictEqual(isValidPaletteId(null), false);
  assert.strictEqual(isValidPaletteId(undefined), false);

  // Prototype attack resistance (PSY-5.17 / PSY-6.1)
  for (const protoKey of ['toString', 'valueOf', 'constructor', '__proto__', 'hasOwnProperty']) {
    assert.strictEqual(isValidPaletteId(protoKey), false);
    assert.strictEqual(resolvePaletteMeta(protoKey).id, 'calm-sage');
  }

  // Safe fallback
  assert.strictEqual(resolvePaletteMeta(undefined).id, 'calm-sage');
  assert.strictEqual(resolvePaletteMeta(null).id, 'calm-sage');
  assert.strictEqual(resolvePaletteMeta('').id, 'calm-sage');
  assert.strictEqual(resolvePaletteMeta('unknown').id, 'calm-sage');
  assert.strictEqual(resolvePaletteMeta('ocean-trust').id, 'ocean-trust');
  assert.strictEqual(resolvePaletteMeta('warm-sand').id, 'warm-sand');
  assert.strictEqual(resolvePaletteMeta('royal-plum').id, 'royal-plum');
});

test('Tokens & Presets validation', () => {
  assert.strictEqual(isValidHexColor('#0284c7'), true);
  assert.strictEqual(isValidHexColor('#ffffff'), true);
  assert.strictEqual(isValidHexColor('invalid'), false);
  assert.strictEqual(isValidHexColor('#123'), false);

  assert.strictEqual(isValidRadius('sm'), true);
  assert.strictEqual(isValidRadius('md'), true);
  assert.strictEqual(isValidRadius('lg'), true);
  assert.strictEqual(isValidRadius('xl'), false);

  assert.strictEqual(isValidScale('sm'), true);
  assert.strictEqual(isValidScale('md'), true);
  assert.strictEqual(isValidScale('lg'), true);
  assert.strictEqual(isValidScale('2xl'), false);

  assert.strictEqual(DEFAULT_PRIMARY_COLOR, '#0284c7');
  assert.strictEqual(DEFAULT_RADIUS, 'md');
  assert.strictEqual(DEFAULT_SCALE, 'md');

  assert.strictEqual(PRESET_PRIMARY_COLORS.length >= 6, true);
  for (const preset of PRESET_PRIMARY_COLORS) {
    assert.strictEqual(isValidHexColor(preset.value), true);
    assert.ok(preset.labelFa.length > 0);
  }

  assert.strictEqual(RADIUS_OPTIONS.length, 3);
  assert.strictEqual(SCALE_OPTIONS.length, 3);
});

test('Primitives SSR - Button renders properly', () => {
  const html = renderToString(React.createElement(Button, { variant: 'primary', size: 'md' }, 'کلیک کنید'));
  assert.match(html, /<button/);
  assert.match(html, /btn-primary/);
  assert.match(html, /btn-md/);
  assert.match(html, /کلیک کنید/);
});

test('Primitives SSR - Input renders with DaisyUI classes', () => {
  const html = renderToString(
    React.createElement(Input, {
      placeholder: 'شماره تلفن',
      size: 'md',
      isError: true,
      defaultValue: '۰۹۱۲۰۰۰۰۰۰۰',
    })
  );
  assert.match(html, /<input/);
  assert.match(html, /input-bordered/);
  assert.match(html, /input-md/);
  assert.match(html, /input-error/);
  assert.match(html, /۰۹۱۲۰۰۰۰۰۰۰/);
});

test('Primitives SSR - Textarea renders with DaisyUI classes', () => {
  const html = renderToString(
    React.createElement(Textarea, {
      placeholder: 'توضیحات کلینیک',
      size: 'lg',
      isError: false,
      defaultValue: 'مرکز تخصصی روان‌درمانی',
    })
  );
  assert.match(html, /<textarea/);
  assert.match(html, /textarea-bordered/);
  assert.match(html, /textarea-lg/);
  assert.match(html, /مرکز تخصصی روان‌درمانی/);
});

test('Primitives SSR - Card structure renders semantic markup', () => {
  const cardElement = React.createElement(
    Card,
    null,
    React.createElement(
      CardHeader,
      null,
      React.createElement(CardTitle, null, 'عنوان کارت'),
      React.createElement(CardDescription, null, 'توضیحات کوتاه کارت')
    ),
    React.createElement(CardContent, null, 'محتوای اصلی کارت'),
    React.createElement(
      CardFooter,
      null,
      React.createElement(Button, { variant: 'primary' }, 'اقدام')
    )
  );

  const html = renderToString(cardElement);
  assert.match(html, /card bg-base-100/);
  assert.match(html, /card-title/);
  assert.match(html, /card-body/);
  assert.match(html, /card-actions/);
  assert.match(html, /justify-end/);
  assert.match(html, /عنوان کارت/);
  assert.match(html, /محتوای اصلی کارت/);
  assert.match(html, /btn-primary/);
});

test('Primitives SSR - Badge renders with DaisyUI classes', () => {
  const html = renderToString(React.createElement(Badge, { variant: 'success', size: 'sm' }, 'فعال'));
  assert.match(html, /<span/);
  assert.match(html, /badge/);
  assert.match(html, /badge-success/);
  assert.match(html, /badge-sm/);
  assert.match(html, /فعال/);
});

test('Primitives SSR - Alert renders with role and DaisyUI classes', () => {
  const html = renderToString(
    React.createElement(Alert, { variant: 'warning' }, 'نوبت شما در انتظار تأیید است.')
  );
  assert.match(html, /role="alert"/);
  assert.match(html, /alert-warning/);
  assert.match(html, /نوبت شما در انتظار تأیید است\./);
});

test('Primitives SSR - EmptyState renders with title and action', () => {
  const html = renderToString(
    React.createElement(EmptyState, {
      title: 'هنوز نوبتی ثبت نشده است',
      description: 'برای دریافت وقت ویزیت روی دکمه زیر کلیک کنید.',
      action: React.createElement(Button, { variant: 'primary' }, 'رزرو نوبت جدید'),
    })
  );
  assert.match(html, /هنوز نوبتی ثبت نشده است/);
  assert.match(html, /برای دریافت وقت ویزیت روی دکمه زیر کلیک کنید\./);
  assert.match(html, /رزرو نوبت جدید/);
});
