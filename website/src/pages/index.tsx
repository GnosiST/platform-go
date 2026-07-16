import React from 'react';
import Link from '@docusaurus/Link';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';

const capabilityCards = [
  {index: '01', zh: '身份与授权', en: 'Identity & access', textZh: 'JWT 会话、Casbin RBAC、租户范围、组织与菜单边界，从第一天就可审计。', textEn: 'JWT sessions, Casbin RBAC, tenant scopes, organizations and menu boundaries with auditability built in.', link: '/docs/intro', linkZh: '开始阅读', linkEn: 'Get started'},
  {index: '02', zh: '能力合同', en: 'Capability contracts', textZh: '用 manifest 描述资源、路由、权限、生命周期和稳定性，让业务包按合同接入。', textEn: 'Manifests describe resources, routes, permissions, lifecycle and stability so business packages attach by contract.', link: '/docs/capabilities', linkZh: '查看能力模型', linkEn: 'Explore capabilities'},
  {index: '03', zh: '运行治理', en: 'Runtime governance', textZh: 'OpenAPI、代码生成、迁移检查和发布证据，让每次扩展都可验证、可追溯。', textEn: 'OpenAPI, code generation, migration checks and release evidence keep every extension verifiable.', link: '/docs/operations', linkZh: '查看运维手册', linkEn: 'Read operations guide'},
];

export default function Home(): JSX.Element {
  const {i18n} = useDocusaurusContext();
  const en = i18n.currentLocale === 'en';
  const text = (zh: string, english: string) => en ? english : zh;
  return (
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
              <span><i className="platform-status-dot" aria-hidden="true" />{text('v0.1.0 已发布', 'v0.1.0 released')}</span><span>Apache-2.0</span><span>Go + Gin + GORM</span>
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

      <section className="platform-section platform-section--overview">
        <div className="container">
          <div className="platform-section__heading platform-section__heading--row"><div><p className="platform-kicker">{text('默认发行版', 'WHAT SHIPS IN THE BASE')}</p><h2>{text('合同先于定制。', 'Contracts before customization.')}</h2></div><p>{text('把重复的基础设施收敛成边界清晰的能力，让下游团队把时间留给真正的业务。', 'Shared infrastructure becomes clear, testable capabilities so downstream teams can focus on their domain.')}</p></div>
          <div className="platform-capabilities">{capabilityCards.map((item) => <article className="platform-capability" key={item.index}><div className="platform-capability__top"><span>{item.index}</span><span aria-hidden="true">↗</span></div><h3>{text(item.zh, item.en)}</h3><p>{text(item.textZh, item.textEn)}</p><Link to={item.link}>{text(item.linkZh, item.linkEn)} <span aria-hidden="true">→</span></Link></article>)}</div>
        </div>
      </section>

      <section className="platform-section platform-section--dark"><div className="container platform-boundary"><div className="platform-boundary__intro"><p className="platform-kicker">OPEN BY DESIGN</p><h2>{text('让业务代码停在边界之外。', 'Keep business code outside the platform boundary.')}</h2><p>{text('平台负责共享机制，业务能力负责自己的数据、工作流和界面。通过 ports 接入，不把具体存储和壳层细节带进业务包。', 'The platform owns shared mechanisms; business capabilities own their data, workflows and UI. Public ports keep storage and shell details out of business packages.')}</p></div><div className="platform-boundary__list" aria-label={text('平台边界原则', 'Platform boundary principles')}><div><span>01</span><strong>{text('可移植', 'Portable')}</strong><p>{text('适配器边界保持基础设施可替换。', 'Adapter boundaries keep infrastructure replaceable.')}</p></div><div><span>02</span><strong>{text('可验证', 'Verifiable')}</strong><p>{text('每个能力对应生成物、测试与发布证据。', 'Every capability maps to artifacts, tests and release evidence.')}</p></div><div><span>03</span><strong>{text('可扩展', 'Extensible')}</strong><p>{text('按需接入认证、存储、队列和搜索。', 'Add authentication, storage, queues and search when needed.')}</p></div></div></div></section>

      <section className="platform-section platform-section--closing"><div className="container platform-closing"><div><p className="platform-kicker">NEXT STEP / 下一步</p><h2>{text('从一份清晰的合同开始。', 'Start with a clear contract.')}</h2></div><div className="platform-closing__actions"><Link className="button button--primary button--lg" to="/docs/intro">{text('阅读快速开始', 'Read the quick start')} <span aria-hidden="true">↗</span></Link><Link className="platform-text-link" to="/docs/capabilities">{text('浏览全部能力', 'Browse capabilities')} <span aria-hidden="true">→</span></Link></div></div></section>
    </main>
  );
}
