import type {Config} from '@docusaurus/types';
import type * as Preset from '@docusaurus/preset-classic';

const config: Config = {
  title: 'platform-go',
  tagline: 'A business-neutral Go operations platform foundation',
  favicon: 'img/favicon.svg',
  url: 'https://gnosist.github.io',
  baseUrl: '/platform-go/',
  organizationName: 'GnosiST',
  projectName: 'platform-go',
  onBrokenLinks: 'throw',
  onBrokenMarkdownLinks: 'throw',
  i18n: {defaultLocale: 'zh-Hans', locales: ['zh-Hans', 'en']},
  presets: [
    [
      'classic',
      {
        docs: {routeBasePath: 'docs', sidebarPath: './sidebars.ts'},
        sitemap: {changefreq: 'weekly', priority: 0.7},
        blog: false,
        theme: {customCss: './src/css/custom.css'},
      } satisfies Preset.Options,
    ],
  ],
  themeConfig: {
    image: 'img/favicon.svg',
    navbar: {
      title: 'platform-go',
      items: [{to: '/docs/intro', label: '文档', position: 'left'}, {href: 'https://github.com/GnosiST/platform-go', label: 'GitHub', position: 'right'}],
    },
    metadata: [
      {name: 'description', content: 'Business-neutral Go operations platform foundation for capabilities, identity, authorization and production operations.'},
      {property: 'og:type', content: 'website'},
      {property: 'og:title', content: 'platform-go | Go operations foundation'},
      {property: 'og:description', content: 'Contracts, secure defaults and production gates for reusable Go services.'},
      {name: 'twitter:card', content: 'summary'},
    ],
  } satisfies Preset.ThemeConfig,
};

export default config;
