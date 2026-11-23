/**
 * Template Catalog - Central Metadata for Platform Templates
 *
 * Provides shared metadata for both public sites and admin settings.
 * Complies with AGENTS.md rule 12 (versioned templates, no user arbitrary code).
 */

export interface TemplateMeta {
  id: string;
  version: string; // e.g. "1.0.0" (matching rule 12 of AGENTS.md)
  labelFa: string; // Persian display name in admin panel
  descriptionFa: string; // Short explanation for psychologist
  daisyTheme: string; // name of [data-theme] in theme.css for this template
}

export const DEFAULT_TEMPLATE_ID = 'minimal';

export const EDITORIAL_TEMPLATE_META: TemplateMeta = {
  id: 'editorial',
  version: '1.0.0',
  labelFa: 'ادیتوریال',
  descriptionFa: 'طراحی آرامش‌بخش، گرم و مبتنی بر روانشناسی با چیدمان ژورنالی و فونت خوانا',
  daisyTheme: 'editorial',
};

export const TEMPLATE_CATALOG: readonly TemplateMeta[] = [
  {
    id: 'minimal',
    version: '1.0.0',
    labelFa: 'مینیمال',
    descriptionFa: 'طراحی خلوت، ساده و آرامش‌بخش مناسب مطب و درمانگران فردی',
    daisyTheme: 'minimal',
  },
  {
    id: 'clinical',
    version: '1.0.0',
    labelFa: 'کلینیکال',
    descriptionFa: 'طراحی ساختاریافته، معتبر و مدرن با تمرکز بر خدمات کلینیک و درمانگران متعدد',
    daisyTheme: 'clinical',
  },
  {
    id: 'mobile-app',
    version: '1.0.0',
    labelFa: 'اپلیکیشن موبایل',
    descriptionFa: 'چیدمان بهینه و ناوبری اپ‌گونه با نوار دسترسی پایین صفحه مناسب تعامل لمسی موبایل',
    daisyTheme: 'mobile-app',
  },
  EDITORIAL_TEMPLATE_META,
] as const;

export const VALID_TEMPLATE_IDS = TEMPLATE_CATALOG.map((t) => t.id) as readonly string[];
export type SupportedTemplateId = 'minimal' | 'clinical' | 'mobile-app' | 'editorial';

export function isValidTemplateId(id: string): id is SupportedTemplateId {
  return VALID_TEMPLATE_IDS.includes(id);
}

export function getTemplateMeta(id: string): TemplateMeta | undefined {
  if (id === EDITORIAL_TEMPLATE_META.id) {
    return EDITORIAL_TEMPLATE_META;
  }
  return TEMPLATE_CATALOG.find((t) => t.id === id);
}

/**
 * Resolves template metadata with safe fallback to DEFAULT_TEMPLATE_ID.
 * Never throws or crashes on unknown or missing template IDs.
 */
export function resolveTemplateMeta(id?: string | null): TemplateMeta {
  if (!id) {
    return TEMPLATE_CATALOG[0];
  }
  const found = getTemplateMeta(id);
  return found || TEMPLATE_CATALOG[0];
}
