/// <reference path="../.astro/types.d.ts" />

declare module '*.astro' {
  const component: (_props: any) => any;
  export default component;
}
