/**
 * Palette Catalog - Central Metadata and Semantic Tokens for Platform Palettes
 *
 * Complies with AGENTS.md rule 12 and DECISIONS.md (PSY-D10):
 * - calm-sage is the default
 * - No user arbitrary code or arbitrary color injection
 * - Maps to semantic DaisyUI tokens
 */

export interface PaletteTokens {
  colorBase100: string;
  colorBase200: string;
  colorBase300: string;
  colorSurfaceCard?: string;
  colorBaseContent: string;
  colorPrimary: string;
  colorPrimaryContent: string;
  colorSecondary: string;
  colorSecondaryContent: string;
  colorAccent: string;
  colorAccentContent: string;
  colorNeutral: string;
  colorNeutralContent: string;
}

export interface PaletteMeta {
  id: string;
  labelFa: string; // Persian display name for admin panel
  descriptionFa: string; // Persian explanation of mood/tone
  primaryColor: string; // Primary hex color for badges and swatches
  tokens: PaletteTokens;
}

export const DEFAULT_PALETTE_ID = 'calm-sage';

export const PALETTE_CATALOG: readonly PaletteMeta[] = [
  {
    id: 'calm-sage',
    labelFa: 'مریم‌گلی آرامش‌بخش (پیش‌فرض)',
    descriptionFa: 'طیف سبز زیتونی و مریم‌گلی برای احساس آرامش، رهایی از استرس و امنیت روانی',
    primaryColor: '#3b7a57',
    tokens: {
      colorBase100: '#f6f8f6',
      colorBase200: '#ebf0eb',
      colorBase300: '#dde5dd',
      colorBaseContent: '#1f2937',
      colorPrimary: '#3b7a57',
      colorPrimaryContent: '#ffffff',
      colorSecondary: '#5c8374',
      colorSecondaryContent: '#ffffff',
      colorAccent: '#84a98c',
      colorAccentContent: '#ffffff',
      colorNeutral: '#2d3748',
      colorNeutralContent: '#f7fafc',
    },
  },
  {
    id: 'ocean-trust',
    labelFa: 'آبی اقیانوسی (اعتماد)',
    descriptionFa: 'طیف آبی عمیق اقیانوسی نماد تعهد، تخصص، شفافیت و اعتماد حرفه‌ای',
    primaryColor: '#0369a1',
    tokens: {
      colorBase100: '#f0f7fc',
      colorBase200: '#e2eff8',
      colorBase300: '#cde2f3',
      colorBaseContent: '#0f172a',
      colorPrimary: '#0369a1',
      colorPrimaryContent: '#ffffff',
      colorSecondary: '#38bdf8',
      colorSecondaryContent: '#0f172a',
      colorAccent: '#0284c7',
      colorAccentContent: '#ffffff',
      colorNeutral: '#1e293b',
      colorNeutralContent: '#f8fafc',
    },
  },
  {
    id: 'warm-sand',
    labelFa: 'شن‌های گرم (صمیمیت)',
    descriptionFa: 'رنگ‌های گرم خاکی و کرم برای فضایی صمیمی، دلپذیر، همدلانه و پذیرا',
    primaryColor: '#b45309',
    tokens: {
      colorBase100: '#faf6f0',
      colorBase200: '#f5ede2',
      colorBase300: '#eedfcb',
      colorBaseContent: '#292524',
      colorPrimary: '#b45309',
      colorPrimaryContent: '#ffffff',
      colorSecondary: '#d97706',
      colorSecondaryContent: '#ffffff',
      colorAccent: '#f59e0b',
      colorAccentContent: '#ffffff',
      colorNeutral: '#451a03',
      colorNeutralContent: '#fef3c7',
    },
  },
  {
    id: 'royal-plum',
    labelFa: 'آلوی سلطنتی (بینش)',
    descriptionFa: 'طیف بنفش و ارغوانی متین نماد عمق روان‌شناختی، خودآگاهی و بلوغ فردی',
    primaryColor: '#7e22ce',
    tokens: {
      colorBase100: '#faf5ff',
      colorBase200: '#f3e8ff',
      colorBase300: '#e9d5ff',
      colorBaseContent: '#1e1b4b',
      colorPrimary: '#7e22ce',
      colorPrimaryContent: '#ffffff',
      colorSecondary: '#a855f7',
      colorSecondaryContent: '#ffffff',
      colorAccent: '#c084fc',
      colorAccentContent: '#ffffff',
      colorNeutral: '#3b0764',
      colorNeutralContent: '#faf5ff',
    },
  },
  {
    id: 'stitch-editorial',
    labelFa: 'استیچ',
    descriptionFa: 'زمینهٔ گرم روشن، کارت سفید، سبز تیره و تأکید سفالی',
    primaryColor: '#2d4f43',
    tokens: {
      colorBase100: '#faf9f6',
      colorBase200: '#f3efe9',
      colorBase300: '#eae6df',
      colorSurfaceCard: '#ffffff',
      colorBaseContent: '#1e2422',
      colorPrimary: '#2d4f43',
      colorPrimaryContent: '#ffffff',
      colorSecondary: '#e8dfd5',
      colorSecondaryContent: '#1e2422',
      colorAccent: '#c9825b',
      colorAccentContent: '#2c2c2c',
      colorNeutral: '#1e2422',
      colorNeutralContent: '#faf9f6',
    },
  },
] as const;

export const VALID_PALETTE_IDS = PALETTE_CATALOG.map((p) => p.id) as readonly string[];
export type SupportedPaletteId = 'calm-sage' | 'ocean-trust' | 'warm-sand' | 'royal-plum' | 'stitch-editorial';

const PALETTE_MAP: Record<string, PaletteMeta> = Object.freeze(
  Object.fromEntries(PALETTE_CATALOG.map((p) => [p.id, p]))
);

/**
 * Checks whether a palette ID is strictly registered as an own property.
 * Safely rejects inherited object prototype properties (e.g. toString, valueOf, constructor).
 */
export function isValidPaletteId(id?: string | null): id is SupportedPaletteId {
  if (typeof id !== 'string' || !id) {
    return false;
  }
  return Object.prototype.hasOwnProperty.call(PALETTE_MAP, id);
}

export function getPaletteMeta(id: string): PaletteMeta | undefined {
  if (isValidPaletteId(id)) {
    return PALETTE_MAP[id];
  }
  return undefined;
}

/**
 * Resolves palette metadata with safe fallback to DEFAULT_PALETTE_ID.
 * Never throws or crashes on unknown or inherited prototype keys.
 */
export function resolvePaletteMeta(id?: string | null): PaletteMeta {
  if (!id || !isValidPaletteId(id)) {
    return PALETTE_CATALOG[0];
  }
  return PALETTE_MAP[id] || PALETTE_CATALOG[0];
}
