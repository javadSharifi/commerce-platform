import { TEMPLATE_CATALOG } from '@psychology/ui';
import type { SiteTemplate } from '../contract.js';
import Header from './Header.astro';
import Footer from './Footer.astro';
import HomeHero from './HomeHero.astro';
import CourseCard from './CourseCard.astro';
import WorkshopCard from './WorkshopCard.astro';
import BlogCard from './BlogCard.astro';

const meta = TEMPLATE_CATALOG.find((t) => t.id === 'clinical')!;

export const clinicalTemplate: SiteTemplate = {
  meta,
  components: {
    Header,
    Footer,
    HomeHero,
    CourseCard,
    WorkshopCard,
    BlogCard,
  },
};

export default clinicalTemplate;
