export const VALID_RADII = ['sm', 'md', 'lg'] as const;
export type ThemeRadius = (typeof VALID_RADII)[number];

export const VALID_SCALES = ['sm', 'md', 'lg'] as const;
export type ThemeScale = (typeof VALID_SCALES)[number];

export const DEFAULT_RADIUS: ThemeRadius = 'md';
export const DEFAULT_SCALE: ThemeScale = 'md';
export const DEFAULT_PRIMARY_COLOR = '#0284c7';

export const HEX_COLOR_REGEX = /^#[0-9a-fA-F]{6}$/;

export interface ThemePresetColor {
  labelFa: string;
  value: string;
}

export const PRESET_PRIMARY_COLORS: readonly ThemePresetColor[] = [
  { labelFa: 'آبی اقیانوسی (پیش‌فرض)', value: '#0284c7' },
  { labelFa: 'سبز کله‌غازی (آرامش‌بخش)', value: '#0d9488' },
  { labelFa: 'نیلی کلاسیک (اعتماد)', value: '#4f46e5' },
  { labelFa: 'سبز زمردی (سلامت)', value: '#059669' },
  { labelFa: 'رز گرم (مشاوره)', value: '#e11d48' },
  { labelFa: 'کهربایی ملایم (امید)', value: '#d97706' },
] as const;

export const RADIUS_OPTIONS: readonly { value: ThemeRadius; labelFa: string; descriptionFa: string }[] = [
  { value: 'sm', labelFa: 'کم (ساده)', descriptionFa: 'گوشه‌های با انحنای اندک و رسمی' },
  { value: 'md', labelFa: 'متوسط (پیش‌فرض)', descriptionFa: 'تعادل استاندارد برای اکثر رابط‌ها' },
  { value: 'lg', labelFa: 'زیاد (نرم و گرد)', descriptionFa: 'گوشه‌های گرد، صمیمی و دوستانه' },
] as const;

export const SCALE_OPTIONS: readonly { value: ThemeScale; labelFa: string; descriptionFa: string }[] = [
  { value: 'sm', labelFa: 'فشرده', descriptionFa: 'فاصله‌ها و اندازه‌های کوچک‌تر برای نمایش اطلاعات بیشتر' },
  { value: 'md', labelFa: 'استاندارد', descriptionFa: 'اندازه متناسب برای خوانایی مطلوب' },
  { value: 'lg', labelFa: 'بزرگ و خوانا', descriptionFa: 'تایپوگرافی درشت‌تر و دکمه‌های بزرگ‌تر' },
] as const;

export function isValidHexColor(color: string): boolean {
  return HEX_COLOR_REGEX.test(color);
}

export function isValidRadius(radius: string): radius is ThemeRadius {
  return (VALID_RADII as readonly string[]).includes(radius);
}

export function isValidScale(scale: string): scale is ThemeScale {
  return (VALID_SCALES as readonly string[]).includes(scale);
}
