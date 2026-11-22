import type { SiteTemplate } from './contract.js';
import { minimalTemplate } from './minimal/index.js';
import { clinicalTemplate } from './clinical/index.js';

export const TEMPLATES_REGISTRY: Record<string, SiteTemplate> = {
  minimal: minimalTemplate,
  clinical: clinicalTemplate,
};

export const DEFAULT_TEMPLATE = minimalTemplate;

/**
 * Resolves a SiteTemplate by ID with guaranteed safe fallback to minimalTemplate.
 * Never throws on undefined, null, empty, or unknown template IDs.
 */
export function resolveTemplate(templateId?: string | null): SiteTemplate {
  if (!templateId) {
    return DEFAULT_TEMPLATE;
  }
  return TEMPLATES_REGISTRY[templateId] ?? DEFAULT_TEMPLATE;
}

export function isTemplateRegistered(templateId: string): boolean {
  return templateId in TEMPLATES_REGISTRY;
}
