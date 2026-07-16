import type {Config} from '@docusaurus/types';
import type * as Preset from '@docusaurus/preset-classic';

const config: Config = {
  title: 'platform-go',
  tagline: '面向 Go 服务的业务中立运营平台底座 / A business-neutral Go operations foundation',
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
        {to: '/docs/intro', label: '文档 / Docs', position: 'left'},
        {to: '/docs/capabilities', label: '能力 / Capabilities', position: 'left'},
        {to: '/docs/operations', label: '运维 / Operations', position: 'left'},
        {type: 'localeDropdown', position: 'right'},
        {label: 'v0.1.0', href: 'https://github.com/GnosiST/platform-go/releases/tag/v0.1.0', position: 'right'},
        {href: 'https://github.com/GnosiST/platform-go', label: 'GitHub', position: 'right'},
      ],
    },
    metadata: [
      {
        name: 'description',
        content: '面向 Go 服务的业务中立运营平台底座，提供能力清单、认证、RBAC、资源合同与生产治理。 Business-neutral Go foundation for reusable operations services.',
      },
      {name: 'keywords', content: 'Go, Gin, GORM, Casbin, Refine, platform foundation, RBAC, capability manifests'},
    ],
    footer: {
      style: 'dark',
      links: [
        {title: '项目 / Project', items: [{label: 'GitHub', href: 'https://github.com/GnosiST/platform-go'}, {label: '发行版本 / Releases', href: 'https://github.com/GnosiST/platform-go/releases'}]},
        {title: '文档 / Docs', items: [{label: '快速开始 / Quick start', to: '/docs/intro'}, {label: '能力与扩展 / Capabilities', to: '/docs/capabilities'}, {label: '运维与安全 / Operations', to: '/docs/operations'}]},
      ],
      copyright: `Copyright © ${new Date().getFullYear()} GnosiST. Built with Docusaurus.`,
    },
  } satisfies Preset.ThemeConfig,
};

export default config;
