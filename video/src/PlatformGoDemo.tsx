import React from 'react';
import { AbsoluteFill, Easing, interpolate, Sequence, useCurrentFrame } from 'remotion';

const colors = {
  ink: '#111827',
  muted: '#667085',
  line: '#d8e0ea',
  page: '#f4f7fb',
  surface: '#ffffff',
  blue: '#315df5',
  green: '#0c9f7a',
  amber: '#b7791f',
  red: '#c2410c',
  navy: '#172033',
};

const font = 'Inter, Arial, "PingFang SC", "Microsoft YaHei", sans-serif';
const mono = 'Menlo, Monaco, Consolas, monospace';
const ease = Easing.bezier(0.16, 1, 0.3, 1);

const fade = (frame: number, start: number, duration = 22) =>
  interpolate(frame, [start, start + duration], [0, 1], {
    extrapolateLeft: 'clamp',
    extrapolateRight: 'clamp',
    easing: ease,
  });

const rise = (frame: number, start: number, distance = 34) =>
  interpolate(frame, [start, start + 24], [distance, 0], {
    extrapolateLeft: 'clamp',
    extrapolateRight: 'clamp',
    easing: ease,
  });

const FadeIn: React.FC<{ children: React.ReactNode; start?: number; distance?: number; style?: React.CSSProperties }> = ({
  children,
  start = 0,
  distance = 34,
  style,
}) => {
  const frame = useCurrentFrame();
  return (
    <div
      style={{
        opacity: fade(frame, start),
        translate: `0px ${rise(frame, start, distance)}px`,
        ...style,
      }}
    >
      {children}
    </div>
  );
};

const Scene: React.FC<{ children: React.ReactNode; dark?: boolean }> = ({ children, dark = false }) => (
  <AbsoluteFill
    style={{
      background: dark ? colors.navy : colors.page,
      color: dark ? '#f8fbff' : colors.ink,
      fontFamily: font,
      padding: '82px 104px',
      overflow: 'hidden',
    }}
  >
    <div
      style={{
        position: 'absolute',
        inset: 0,
        background:
          'radial-gradient(circle at 12% 12%, rgba(49,93,245,0.13), transparent 26%), radial-gradient(circle at 88% 82%, rgba(12,159,122,0.12), transparent 28%)',
      }}
    />
    <div style={{ position: 'relative', height: '100%' }}>{children}</div>
  </AbsoluteFill>
);

const Kicker: React.FC<{ children: React.ReactNode; dark?: boolean }> = ({ children, dark = false }) => (
  <div
    style={{
      color: dark ? '#86efcb' : colors.green,
      fontFamily: mono,
      fontSize: 30,
      fontWeight: 700,
      letterSpacing: 0,
      textTransform: 'uppercase',
    }}
  >
    {children}
  </div>
);

const Title: React.FC<{ children: React.ReactNode; dark?: boolean; maxWidth?: number }> = ({ children, dark = false, maxWidth = 1180 }) => (
  <h1
    style={{
      color: dark ? '#ffffff' : colors.ink,
      fontSize: 86,
      lineHeight: 1.04,
      margin: '22px 0 24px',
      maxWidth,
      letterSpacing: 0,
    }}
  >
    {children}
  </h1>
);

const Body: React.FC<{ children: React.ReactNode; dark?: boolean; width?: number }> = ({ children, dark = false, width = 980 }) => (
  <p
    style={{
      color: dark ? '#cbd5e1' : colors.muted,
      fontSize: 36,
      lineHeight: 1.38,
      margin: 0,
      maxWidth: width,
    }}
  >
    {children}
  </p>
);

const BrowserFrame: React.FC<{ title: string; route: string; children: React.ReactNode; scale?: number }> = ({
  title,
  route,
  children,
  scale = 1,
}) => (
  <div
    style={{
      width: 1180,
      height: 704,
      borderRadius: 24,
      border: `1px solid ${colors.line}`,
      background: colors.surface,
      boxShadow: '0 26px 90px rgba(20,32,54,0.16)',
      overflow: 'hidden',
      scale,
      transformOrigin: 'center',
    }}
  >
    <div
      style={{
        height: 64,
        display: 'grid',
        gridTemplateColumns: '120px 1fr 220px',
        alignItems: 'center',
        padding: '0 22px',
        background: '#edf2f8',
        borderBottom: `1px solid ${colors.line}`,
        gap: 18,
      }}
    >
      <div style={{ display: 'flex', gap: 10 }}>
        {['#f87171', '#fbbf24', '#34d399'].map((color) => (
          <div key={color} style={{ width: 16, height: 16, borderRadius: 999, background: color }} />
        ))}
      </div>
      <div
        style={{
          height: 36,
          borderRadius: 999,
          background: '#ffffff',
          border: `1px solid ${colors.line}`,
          display: 'flex',
          alignItems: 'center',
          padding: '0 20px',
          color: colors.muted,
          fontFamily: mono,
          fontSize: 18,
        }}
      >
        {route}
      </div>
      <div style={{ color: colors.muted, textAlign: 'right', fontSize: 18 }}>{title}</div>
    </div>
    <div style={{ height: 640 }}>{children}</div>
  </div>
);

const navItems = ['概览', '用户', '组织机构', '角色', '权限', '菜单', '字典参数', '演示数据', '审计日志'];

const AdminShell: React.FC<{ active: string; children: React.ReactNode; toolbar?: React.ReactNode }> = ({ active, children, toolbar }) => (
  <div style={{ height: '100%', display: 'grid', gridTemplateColumns: '250px 1fr', background: '#f8fafc' }}>
    <aside style={{ background: '#111827', color: '#e5e7eb', padding: '28px 20px' }}>
      <div style={{ fontSize: 24, fontWeight: 800, marginBottom: 28 }}>platform-go</div>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
        {navItems.map((item) => (
          <div
            key={item}
            style={{
              padding: '12px 14px',
              borderRadius: 10,
              background: item === active ? colors.blue : 'transparent',
              color: item === active ? '#ffffff' : '#cbd5e1',
              fontSize: 19,
              fontWeight: item === active ? 800 : 500,
            }}
          >
            {item}
          </div>
        ))}
      </div>
    </aside>
    <main style={{ padding: 28, display: 'grid', gridTemplateRows: '72px 1fr', gap: 20 }}>
      <header style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
        <div>
          <div style={{ color: colors.muted, fontSize: 18 }}>Admin Console</div>
          <div style={{ color: colors.ink, fontSize: 34, fontWeight: 850 }}>{active}</div>
        </div>
        {toolbar}
      </header>
      {children}
    </main>
  </div>
);

const Pill: React.FC<{ children: React.ReactNode; tone?: 'blue' | 'green' | 'amber' | 'red' | 'gray' }> = ({ children, tone = 'gray' }) => {
  const map = {
    blue: ['#eef4ff', colors.blue],
    green: ['#ecfdf5', colors.green],
    amber: ['#fffbeb', colors.amber],
    red: ['#fff7ed', colors.red],
    gray: ['#f2f4f7', colors.muted],
  } as const;
  const [background, color] = map[tone];
  return (
    <span
      style={{
        display: 'inline-flex',
        alignItems: 'center',
        padding: '7px 12px',
        borderRadius: 999,
        background,
        color,
        fontSize: 17,
        fontWeight: 800,
        whiteSpace: 'nowrap',
      }}
    >
      {children}
    </span>
  );
};

const Table: React.FC<{ rows: Array<Array<React.ReactNode>>; headers: string[] }> = ({ rows, headers }) => (
  <div style={{ border: `1px solid ${colors.line}`, borderRadius: 16, overflow: 'hidden', background: '#fff' }}>
    <div
      style={{
        display: 'grid',
        gridTemplateColumns: `repeat(${headers.length}, 1fr)`,
        background: '#f1f5f9',
        color: colors.muted,
        fontSize: 16,
        fontWeight: 850,
      }}
    >
      {headers.map((header) => (
        <div key={header} style={{ padding: '14px 18px' }}>
          {header}
        </div>
      ))}
    </div>
    {rows.map((row, index) => (
      <div
        key={index}
        style={{
          display: 'grid',
          gridTemplateColumns: `repeat(${headers.length}, 1fr)`,
          borderTop: `1px solid ${colors.line}`,
          fontSize: 18,
          color: colors.ink,
        }}
      >
        {row.map((cell, cellIndex) => (
          <div key={cellIndex} style={{ padding: '15px 18px', display: 'flex', alignItems: 'center', minHeight: 56 }}>
            {cell}
          </div>
        ))}
      </div>
    ))}
  </div>
);

const Callout: React.FC<{ title: string; body: string; tone?: 'blue' | 'green' | 'amber' | 'red' }> = ({ title, body, tone = 'blue' }) => {
  const color = { blue: colors.blue, green: colors.green, amber: colors.amber, red: colors.red }[tone];
  return (
    <div style={{ padding: 22, borderRadius: 18, background: '#ffffff', border: `2px solid ${color}`, boxShadow: '0 14px 36px rgba(20,32,54,0.08)' }}>
      <div style={{ color, fontFamily: mono, fontSize: 18, fontWeight: 850, marginBottom: 10 }}>{title}</div>
      <div style={{ color: colors.ink, fontSize: 24, lineHeight: 1.35, fontWeight: 750 }}>{body}</div>
    </div>
  );
};

const OpeningScene: React.FC = () => {
  const frame = useCurrentFrame();
  const steps = ['登录与会话', '后台资源', '角色权限', '菜单分配', '生产门禁'];
  return (
    <Scene dark>
      <FadeIn>
        <Kicker dark>platform-go system demo</Kicker>
        <Title dark maxWidth={1120}>不是概念片，这次演示系统如何跑起来。</Title>
        <Body dark width={1040}>从 Admin 登录进入真实工作台，再走资源写入、角色授权、菜单分配和生产运行边界。</Body>
      </FadeIn>
      <div style={{ position: 'absolute', right: 40, top: 72 }}>
        <BrowserFrame title="demo path" route="http://127.0.0.1:9202" scale={0.62}>
          <div style={{ height: '100%', padding: 44, background: '#f8fafc' }}>
            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 26, height: '100%' }}>
              <div style={{ display: 'flex', flexDirection: 'column', justifyContent: 'center', gap: 18 }}>
                {steps.map((step, index) => (
                  <div
                    key={step}
                    style={{
                      opacity: fade(frame, 18 + index * 12),
                      translate: `${rise(frame, 18 + index * 12, 26)}px 0px`,
                      padding: '20px 24px',
                      borderRadius: 16,
                      background: index === 2 ? '#eef4ff' : '#fff',
                      border: `1px solid ${index === 2 ? colors.blue : colors.line}`,
                      fontSize: 26,
                      fontWeight: 850,
                    }}
                  >
                    <span style={{ color: colors.blue, fontFamily: mono, marginRight: 18 }}>{String(index + 1).padStart(2, '0')}</span>
                    {step}
                  </div>
                ))}
              </div>
              <div style={{ borderRadius: 22, background: colors.navy, color: '#fff', padding: 34, display: 'flex', flexDirection: 'column', justifyContent: 'space-between' }}>
                <div style={{ fontSize: 28, fontWeight: 850 }}>演示覆盖</div>
                <div style={{ display: 'grid', gap: 12, fontSize: 22, color: '#dbeafe' }}>
                  <div>JWT 会话</div>
                  <div>Refine Admin</div>
                  <div>GORM 持久化</div>
                  <div>Casbin RBAC</div>
                  <div>role_menu 写入门禁</div>
                </div>
                <div style={{ fontFamily: mono, color: '#86efcb', fontSize: 18 }}>runtime verified</div>
              </div>
            </div>
          </div>
        </BrowserFrame>
      </div>
    </Scene>
  );
};

const LoginScene: React.FC = () => {
  const frame = useCurrentFrame();
  return (
    <Scene>
      <FadeIn>
        <Kicker>01 / 登录与会话</Kicker>
        <Title maxWidth={940}>Admin 入口先确认身份，再进入受保护控制台。</Title>
      </FadeIn>
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 460px', gap: 44, marginTop: 36, alignItems: 'center' }}>
        <FadeIn start={14}>
          <BrowserFrame title="Admin Login" route="/login">
            <div style={{ height: '100%', display: 'grid', gridTemplateColumns: '1fr 420px', background: '#f8fafc' }}>
              <div style={{ padding: 54 }}>
                <div style={{ color: colors.blue, fontFamily: mono, fontSize: 20, fontWeight: 900 }}>AUTH PROVIDERS</div>
                <div style={{ marginTop: 34, display: 'grid', gap: 18 }}>
                  {[
                    ['演示登录', 'admin / ops', 'blue'],
                    ['Admin OIDC', 'authorization code + PKCE', 'green'],
                    ['App 登录', '独立 app tokenType', 'gray'],
                  ].map(([title, body, tone], index) => (
                    <div
                      key={title}
                      style={{
                        opacity: fade(frame, 28 + index * 10),
                        padding: 24,
                        borderRadius: 18,
                        background: '#fff',
                        border: `1px solid ${index === 0 ? colors.blue : colors.line}`,
                      }}
                    >
                      <div style={{ fontSize: 28, fontWeight: 850 }}>{title}</div>
                      <div style={{ marginTop: 8 }}>
                        <Pill tone={tone as 'blue' | 'green' | 'gray'}>{body}</Pill>
                      </div>
                    </div>
                  ))}
                </div>
              </div>
              <div style={{ background: '#ffffff', borderLeft: `1px solid ${colors.line}`, padding: 48 }}>
                <div style={{ fontSize: 34, fontWeight: 900, marginBottom: 24 }}>登录</div>
                <div style={{ display: 'grid', gap: 18 }}>
                  <div style={{ height: 54, borderRadius: 12, border: `1px solid ${colors.line}`, padding: '14px 16px', fontSize: 20, color: colors.muted }}>admin</div>
                  <div style={{ height: 54, borderRadius: 12, border: `1px solid ${colors.line}`, padding: '14px 16px', fontSize: 20, color: colors.muted }}>demo password</div>
                  <div style={{ height: 58, borderRadius: 14, background: colors.blue, color: '#fff', display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: 22, fontWeight: 900 }}>进入控制台</div>
                </div>
              </div>
            </div>
          </BrowserFrame>
        </FadeIn>
        <FadeIn start={36} style={{ display: 'grid', gap: 18 }}>
          <Callout title="SERVER SESSION" body="JWT 只负责 HTTP 凭证；TTL、刷新和注销由服务端 session 决定。" tone="green" />
          <Callout title="PRODUCTION RULE" body="生产环境必须关闭 demo provider，并使用独立密钥和持久化会话库。" tone="amber" />
        </FadeIn>
      </div>
    </Scene>
  );
};

const ResourceScene: React.FC = () => {
  const frame = useCurrentFrame();
  return (
    <Scene>
      <FadeIn>
        <Kicker>02 / 管理资源写入</Kicker>
        <Title maxWidth={1030}>不是静态页面：资源 schema 驱动列表、查询和表单。</Title>
      </FadeIn>
      <FadeIn start={12} style={{ marginTop: 22 }}>
        <BrowserFrame title="Generic Resource Console" route="/dictionary-parameters">
          <AdminShell
            active="字典参数"
            toolbar={
              <div style={{ display: 'flex', gap: 10 }}>
                <Pill tone="blue">结构化查询</Pill>
                <Pill tone="green">新增记录</Pill>
              </div>
            }
          >
            <div style={{ display: 'grid', gridTemplateRows: '70px 1fr', gap: 18 }}>
              <div style={{ display: 'flex', gap: 12, alignItems: 'center' }}>
                <div style={{ width: 360, height: 48, borderRadius: 12, border: `1px solid ${colors.line}`, background: '#fff', color: colors.muted, padding: '12px 16px', fontSize: 18 }}>keywords: status enabled</div>
                <Pill tone="blue">POST /query</Pill>
                <Pill tone="green">permission checked</Pill>
              </div>
              <div style={{ position: 'relative' }}>
                <Table
                  headers={['编码', '名称', '状态', '更新时间']}
                  rows={[
                    ['order-status.pending', '待处理', <Pill tone="amber">enabled</Pill>, '2026-07-17'],
                    ['order-status.done', '已完成', <Pill tone="green">enabled</Pill>, '2026-07-17'],
                    ['tenant-tier.basic', '基础租户', <Pill tone="blue">enabled</Pill>, '2026-07-17'],
                    ['feature.flag.demo', '演示开关', <Pill tone="gray">draft</Pill>, '2026-07-17'],
                  ]}
                />
                <div
                  style={{
                    opacity: fade(frame, 76),
                    position: 'absolute',
                    right: 26,
                    bottom: 28,
                    width: 340,
                    padding: 22,
                    borderRadius: 18,
                    background: '#fff',
                    border: `2px solid ${colors.blue}`,
                    boxShadow: '0 18px 50px rgba(20,32,54,0.18)',
                  }}
                >
                  <div style={{ fontSize: 24, fontWeight: 900, marginBottom: 14 }}>新增字典参数</div>
                  <div style={{ display: 'grid', gap: 10 }}>
                    {['code', 'name', 'status'].map((item) => (
                      <div key={item} style={{ height: 36, borderRadius: 9, background: '#f1f5f9', color: colors.muted, padding: '8px 12px', fontSize: 15 }}>{item}</div>
                    ))}
                    <div style={{ marginTop: 6, height: 40, borderRadius: 10, background: colors.blue, color: '#fff', display: 'flex', alignItems: 'center', justifyContent: 'center', fontWeight: 900 }}>保存</div>
                  </div>
                </div>
              </div>
            </div>
          </AdminShell>
        </BrowserFrame>
      </FadeIn>
    </Scene>
  );
};

const RbacScene: React.FC = () => {
  const frame = useCurrentFrame();
  return (
    <Scene>
      <FadeIn>
        <Kicker>03 / 角色权限与菜单</Kicker>
        <Title maxWidth={1160}>角色不只看权限，也能进入菜单分配流程。</Title>
      </FadeIn>
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 430px', gap: 44, marginTop: 24, alignItems: 'center' }}>
        <FadeIn start={10}>
          <BrowserFrame title="Role Governance" route="/roles">
            <AdminShell active="角色" toolbar={<Pill tone="green">target mode</Pill>}>
              <div style={{ display: 'grid', gridTemplateColumns: '310px 1fr', gap: 20 }}>
                <div style={{ borderRadius: 16, border: `1px solid ${colors.line}`, background: '#fff', padding: 18 }}>
                  <div style={{ fontWeight: 900, fontSize: 22, marginBottom: 14 }}>角色树</div>
                  {['平台管理员', '运维人员', '审计员', '业务观察员'].map((item, index) => (
                    <div
                      key={item}
                      style={{
                        padding: '13px 12px',
                        marginBottom: 8,
                        borderRadius: 10,
                        background: index === 1 ? '#eef4ff' : '#f8fafc',
                        border: `1px solid ${index === 1 ? colors.blue : colors.line}`,
                        fontSize: 18,
                        fontWeight: index === 1 ? 850 : 600,
                      }}
                    >
                      {item}
                    </div>
                  ))}
                </div>
                <div style={{ display: 'grid', gridTemplateRows: '1fr 1fr', gap: 18 }}>
                  <div style={{ borderRadius: 16, border: `1px solid ${colors.line}`, background: '#fff', padding: 18 }}>
                    <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 12 }}>
                      <div style={{ fontWeight: 900, fontSize: 22 }}>授权 Tree Transfer</div>
                      <Pill tone="blue">allow / deny / data scope</Pill>
                    </div>
                    <Table
                      headers={['权限', '类型', '结果']}
                      rows={[
                        ['admin:tenant:read', 'API', <Pill tone="green">允许</Pill>],
                        ['admin:role:update', 'API', <Pill tone="amber">需复核</Pill>],
                        ['page:menu.configure', '按钮', <Pill tone="blue">页面按钮</Pill>],
                      ]}
                    />
                  </div>
                  <div style={{ borderRadius: 16, border: `2px solid ${colors.green}`, background: '#fff', padding: 18, opacity: fade(frame, 66) }}>
                    <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 12 }}>
                      <div style={{ fontWeight: 900, fontSize: 22 }}>分配菜单</div>
                      <Pill tone="green">role_menu target-write</Pill>
                    </div>
                    <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 1fr', gap: 12 }}>
                      {['概览', '租户', '角色', '菜单', '审计', '演示数据'].map((item, index) => (
                        <div key={item} style={{ padding: 14, borderRadius: 12, background: index < 4 ? '#ecfdf5' : '#f1f5f9', color: index < 4 ? colors.green : colors.muted, fontSize: 18, fontWeight: 850 }}>
                          {item}
                        </div>
                      ))}
                    </div>
                  </div>
                </div>
              </div>
            </AdminShell>
          </BrowserFrame>
        </FadeIn>
        <FadeIn start={42} style={{ display: 'grid', gap: 18 }}>
          <Callout title="FIXED FLOW" body="菜单入口从只读查看变成可写分配；无变更保存也能正常关闭。" tone="green" />
          <Callout title="SECURITY BOUNDARY" body="菜单只是可见性提示，后端资源动作仍由 Casbin 权限校验。" tone="blue" />
        </FadeIn>
      </div>
    </Scene>
  );
};

const DemoDataScene: React.FC = () => (
  <Scene>
    <FadeIn>
      <Kicker>04 / 演示数据与审计</Kicker>
      <Title maxWidth={1100}>演示数据通过 capability 声明，应用后写入资源并留下审计线索。</Title>
    </FadeIn>
    <FadeIn start={16} style={{ marginTop: 28 }}>
      <BrowserFrame title="Demo Data Apply" route="/demo-data">
        <AdminShell active="演示数据" toolbar={<Pill tone="amber">development only</Pill>}>
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 22 }}>
            <div style={{ borderRadius: 18, border: `1px solid ${colors.line}`, background: '#fff', padding: 24 }}>
              <div style={{ color: colors.blue, fontFamily: mono, fontSize: 18, fontWeight: 900 }}>DATASET</div>
              <div style={{ fontSize: 32, fontWeight: 900, margin: '12px 0' }}>platform-demo-tenants</div>
              <div style={{ color: colors.muted, fontSize: 22, lineHeight: 1.35 }}>本地演示和底座验证使用的租户数据，不进入生产 profile。</div>
              <div style={{ marginTop: 28, display: 'flex', gap: 12 }}>
                <Pill tone="green">target: tenants</Pill>
                <Pill tone="blue">duplicate-safe</Pill>
              </div>
              <div style={{ marginTop: 34, height: 58, borderRadius: 14, background: colors.green, color: '#fff', display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: 22, fontWeight: 900 }}>
                应用数据集
              </div>
            </div>
            <div style={{ display: 'grid', gap: 18 }}>
              <Callout title="WRITE RESULT" body="租户、字典、参数等资源通过统一 Store 写入，重复 apply 不产生重复记录。" tone="green" />
              <Callout title="AUDIT RESULT" body="generic admin resource writes 记录 actor、resource、target 与 action。" tone="blue" />
              <Callout title="PRODUCTION RULE" body="生产 runtime 拒绝 demo-data capability，并要求关闭 demo auth。" tone="red" />
            </div>
          </div>
        </AdminShell>
      </BrowserFrame>
    </FadeIn>
  </Scene>
);

const RuntimeScene: React.FC = () => {
  const frame = useCurrentFrame();
  const cards = [
    ['GORM stores', 'admin resources / sessions / lifecycle history', 'green'],
    ['Redis cache', 'peer invalidation, source of truth stays DB', 'blue'],
    ['Production preflight', 'env, secrets, demo-off, approval evidence', 'amber'],
    ['Menu rollout', 'E2E complete; production writes still approval-gated', 'red'],
  ] as const;
  return (
    <Scene dark>
      <FadeIn>
        <Kicker dark>05 / 运行与生产门禁</Kicker>
        <Title dark maxWidth={1190}>系统可以演示，但生产切换仍然要走门禁。</Title>
        <Body dark width={1080}>这条底座支持真实运行路径，同时把危险操作留在显式审批、预检和回滚证据之后。</Body>
      </FadeIn>
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 24, marginTop: 54 }}>
        {cards.map(([title, body, tone], index) => (
          <div
            key={title}
            style={{
              opacity: fade(frame, 24 + index * 10),
              translate: `0px ${rise(frame, 24 + index * 10, 24)}px`,
              minHeight: 134,
              borderRadius: 22,
              background: 'rgba(255,255,255,0.08)',
              border: '1px solid rgba(255,255,255,0.18)',
              padding: 28,
            }}
          >
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: 18 }}>
              <div style={{ color: '#fff', fontSize: 34, fontWeight: 900 }}>{title}</div>
              <Pill tone={tone as 'green' | 'blue' | 'amber' | 'red'}>verified</Pill>
            </div>
            <div style={{ marginTop: 16, color: '#cbd5e1', fontSize: 24, lineHeight: 1.35 }}>{body}</div>
          </div>
        ))}
      </div>
      <FadeIn start={76}>
        <div style={{ position: 'absolute', left: 0, right: 0, bottom: 0, padding: 30, borderRadius: 24, background: '#0f172a', border: '1px solid rgba(255,255,255,0.18)', display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <div style={{ color: '#fff', fontSize: 30, fontWeight: 900 }}>production command surface</div>
          <div style={{ color: '#86efcb', fontFamily: mono, fontSize: 22 }}>validate → preflight → promote → rollback evidence</div>
        </div>
      </FadeIn>
    </Scene>
  );
};

const CloseScene: React.FC = () => (
  <Scene>
    <div style={{ height: '100%', display: 'flex', flexDirection: 'column', justifyContent: 'center', alignItems: 'center', textAlign: 'center' }}>
      <FadeIn>
        <Kicker>system demo complete</Kicker>
        <Title maxWidth={1260}>一条完整系统演示线：登录、资源、权限、菜单、运行门禁。</Title>
        <Body width={1080}>platform-go 是业务中立底座；业务能力接入前，先把身份、授权、资源合同和生产治理跑通。</Body>
      </FadeIn>
      <FadeIn start={34}>
        <div style={{ display: 'flex', gap: 14, marginTop: 42 }}>
          {['Gin', 'GORM', 'Casbin', 'JWT', 'Refine', 'React', 'Ant Design'].map((item) => (
            <Pill key={item} tone={item === 'GORM' || item === 'Casbin' ? 'green' : 'blue'}>
              {item}
            </Pill>
          ))}
        </div>
      </FadeIn>
    </div>
  </Scene>
);

export const PlatformGoDemo: React.FC = () => (
  <AbsoluteFill style={{ fontFamily: font }}>
    <Sequence from={0} durationInFrames={150}>
      <OpeningScene />
    </Sequence>
    <Sequence from={150} durationInFrames={240}>
      <LoginScene />
    </Sequence>
    <Sequence from={390} durationInFrames={280}>
      <ResourceScene />
    </Sequence>
    <Sequence from={670} durationInFrames={300}>
      <RbacScene />
    </Sequence>
    <Sequence from={970} durationInFrames={240}>
      <DemoDataScene />
    </Sequence>
    <Sequence from={1210} durationInFrames={250}>
      <RuntimeScene />
    </Sequence>
    <Sequence from={1460} durationInFrames={160}>
      <CloseScene />
    </Sequence>
  </AbsoluteFill>
);
