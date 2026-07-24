import React from 'react';
import Link from '@docusaurus/Link';
import useBaseUrl from '@docusaurus/useBaseUrl';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import Layout from '@theme/Layout';

type Copy = {zh: string; en: string};

const pillars: Array<{number: string; title: Copy; body: Copy; to: string}> = [
  {
    number: '01',
    title: {zh: '合同优先', en: 'Contracts first'},
    body: {zh: 'Manifest、资源 schema、权限和路由先于实现，让 Admin、OpenAPI 和代码生成物保持同步。', en: 'Manifests, resource schemas, permissions, and routes arrive before implementation so Admin, OpenAPI, and generated artifacts stay aligned.'},
    to: '/docs/business-integration',
  },
  {
    number: '02',
    title: {zh: '身份有边界', en: 'Bounded identity'},
    body: {zh: '会话撤销、RBAC、租户范围和 credential-auth 各自有明确所有权，菜单从不替代后端授权。', en: 'Session revocation, RBAC, tenant scope, and credential auth have explicit ownership. Menus never replace backend authorization.'},
    to: '/docs/authentication',
  },
  {
    number: '03',
    title: {zh: '运行可证明', en: 'Provable runtime'},
    body: {zh: '预检、合同校验、发布证据和受控推广边界，防止“本地可跑”被误当成“生产已认证”。', en: 'Preflight, contract checks, release evidence, and controlled promotion boundaries prevent local success from being mistaken for production certification.'},
    to: '/docs/production-checklist',
  },
];

const supportRows: Array<{label: Copy; state: Copy; note: Copy; to: string}> = [
  {label: {zh: '基础平台', en: 'Foundation'}, state: {zh: '默认支持', en: 'Default'}, note: {zh: '身份、RBAC、资源、审计、文件、单数据源', en: 'Identity, RBAC, resources, audit, files, one datasource'}, to: '/docs/capabilities'},
  {label: {zh: '可选能力', en: 'Optional'}, state: {zh: '显式组合', en: 'Explicit profile'}, note: {zh: '通知、任务、OIDC、credential-auth', en: 'Notification, jobs, OIDC, credential auth'}, to: '/docs/support-matrix'},
  {label: {zh: '受控集成', en: 'Controlled'}, state: {zh: '默认关闭', en: 'Default-off'}, note: {zh: '多数据源、Outbox/RabbitMQ、搜索投影', en: 'Multi-datasource, Outbox/RabbitMQ, search projection'}, to: '/docs/support-matrix'},
];

export default function Home(): JSX.Element {
  const {i18n} = useDocusaurusContext();
  const en = i18n.currentLocale === 'en';
  const text = (copy: Copy) => en ? copy.en : copy.zh;
  const demoPoster = useBaseUrl('video/platform-go-demo.png');
  const pageTitle = text({zh: 'platform-go | Go 业务中立平台底座', en: 'platform-go | Business-neutral Go foundation'});
  const pageDescription = text({zh: '用于可组合业务能力、身份授权、Admin 资源合同和生产治理的 Go 平台底座。', en: 'A Go platform foundation for composable business capabilities, identity, Admin resource contracts, and production governance.'});
  const releaseState = text({zh: 'v0.1.0 已发布', en: 'v0.1.0 released'});

  return (
    <Layout title={pageTitle} description={pageDescription}>
      <main className="platform-home">
        <section className="platform-hero" style={{backgroundImage: `url(${demoPoster})`}}>
          <div className="platform-hero__wash" aria-hidden="true" />
          <div className="container platform-hero__content">
            <p className="platform-eyebrow">GO PLATFORM FOUNDATION</p>
            <h1>platform-go</h1>
            <p className="platform-hero__lede">{text({zh: '为 Go 服务建立可组合、可审计、可推广的平台层。', en: 'A composable, auditable, promotable platform layer for Go services.'})}</p>
            <p className="platform-hero__detail">{text({zh: '把身份、RBAC、资源合同、Admin 工作台和生产预检收敛为共享机制。业务能力通过公开 manifest 与端口接入，领域代码始终留在下游。', en: 'Identity, RBAC, resource contracts, an Admin workspace, and production preflight become shared mechanisms. Business capabilities attach through public manifests and ports while domain code stays downstream.'})}</p>
            <div className="platform-actions" aria-label={text({zh: '主要操作', en: 'Primary actions'})}>
              <Link className="button button--primary button--lg" to="/docs/getting-started">{text({zh: '五分钟开始', en: 'Start in five minutes'})}</Link>
              <Link className="button button--secondary button--lg" to="/docs/business-integration">{text({zh: '接入业务能力', en: 'Integrate a capability'})}</Link>
            </div>
            <dl className="platform-hero__facts">
              <div><dt>{releaseState}</dt><dd>{text({zh: '已发布版本', en: 'released version'})}</dd></div>
              <div><dt>67 / 67</dt><dd>{text({zh: '任务图已闭合', en: 'task graph closed'})}</dd></div>
              <div><dt>Apache-2.0</dt><dd>{text({zh: '开源许可', en: 'open-source license'})}</dd></div>
            </dl>
          </div>
        </section>

        <section className="platform-runway">
          <div className="container platform-runway__grid">
            <div>
              <p className="platform-eyebrow">LOCAL SUCCESS PATH</p>
              <h2>{text({zh: '从健康响应到 Admin 工作台。', en: 'From a healthy response to the Admin workspace.'})}</h2>
              <p>{text({zh: '无需猜测默认端口和成功标准。启动 API、运行 Admin，然后确认健康响应和浏览器界面。', en: 'No guessing about ports or success criteria. Start the API, run Admin, then confirm the health response and browser UI.'})}</p>
            </div>
            <pre className="platform-command"><code>{`go run ./cmd/platform-api\n\nnpm --prefix admin run dev\n\ncurl -fsS http://127.0.0.1:9200/api/health`}</code></pre>
            <Link className="platform-arrow-link" to="/docs/getting-started">{text({zh: '查看完整本地运行指南', en: 'Read the local run guide'})}<span aria-hidden="true">→</span></Link>
          </div>
        </section>

        <section className="platform-section">
          <div className="container">
            <div className="platform-heading">
              <p className="platform-eyebrow">HOW THE FOUNDATION WORKS</p>
              <h2>{text({zh: '共享机制，明确边界。', en: 'Shared mechanisms, explicit boundaries.'})}</h2>
              <p>{text({zh: '平台提供可复用能力和合同。业务包拥有自己的数据、状态机、界面与交付节奏。', en: 'The platform supplies reusable capabilities and contracts. Business packages own their data, state machines, UI, and delivery cadence.'})}</p>
            </div>
            <div className="platform-pillars">
              {pillars.map((pillar) => (
                <Link className="platform-pillar" to={pillar.to} key={pillar.number}>
                  <span>{pillar.number}</span>
                  <h3>{text(pillar.title)}</h3>
                  <p>{text(pillar.body)}</p>
                  <b aria-hidden="true">→</b>
                </Link>
              ))}
            </div>
          </div>
        </section>

        <section className="platform-section platform-section--graph">
          <div className="container platform-graph">
            <div className="platform-heading">
              <p className="platform-eyebrow">COMPOSITION MODEL</p>
              <h2>{text({zh: '下游声明能力，平台生成一致的表面。', en: 'Downstream declares capability. Platform generates a consistent surface.'})}</h2>
              <p>{text({zh: '外部业务只使用公开包。资源、权限、路由与生命周期先进入 manifest，再投射到 API、Admin 与生成物。', en: 'External business code uses only public packages. Resources, permissions, routes, and lifecycle enter a manifest before projecting to API, Admin, and generated artifacts.'})}</p>
            </div>
            <div className="platform-flow" role="img" aria-label={text({zh: '业务能力通过公开合同进入 API、Admin 与运行时适配器的流程', en: 'Flow from a business capability through public contracts to API, Admin, and runtime adapters'})}>
              <div><small>01</small><strong>{text({zh: '业务 capability', en: 'Business capability'})}</strong><em>pkg/platform/*</em></div>
              <i aria-hidden="true">→</i>
              <div><small>02</small><strong>{text({zh: 'Manifest 合同', en: 'Manifest contract'})}</strong><em>{text({zh: '资源 · 权限 · 路由', en: 'resources · permissions · routes'})}</em></div>
              <i aria-hidden="true">→</i>
              <div><small>03</small><strong>{text({zh: '可验证表面', en: 'Verifiable surface'})}</strong><em>Gin · Admin · OpenAPI</em></div>
              <i aria-hidden="true">→</i>
              <div><small>04</small><strong>{text({zh: '运行时端口', en: 'Runtime ports'})}</strong><em>GORM · Cache · Storage</em></div>
            </div>
            <Link className="platform-arrow-link" to="/docs/business-integration">{text({zh: '按教程接入第一个能力', en: 'Follow the first-capability tutorial'})}<span aria-hidden="true">→</span></Link>
          </div>
        </section>

        <section className="platform-section platform-section--matrix">
          <div className="container">
            <div className="platform-heading platform-heading--split">
              <div><p className="platform-eyebrow">SUPPORT, NOT SLOGANS</p><h2>{text({zh: '清楚说明默认、可选与受控。', en: 'Clear about default, optional, and controlled.'})}</h2></div>
              <p>{text({zh: '67 个任务节点全部实现，不代表每一项都在默认生产路径启用。支持矩阵把合同完成和外部推广证据分开。', en: 'All 67 task nodes are implemented. That does not mean every path is enabled by default in production. The support matrix separates contract completion from external promotion evidence.'})}</p>
            </div>
            <div className="platform-matrix">
              {supportRows.map((row) => <Link to={row.to} key={row.label.en}><span>{text(row.label)}</span><b>{text(row.state)}</b><p>{text(row.note)}</p><i aria-hidden="true">→</i></Link>)}
            </div>
          </div>
        </section>

        <section className="platform-closing">
          <div className="container platform-closing__inner">
            <div><p className="platform-eyebrow">DOCUMENTATION BY TASK</p><h2>{text({zh: '从一份清晰的合同开始交付。', en: 'Start delivery with a clear contract.'})}</h2></div>
            <div className="platform-closing__links">
              <Link to="/docs/authentication">{text({zh: '认证与授权', en: 'Authentication'})}<span>→</span></Link>
              <Link to="/docs/production-checklist">{text({zh: '生产检查', en: 'Production checklist'})}<span>→</span></Link>
              <a href="https://github.com/GnosiST/platform-go">GitHub<span>↗</span></a>
            </div>
          </div>
        </section>
      </main>
    </Layout>
  );
}
