import {
  ApiOutlined,
  AppstoreOutlined,
  CodeOutlined,
  FileTextOutlined,
  LinkOutlined,
  SafetyCertificateOutlined,
  ShoppingOutlined,
  TeamOutlined,
  UserOutlined,
} from "@ant-design/icons";
import { Button, Table, Typography } from "antd";
import { useMemo, useState, type CSSProperties } from "react";
import type { AdminCurrentSession, CapabilityItem, LocalizedText } from "../api/client";
import { dictionaries, type Dictionary, type Language } from "../i18n";
import { projectRoleManagementNavigation } from "../resources/roleManagementNavigation";
import type { AdminResourceDefinition } from "../resources/registry";
import { PlatformPaginationBar } from "../ui";
import { dashboardAnnouncements, dashboardPlugins, dashboardUpdates } from "./dashboardData";

type DashboardHomeProps = {
  language: Language;
  dictionary: Dictionary;
  session: AdminCurrentSession;
  capabilities: CapabilityItem[];
  resources: AdminResourceDefinition[];
  permissions: string[];
  onRouteChange: (route: string) => void;
};

const chartPoints = [12, 21, 30, 43, 36, 78, 88, 92];
const dashboardPluginPageSize = 3;

export function DashboardHome({
  language,
  dictionary,
  session,
  capabilities,
  resources,
  permissions,
  onRouteChange,
}: DashboardHomeProps) {
  const displayName = session.user.name || session.user.username || dictionary.admin;
  const enabledCapabilityCount = capabilities.length;
  const permissionCount = permissions.includes("*") ? resources.reduce((total, resource) => total + (resource.permission ? 1 : 0), 0) : permissions.length;
  const [pluginPage, setPluginPage] = useState(1);
  const pluginRows = useMemo(() => {
    const start = (pluginPage - 1) * dashboardPluginPageSize;
    return dashboardPlugins.slice(start, start + dashboardPluginPageSize);
  }, [pluginPage]);
  const roleManagementResource = projectRoleManagementNavigation(resources, {
    zh: dictionaries.zh.roleManagement,
    en: dictionaries.en.roleManagement,
  }).find((resource) => resource.route === "/roles" || resource.route === "/role-groups");
  const quickActions = [
    { key: "menus", label: dictionary.menus, route: "/menus", icon: AppstoreOutlined },
    { key: "api", label: dictionary.apiResources, route: "/api-resources", icon: ApiOutlined },
    ...(roleManagementResource
      ? [{ key: "role-management", label: dictionary.roleManagement, route: roleManagementResource.route, icon: TeamOutlined }]
      : []),
    { key: "users", label: dictionary.users, route: "/users", icon: UserOutlined },
    { key: "capabilities", label: dictionary.capabilities, route: "/capabilities", icon: CodeOutlined },
    { key: "demo-data", label: dictionary.demoData, route: "/demo-data", icon: SafetyCertificateOutlined },
  ].filter((item) => item.route === "/demo-data" || resources.some((resource) => resource.route === item.route));

  return (
    <section className="dashboard-home">
      <div className="dashboard-hero-panel">
        <div>
          <Typography.Text className="page-eyebrow">{dictionary.dashboardEyebrow}</Typography.Text>
          <Typography.Title level={1}>{`${dictionary.dashboardWelcome} · ${displayName}`}</Typography.Title>
          <Typography.Paragraph>{`${formatDashboardDate(language)} · ${dictionary.dashboardSubtitle}`}</Typography.Paragraph>
        </div>
        <div className="dashboard-hero-actions">
          <Button icon={<ShoppingOutlined />} type="primary">
            {dictionary.dashboardBuyLicense}
          </Button>
          <Button icon={<AppstoreOutlined />}>{dictionary.dashboardMarketplace}</Button>
        </div>
      </div>

      <div className="dashboard-metric-grid">
        <DashboardMetric label={dictionary.dashboardAuthorizedResources} points={[18, 24, 31, 40, 46, 58, 66, 72]} value={String(resources.length)} extra={session.roles.join(" / ") || dictionary.admin} />
        <DashboardMetric label={dictionary.dashboardEnabledCapabilities} points={[22, 28, 34, 48, 58, 66, 72, 80]} value={String(enabledCapabilityCount)} extra={dictionary.dashboardTrendUp} />
        <DashboardMetric label={dictionary.dashboardPermissionCodes} points={[20, 26, 34, 48, 42, 72, 78, 80]} value={String(permissionCount)} extra={permissions.includes("*") ? dictionary.all : dictionary.dashboardTrendUp} />
      </div>

      <div className="dashboard-layout">
        <div className="dashboard-main-column">
          <section className="dashboard-panel dashboard-chart-panel">
            <PanelTitle title={dictionary.dashboardContentData} />
            <DashboardTrendChart label={dictionary.dashboardContentData} language={language} points={chartPoints} />
          </section>

          <section className="dashboard-panel">
            <PanelTitle title={dictionary.dashboardLatestPlugins} />
            <Table
              className="dashboard-table platform-data-table"
              dataSource={pluginRows}
              pagination={false}
              rowKey="id"
              size="small"
              columns={[
                {
                  title: dictionary.dashboardLatestPlugins,
                  dataIndex: "title",
                  render: (value: LocalizedText) => <Typography.Text strong>{localizedText(value, language)}</Typography.Text>,
                },
                {
                  title: dictionary.description,
                  dataIndex: "description",
                  ellipsis: true,
                  render: (value: LocalizedText) => localizedText(value, language),
                },
                {
                  title: dictionary.price,
                  dataIndex: "price",
                  width: 120,
                },
              ]}
            />
            <PlatformPaginationBar
              config={{
                showQuickJumper: false,
                showSizeChanger: false,
                showTotal: (total, range) =>
                  formatTemplate(dictionary.paginationRange, {
                    start: String(range[0]),
                    end: String(range[1]),
                    total: String(total),
                  }),
              }}
              current={pluginPage}
              disabled={false}
              labels={{
                pageSize: dictionary.pageSize,
                goToPage: dictionary.goToPage,
                page: dictionary.page,
                paginationRange: dictionary.paginationRange,
              }}
              pageSize={dashboardPluginPageSize}
              total={dashboardPlugins.length}
              onChange={(page) => setPluginPage(page)}
            />
          </section>

          <section className="dashboard-panel">
            <PanelTitle title={dictionary.dashboardLatestUpdates} />
            <Table
              className="dashboard-table platform-data-table"
              dataSource={dashboardUpdates}
              pagination={false}
              rowKey="id"
              size="small"
              columns={[
                { title: dictionary.dashboardRank, width: 72, render: (_value, _row, index) => index + 1 },
                {
                  title: dictionary.dashboardUpdateContent,
                  dataIndex: "content",
                  ellipsis: true,
                  render: (value: LocalizedText) => localizedText(value, language),
                },
                { title: dictionary.dashboardSubmitter, dataIndex: "submitter", width: 140 },
                { title: dictionary.dashboardTime, dataIndex: "time", width: 180 },
              ]}
            />
          </section>
        </div>

        <aside className="dashboard-side-column">
          <section className="dashboard-panel">
            <PanelTitle title={dictionary.dashboardQuickActions} action={dictionary.dashboardMore} />
            <Typography.Text className="secondary-text">{dictionary.dashboardCommonEntry}</Typography.Text>
            <div className="quick-action-grid">
              {quickActions.map((item) => {
                const Icon = item.icon;
                return (
                  <button key={item.key} type="button" onClick={() => onRouteChange(item.route)}>
                    <Icon />
                    <span>{item.label}</span>
                  </button>
                );
              })}
            </div>
            <Typography.Text className="secondary-text">{dictionary.dashboardExternalLinks}</Typography.Text>
            <div className="dashboard-link-list">
              <a href="https://gin-gonic.com/docs/" rel="noreferrer" target="_blank">
                <FileTextOutlined />
                <span>GIN</span>
                <em>{dictionary.dashboardOpen}</em>
              </a>
              <a href="https://refine.dev/docs/" rel="noreferrer" target="_blank">
                <LinkOutlined />
                <span>Refine</span>
                <em>{dictionary.dashboardOpen}</em>
              </a>
            </div>
          </section>

          <section className="dashboard-panel">
            <PanelTitle title={dictionary.dashboardAnnouncements} action={dictionary.dashboardMore} />
            <div className="announcement-list">
              {dashboardAnnouncements.map((item) => (
                <article className={`announcement-item ${item.type}`} key={item.id}>
                  <div>
                    <span>{announcementTypeLabel(dictionary, item.type)}</span>
                    <time>{localizedText(item.time, language)}</time>
                  </div>
                  <p>{localizedText(item.title, language)}</p>
                </article>
              ))}
            </div>
          </section>

          <section className="dashboard-panel dashboard-docs-panel">
            <PanelTitle title={dictionary.dashboardDocs} action={dictionary.dashboardMore} />
            <a href="https://react.dev/" rel="noreferrer" target="_blank">React</a>
            <a href="https://gin-gonic.com/docs/" rel="noreferrer" target="_blank">GIN</a>
            <a href="https://github.com/" rel="noreferrer" target="_blank">GitHub</a>
            <a href="https://refine.dev/docs/" rel="noreferrer" target="_blank">Refine</a>
          </section>

          <section className="dashboard-commercial-panel">
            <Typography.Text>{dictionary.dashboardCommercialSupport}</Typography.Text>
            <Typography.Title level={3}>{dictionary.dashboardCommercialTitle}</Typography.Title>
            <Typography.Paragraph>{dictionary.dashboardCommercialDesc}</Typography.Paragraph>
            <div>
              <Button type="primary">{dictionary.dashboardBuyNow}</Button>
              <Button type="link">{dictionary.dashboardViewMarketplace}</Button>
            </div>
          </section>
        </aside>
      </div>
    </section>
  );
}

function DashboardMetric({ label, value, extra, points }: { label: string; value: string; extra: string; points: number[] }) {
  return (
    <section className="dashboard-metric-card">
      <div>
        <Typography.Text>{label}</Typography.Text>
        <strong>{value}</strong>
        <span>{extra}</span>
      </div>
      <div className="dashboard-sparkline" aria-hidden="true">
        {points.map((point, index) => (
          <i key={index} style={{ "--point": `${point}%` } as CSSProperties} />
        ))}
      </div>
    </section>
  );
}

function DashboardTrendChart({ label, language, points }: { label: string; language: Language; points: number[] }) {
  const width = 640;
  const height = 220;
  const padding = 20;
  const chartHeight = height - padding * 2;
  const maxPoint = Math.max(...points, 1);
  const coordinates = points.map((point, index) => {
    const x = padding + (index / Math.max(points.length - 1, 1)) * (width - padding * 2);
    const y = padding + chartHeight - (point / maxPoint) * chartHeight;
    return [x, y] as const;
  });
  const linePath = coordinates.map(([x, y], index) => `${index === 0 ? "M" : "L"} ${x} ${y}`).join(" ");
  const areaPath = `${linePath} L ${width - padding} ${height - padding} L ${padding} ${height - padding} Z`;

  return (
    <div className="dashboard-chart" aria-label={label} role="img">
      <svg aria-hidden="true" focusable="false" viewBox={`0 0 ${width} ${height}`}>
        <path className="dashboard-chart-area" d={areaPath} />
        <path className="dashboard-chart-line" d={linePath} />
        {coordinates.map(([x, y], index) => (
          <circle className="dashboard-chart-point" cx={x} cy={y} key={index} r="4" />
        ))}
      </svg>
      <div className="dashboard-chart-axis">
        {dashboardMonthLabels(language, points.length).map((month) => (
          <span key={month}>{month}</span>
        ))}
      </div>
    </div>
  );
}

function PanelTitle({ title, action }: { title: string; action?: string }) {
  return (
    <div className="dashboard-panel-title">
      <Typography.Text strong>{title}</Typography.Text>
      {action ? <Button type="text">{action}</Button> : null}
    </div>
  );
}

function announcementTypeLabel(dictionary: Dictionary, type: "notice" | "compliant" | "service") {
  const labels = {
    notice: dictionary.dashboardNotice,
    compliant: dictionary.dashboardCompliant,
    service: dictionary.dashboardService,
  };
  return labels[type];
}

function localizedText(value: LocalizedText, language: Language) {
  return value[language] || value.zh || value.en;
}

function formatTemplate(template: string, values: Record<string, string>) {
  return Object.entries(values).reduce((result, [key, value]) => result.replaceAll(`{${key}}`, value), template);
}

function formatDashboardDate(language: Language) {
  return new Intl.DateTimeFormat(language === "zh" ? "zh-CN" : "en-US", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
  }).format(new Date());
}

function dashboardMonthLabels(language: Language, count: number) {
  const formatter = new Intl.DateTimeFormat(language === "zh" ? "zh-CN" : "en-US", { month: "short" });
  const now = new Date();
  return Array.from({ length: count }, (_, index) => {
    const date = new Date(now);
    date.setMonth(now.getMonth() - (count - index - 1));
    return formatter.format(date);
  });
}
