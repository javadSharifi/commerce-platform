export interface ThemeConfig {
  id: string;
  name: string;
  version: string;
  defaultPrimaryColor: string;
  fontFamily: string;
}

export const defaultTheme: ThemeConfig = {
  id: 'clinic-standard',
  name: 'پوسته استاندارد کلینیک روان‌شناسی',
  version: '1.0.0',
  defaultPrimaryColor: '#0284c7',
  fontFamily: 'system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif',
};
