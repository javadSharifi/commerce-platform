import type { SiteTemplate } from './contract.js';
import { minimalTemplate } from './minimal/index.js';
import { clinicalTemplate } from './clinical/index.js';
import { mobileAppTemplate } from './mobile-app/index.js';
import { editorialTemplate } from './editorial/index.js';

export const TEMPLATES_REGISTRY: Record<string, SiteTemplate> = Object.freeze({
  minimal: minimalTemplate,
  clinical: clinicalTemplate,
  'mobile-app': mobileAppTemplate,
  editorial: editorialTemplate,
});

export const DEFAULT_TEMPLATE = minimalTemplate;

/**
 * Checks whether a template ID is strictly registered as an own property.
 * Safely rejects inherited object prototype properties (e.g. toString, __proto__, valueOf, constructor).
 */
export function isTemplateRegistered(templateId?: string | null): boolean {
  if (typeof templateId !== 'string' || !templateId) {
    return false;
  }
  return Object.prototype.hasOwnProperty.call(TEMPLATES_REGISTRY, templateId);
}

/**
 * Resolves a SiteTemplate by ID with guaranteed safe fallback to minimalTemplate.
 * Only selects registered template IDs; inherited object prototype properties and
 * unknown IDs safely fall back to minimal without throwing or crashing SSR.
 */
export function resolveTemplate(templateId?: string | null): SiteTemplate {
  if (!isTemplateRegistered(templateId)) {
    return DEFAULT_TEMPLATE;
  }
  const resolved = TEMPLATES_REGISTRY[templateId!];
  return resolved && typeof resolved === 'object' && resolved.components ? resolved : DEFAULT_TEMPLATE;
}
