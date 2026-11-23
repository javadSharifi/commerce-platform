import type { APIRoute } from 'astro';
import { TEMPLATES_REGISTRY } from '../../templates/registry.js';
import { TEMPLATE_CATALOG } from '@psychology/ui';

export const GET: APIRoute = async ({ url }) => {
  const includeAll = url.searchParams.get('all') === 'true';
  const templates = Object.keys(TEMPLATES_REGISTRY)
    .filter((id) => includeAll || TEMPLATE_CATALOG.some((c) => c.id === id))
    .map((id) => {
    const template = TEMPLATES_REGISTRY[id];
    return {
      id,
      meta: template.meta,
      components: {
        hasHeader: typeof template.components.Header !== 'undefined',
        hasFooter: typeof template.components.Footer !== 'undefined',
        hasHomeHero: typeof template.components.HomeHero !== 'undefined',
        hasCourseCard: typeof template.components.CourseCard !== 'undefined',
        hasWorkshopCard: typeof template.components.WorkshopCard !== 'undefined',
        hasBlogCard: typeof template.components.BlogCard !== 'undefined',
        hasShell: typeof template.components.Shell !== 'undefined',
      },
    };
  });

  return new Response(JSON.stringify(templates), {
    status: 200,
    headers: {
      'Content-Type': 'application/json',
      'Cache-Control': 'no-store',
    },
  });
};
