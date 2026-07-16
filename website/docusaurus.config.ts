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
        blog: false,
        theme: {customCss: './src/css/custom.css'},
      } satisfies Preset.Options,
    ],
  ],
  themeConfig: {
    navbar: {
      title: 'platform-go',
      items: [{to: '/docs/intro', label: '文档', position: 'left'}, {href: 'https://github.com/GnosiST/platform-go', label: 'GitHub', position: 'right'}],
    },
    metadata: [{name: 'description', content: 'Business-neutral Go operations platform foundation'}],
  } satisfies Preset.ThemeConfig,
};

export default config;
