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
        docs: {
          routeBasePath: 'docs',
          sidebarPath: './sidebars.ts',
          showLastUpdateTime: true,
          breadcrumbs: true,
        },
        blog: false,
        sitemap: {
          changefreq: 'weekly',
          priority: 0.7,
        },
        theme: {customCss: './src/css/custom.css'},
      } satisfies Preset.Options,
    ],
  ],
  headTags: [
    {
      tagName: 'meta',
      attributes: {
        name: 'theme-color',
        content: '#0b1220',
      },
    },
    {
      tagName: 'meta',
      attributes: {
        property: 'og:type',
        content: 'website',
      },
    },
    {
      tagName: 'meta',
      attributes: {
        property: 'og:site_name',
        content: 'platform-go',
      },
    },
    {
      tagName: 'meta',
      attributes: {
        name: 'twitter:card',
        content: 'summary_large_image',
      },
    },
    {
      tagName: 'link',
      attributes: {
        rel: 'me',
        href: 'https://github.com/GnosiST/platform-go',
      },
    },
  ],
  themeConfig: {
    image: 'img/platform-go-social.svg',
    navbar: {
      title: 'platform-go',
      items: [
        {to: '/docs/intro', label: '文档', position: 'left'},
        {href: 'https://github.com/GnosiST/platform-go', label: 'GitHub', position: 'right'},
      ],
    },
    metadata: [
      {
        name: 'description',
        content: 'Business-neutral Go platform foundation for capability manifests, authentication, RBAC, resource contracts and operations.',
      },
      {name: 'keywords', content: 'Go, Gin, GORM, Casbin, Refine, platform foundation, RBAC, capability manifests'},
    ],
    footer: {
      style: 'dark',
      links: [
        {title: '项目', items: [{label: 'GitHub', href: 'https://github.com/GnosiST/platform-go'}]},
        {title: '文档', items: [{label: '快速开始', to: '/docs/intro'}, {label: '能力与扩展', to: '/docs/capabilities'}]},
      ],
      copyright: `Copyright © ${new Date().getFullYear()} GnosiST. Built with Docusaurus.`,
    },
  } satisfies Preset.ThemeConfig,
};

export default config;
