import React from 'react';
import Link from '@docusaurus/Link';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import useBaseUrl from '@docusaurus/useBaseUrl';
import Layout from '@theme/Layout';

const capabilityCards = [
  {index: '01', zh: '身份与授权', en: 'Identity & access', textZh: 'JWT 会话、Casbin RBAC、租户范围、组织与菜单边界，从第一天就可审计。', textEn: 'JWT sessions, Casbin RBAC, tenant scopes, organizations and menu boundaries with auditability built in.', link: '/docs/intro', linkZh: '开始阅读', linkEn: 'Get started'},
  {index: '02', zh: '能力合同', en: 'Capability contracts', textZh: '用 manifest 描述资源、路由、权限、生命周期和稳定性，让业务包按合同接入。', textEn: 'Manifests describe resources, routes, permissions, lifecycle and stability so business packages attach by contract.', link: '/docs/capabilities', linkZh: '查看能力模型', linkEn: 'Explore capabilities'},
  {index: '03', zh: '运行治理', en: 'Runtime governance', textZh: 'OpenAPI、代码生成、迁移检查和发布证据，让每次扩展都可验证、可追溯。', textEn: 'OpenAPI, code generation, migration checks and release evidence keep every extension verifiable.', link: '/docs/operations', linkZh: '查看运维手册', linkEn: 'Read operations guide'},
];

export default function Home(): JSX.Element {
  const {i18n} = useDocusaurusContext();
  const en = i18n.currentLocale === 'en';
  const text = (zh: string, english: string) => en ? english : zh;
  const pageTitle = text('Go 运营平台底座', 'Go operations foundation');
  const pageDescription = text(
    '面向 Go 服务的业务中立底座，提供能力合同、身份授权、数据边界和生产治理。',
    'A business-neutral Go foundation for capability contracts, identity, data boundaries and production governance.',
  );
  const demoVideo = useBaseUrl('video/platform-go-demo.mp4');
  const demoPoster = useBaseUrl('video/platform-go-demo.png');
  return (
    <Layout title={pageTitle} description={pageDescription}>
      <main className="platform-home">
        <section className="platform-hero">
          <div className="container platform-hero__grid">
            <div className="platform-hero__copy">
              <p className="platform-kicker"><span className="platform-kicker__mark" aria-hidden="true" />GO OPERATIONS FOUNDATION / 0.1</p>
              <h1>{text('先把平台层做好，业务才能跑得更快。', 'Build the platform layer first. Let business move faster.')}</h1>
              <p className="platform-hero__lede">{text('面向 Go 服务的业务中立底座，把能力、身份、授权、数据合同和生产运维收进一套可扩展的系统边界。', 'A business-neutral foundation for Go services that brings capabilities, identity, authorization, data contracts and production operations into one extensible boundary.')}</p>
              <div className="platform-actions" aria-label={text('主要操作', 'Primary actions')}>
                <Link className="button button--primary button--lg" to="/docs/intro">{text('开始阅读文档', 'Read the docs')} <span aria-hidden="true">↗</span></Link>
                <a className="button button--secondary button--lg" href="https://github.com/GnosiST/platform-go">{text('在 GitHub 查看', 'View on GitHub')} <span aria-hidden="true">↗</span></a>
              </div>
              <div className="platform-hero__meta" aria-label={text('项目状态', 'Project status')}>
                <span><i className="platform-status-dot" aria-hidden="true" />{text('v0.1.0 待发布', 'v0.1.0 unreleased')}</span><span>Apache-2.0</span><span>Go + Gin + GORM</span>
              </div>
            </div>
            <div className="platform-hero__visual" aria-label={text('请求从身份进入数据运行时的架构路径', 'Architecture path from identity to data runtime')}>
              <div className="platform-hero__visual-head"><span>{text('请求 / 数据路径', 'REQUEST / DATA PATH')}</span><span className="platform-code-tag">platform-go</span></div>
              <div className="platform-flow">
                {[['01', 'Identity', '可信身份 / Trusted identity'], ['02', 'Capability', '服务合同 / Service contract'], ['03', 'Policy', '租户与权限 / Scope & policy'], ['04', 'Runtime', 'GORM 数据层 / Data runtime']].map(([number, title, caption], index) => <React.Fragment key={number}><div className={`platform-flow__node${index === 2 ? ' platform-flow__node--accent' : ''}`}><b>{number}</b><span>{title}</span><small>{caption}</small></div>{index < 3 && <div className="platform-flow__connector" aria-hidden="true" />}</React.Fragment>)}
              </div>
              <div className="platform-hero__visual-foot"><span>request_id</span><code>ctx → contract → scope → store</code></div>
            </div>
          </div>
        </section>

        <section className="platform-section platform-section--demo">
          <div className="container platform-demo">
            <div className="platform-demo__intro">
              <p className="platform-kicker">{text('一分钟看懂平台路径', 'ONE-MINUTE PLATFORM TOUR')}</p>
              <h2>{text('从身份到数据，边界如何工作。', 'See the boundary from identity to data.')}</h2>
              <p>{text('这段演示用一个请求路径串起平台的核心合同：可信身份、能力声明、租户与权限策略，最后才进入数据运行时。', 'This short tour follows one request through the core contracts: trusted identity, capability declarations, tenant and policy scope, and finally the data runtime.')}</p>
            </div>
            <figure className="platform-demo__frame">
              <video controls preload="metadata" poster={demoPoster} aria-label={text('platform-go 运行路径演示视频', 'platform-go runtime path demo')}>
                <source src={demoVideo} type="video/mp4" />
                {text('你的浏览器不支持视频播放，请打开视频文件查看。', 'Your browser does not support video playback. Open the video file directly.')}
              </video>
              <figcaption>{text('Remotion 生成 · 1920×1080 · 12 秒', 'Rendered with Remotion · 1920×1080 · 12 seconds')}</figcaption>
            </figure>
          </div>
        </section>

        <section className="platform-section platform-section--overview">
          <div className="container">
            <div className="platform-section__heading platform-section__heading--row"><div><p className="platform-kicker">{text('默认发行版', 'WHAT SHIPS IN THE BASE')}</p><h2>{text('合同先于定制。', 'Contracts before customization.')}</h2></div><p>{text('把重复的基础设施收敛成边界清晰的能力，让下游团队把时间留给真正的业务。', 'Shared infrastructure becomes clear, testable capabilities so downstream teams can focus on their domain.')}</p></div>
            <div className="platform-capabilities">{capabilityCards.map((item) => <article className="platform-capability" key={item.index}><div className="platform-capability__top"><span>{item.index}</span><span aria-hidden="true">↗</span></div><h3>{text(item.zh, item.en)}</h3><p>{text(item.textZh, item.textEn)}</p><Link to={item.link}>{text(item.linkZh, item.linkEn)} <span aria-hidden="true">→</span></Link></article>)}</div>
          </div>
        </section>

        <section className="platform-section platform-section--dark"><div className="container platform-boundary"><div className="platform-boundary__intro"><p className="platform-kicker">OPEN BY DESIGN</p><h2>{text('让业务代码停在边界之外。', 'Keep business code outside the platform boundary.')}</h2><p>{text('平台负责共享机制，业务能力负责自己的数据、工作流和界面。通过 ports 接入，不把具体存储和壳层细节带进业务包。', 'The platform owns shared mechanisms; business capabilities own their data, workflows and UI. Public ports keep storage and shell details out of business packages.')}</p></div><div className="platform-boundary__list" aria-label={text('平台边界原则', 'Platform boundary principles')}><div><span>01</span><strong>{text('可移植', 'Portable')}</strong><p>{text('适配器边界保持基础设施可替换。', 'Adapter boundaries keep infrastructure replaceable.')}</p></div><div><span>02</span><strong>{text('可验证', 'Verifiable')}</strong><p>{text('每个能力对应生成物、测试与发布证据。', 'Every capability maps to artifacts, tests and release evidence.')}</p></div><div><span>03</span><strong>{text('可扩展', 'Extensible')}</strong><p>{text('按需接入认证、存储、队列和搜索。', 'Add authentication, storage, queues and search when needed.')}</p></div></div></div></section>

        <section className="platform-section platform-section--closing"><div className="container platform-closing"><div><p className="platform-kicker">NEXT STEP / 下一步</p><h2>{text('从一份清晰的合同开始。', 'Start with a clear contract.')}</h2></div><div className="platform-closing__actions"><Link className="button button--primary button--lg" to="/docs/intro">{text('阅读快速开始', 'Read the quick start')} <span aria-hidden="true">↗</span></Link><Link className="platform-text-link" to="/docs/capabilities">{text('浏览全部能力', 'Browse capabilities')} <span aria-hidden="true">→</span></Link></div></div></section>
      </main>
    </Layout>
  );
}
