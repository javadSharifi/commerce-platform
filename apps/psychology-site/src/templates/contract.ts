import type { TemplateMeta } from '@psychology/ui';
import type { AstroComponentFactory } from 'astro/runtime/server/index.js';

export interface SiteTemplate {
  meta: TemplateMeta;
  components: {
    Header: AstroComponentFactory;
    Footer: AstroComponentFactory;
    HomeHero: AstroComponentFactory;
    CourseCard: AstroComponentFactory;
    WorkshopCard: AstroComponentFactory;
    BlogCard: AstroComponentFactory;
  };
}
