/// <reference path="../.astro/types.d.ts" />

declare namespace App {
  interface Locals {
    site?: import('@psychology/api-client').PublicSiteResponse;
  }
}

declare module '*.astro' {
  const component: (_props: any) => any;
  export default component;
}
