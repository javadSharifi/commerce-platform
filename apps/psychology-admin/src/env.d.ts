/// <reference path="../.astro/types.d.ts" />

declare namespace App {
  interface Locals {
    user?: {
      id?: string;
      full_name: string;
      email?: string;
      role?: string;
    };
    activeTenant?: {
      id: string;
      name: string;
      slug: string;
      role: string;
    };
    memberships?: Array<{
      id: string;
      tenant_id: string;
      tenant_name: string;
      tenant_slug: string;
      role: string;
    }>;
  }
}

declare module '*.astro' {
  const component: (_props: any) => any;
  export default component;
}
