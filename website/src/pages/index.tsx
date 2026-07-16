import React from 'react';
import Link from '@docusaurus/Link';

const capabilities = [
  {
    index: '01',
    title: '能力清单',
    text: '用可校验的 manifest 描述资源、路由、权限、生命周期与稳定性。',
    link: '/docs/capabilities',
    label: '查看能力模型',
  },
  {
    index: '02',
    title: '安全边界',
    text: 'JWT 会话、Casbin RBAC、租户范围和审计默认进入平台底座。',
    link: '/docs/intro',
    label: '阅读快速开始',
  },
  {
    index: '03',
    title: '运行治理',
    text: 'OpenAPI、代码生成、迁移检查与发布证据让每次扩展都可追溯。',
    link: '/docs/operations',
    label: '查看运维手册',
  },
];

export default function Home(): JSX.Element {
  return (
    <main className="platform-home">
      <section className="platform-hero">
        <div className="container platform-hero__grid">
          <div className="platform-hero__copy">
            <p className="platform-kicker"><span className="platform-kicker__mark" aria-hidden="true" />GO OPERATIONS FOUNDATION / 0.1</p>
            <h1>先把平台层做好，业务才能跑得更快。</h1>
            <p className="platform-hero__lede">
              面向 Go 服务的业务中立底座，把能力、身份、授权、数据合同和生产运维收进一套可扩展的系统边界。
            </p>
            <div className="platform-actions" aria-label="主要操作">
              <Link className="button button--primary button--lg" to="/docs/intro">开始阅读文档 <span aria-hidden="true">↗</span></Link>
              <a className="button button--secondary button--lg" href="https://github.com/GnosiST/platform-go">在 GitHub 查看 <span aria-hidden="true">↗</span></a>
            </div>
            <div className="platform-hero__meta" aria-label="项目状态">
              <span><i className="platform-status-dot" aria-hidden="true" />v0.1.0 已发布</span>
              <span>MIT License</span>
              <span>Go + Gin + GORM</span>
            </div>
          </div>

          <div className="platform-hero__visual" aria-label="请求从身份进入数据运行时的架构路径">
            <div className="platform-hero__visual-head">
              <span>REQUEST / DATA PATH</span>
              <span className="platform-code-tag">platform-go</span>
            </div>
            <div className="platform-flow">
              <div className="platform-flow__node"><b>01</b><span>Identity</span><small>可信身份</small></div>
              <div className="platform-flow__connector" aria-hidden="true" />
              <div className="platform-flow__node"><b>02</b><span>Capability</span><small>服务合同</small></div>
              <div className="platform-flow__connector" aria-hidden="true" />
              <div className="platform-flow__node platform-flow__node--accent"><b>03</b><span>Policy</span><small>租户与权限</small></div>
              <div className="platform-flow__connector" aria-hidden="true" />
              <div className="platform-flow__node"><b>04</b><span>Runtime</span><small>GORM 数据层</small></div>
            </div>
            <div className="platform-hero__visual-foot"><span>request_id</span><code>ctx → contract → scope → store</code></div>
          </div>
        </div>
      </section>

      <section className="platform-section platform-section--overview">
        <div className="container">
          <div className="platform-section__heading platform-section__heading--row">
            <div><p className="platform-kicker">WHAT SHIPS IN THE BASE</p><h2>合同先于定制。</h2></div>
            <p>把重复的基础设施收敛成边界清晰的能力，让下游团队把时间留给真正的业务。</p>
          </div>
          <div className="platform-capabilities">
            {capabilities.map((item) => (
              <article className="platform-capability" key={item.index}>
                <div className="platform-capability__top"><span>{item.index}</span><span aria-hidden="true">↗</span></div>
                <h3>{item.title}</h3>
                <p>{item.text}</p>
                <Link to={item.link}>{item.label} <span aria-hidden="true">→</span></Link>
              </article>
            ))}
          </div>
        </div>
      </section>

      <section className="platform-section platform-section--dark">
        <div className="container platform-boundary">
          <div className="platform-boundary__intro"><p className="platform-kicker">OPEN BY DESIGN</p><h2>让业务代码停在边界之外。</h2><p>平台负责共享机制，业务能力负责自己的数据、工作流和界面。通过 ports 接入，不把具体存储和壳层细节带进业务包。</p></div>
          <div className="platform-boundary__list" aria-label="平台边界原则">
            <div><span>01</span><strong>可移植</strong><p>SQL-less 服务合同与适配器边界，保持基础设施可替换。</p></div>
            <div><span>02</span><strong>可验证</strong><p>每个能力都对应生成物、测试与发布证据，而不是口头约定。</p></div>
            <div><span>03</span><strong>可扩展</strong><p>从一个能力包开始，按需接入认证、存储、队列和搜索。</p></div>
          </div>
        </div>
      </section>

      <section className="platform-section platform-section--closing">
        <div className="container platform-closing">
          <div><p className="platform-kicker">NEXT STEP</p><h2>从一份清晰的合同开始。</h2></div>
          <div className="platform-closing__actions"><Link className="button button--primary button--lg" to="/docs/intro">阅读快速开始 <span aria-hidden="true">↗</span></Link><Link className="platform-text-link" to="/docs/capabilities">浏览全部能力 <span aria-hidden="true">→</span></Link></div>
        </div>
      </section>
    </main>
  );
}
