/**
 * ViewModels for Site Templates (Phase 5 - PSY-5.4)
 *
 * Rules:
 * - Templates are purely visual and dumb (80/20 rule, Pattern 04).
 * - No API imports, no fetch, no raw dates or financial logic inside templates.
 * - All data is passed as pre-formatted ViewModels from pages.
 */

export interface NavItemVM {
  label: string;
  href: string;
  isActive?: boolean;
}

export interface SiteHeaderVM {
  clinicTitle: string;
  navItems: NavItemVM[];
  primaryAction?: {
    label: string;
    href: string;
  };
}

export interface SiteShellVM {
  clinicTitle: string;
  headerVM: SiteHeaderVM;
  footerVM: FooterVM;
  currentPath: string;
  hideHeaderFooter?: boolean;
}

export interface FooterVM {
  clinicName: string;
  copyrightYear: string;
  phone?: string;
  address?: string;
  description?: string;
}

export interface HomeHeroVM {
  badgeText?: string;
  title: string;
  description: string;
  primaryCta: {
    label: string;
    href: string;
  };
  secondaryCta?: {
    label: string;
    href: string;
  };
}

export interface CourseCardVM {
  id: string;
  slug: string;
  title: string;
  instructorName: string;
  priceLabel: string;
  lessonsCountLabel?: string;
  coverUrl?: string | null;
}

export interface WorkshopCardVM {
  id: string;
  title: string;
  instructorName: string;
  priceLabel: string;
  dateLabel: string;
  capacityLabel: string;
  coverUrl?: string | null;
}

export interface BlogCardVM {
  id: string;
  slug: string;
  title: string;
  excerpt: string;
  dateLabel: string;
  coverUrl?: string | null;
}
