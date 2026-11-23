import { EDITORIAL_TEMPLATE_META } from '@psychology/ui';
import type { SiteTemplate } from '../contract.js';
import Header from './Header.astro';
import Footer from './Footer.astro';
import HomeHero from './HomeHero.astro';
import CourseCard from './CourseCard.astro';
import WorkshopCard from './WorkshopCard.astro';
import BlogCard from './BlogCard.astro';
import Shell from './Shell.astro';

export const editorialTemplate: SiteTemplate = {
  meta: EDITORIAL_TEMPLATE_META,
  components: {
    Header,
    Footer,
    HomeHero,
    CourseCard,
    WorkshopCard,
    BlogCard,
    Shell,
  },
};

export default editorialTemplate;
