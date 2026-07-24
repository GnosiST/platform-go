import type {SidebarsConfig} from '@docusaurus/plugin-content-docs';

const sidebars: SidebarsConfig = {
  tutorialSidebar: [
    {type: 'category', label: '开始使用 / Start', items: ['intro', 'getting-started', 'business-integration', 'architecture']},
    {type: 'category', label: '核心概念 / Core', items: ['capabilities', 'authentication', 'api', 'security', 'support-matrix']},
    {type: 'category', label: '开发 / Build', items: ['development', 'human-ai-development']},
    {type: 'category', label: '运行 / Run', items: ['configuration', 'deployment', 'production-checklist', 'operations', 'roadmap', 'faq']},
  ],
};

export default sidebars;
