import type {SidebarsConfig} from '@docusaurus/plugin-content-docs';

const sidebars: SidebarsConfig = {
  tutorialSidebar: [
    {type: 'category', label: '开始使用 / Start here', items: ['intro', 'architecture', 'development', 'human-ai-development']},
    {type: 'category', label: '平台能力 / Platform', items: ['capabilities', 'api', 'security']},
    {type: 'category', label: '运行与社区 / Run & community', items: ['operations', 'deployment', 'roadmap', 'faq']},
  ],
};

export default sidebars;
