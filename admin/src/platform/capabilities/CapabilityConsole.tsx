import {
  ApiOutlined,
  CloudUploadOutlined,
  CopyOutlined,
  DownloadOutlined,
  EditOutlined,
  EyeOutlined,
  FileSearchOutlined,
  FilterOutlined,
  PlusOutlined,
  ReloadOutlined,
  SearchOutlined,
  SettingOutlined,
  StopOutlined,
} from "@ant-design/icons";
import { Alert, Button, Empty, Input, Progress, Segmented, Space, Spin, Table, Tag, Typography } from "antd";
import { useEffect, useMemo, useState } from "react";
import type { Dictionary, Language } from "../i18n";
import type { CapabilityKind, CapabilityView } from "./metadata";

type CapabilityConsoleProps = {
  capabilities: CapabilityView[];
  optionalCapabilities: CapabilityView[];
  language: Language;
  dictionary: Dictionary;
  loading: boolean;
  error: string;
};

type CapabilityFilter = "all" | CapabilityKind;

export function CapabilityConsole({
  capabilities,
  optionalCapabilities,
  language,
  dictionary,
  loading,
  error,
}: CapabilityConsoleProps) {
  const [query, setQuery] = useState("");
  const [filter, setFilter] = useState<CapabilityFilter>("all");
  const [selectedID, setSelectedID] = useState("");
  const [installedOptionals, setInstalledOptionals] = useState<string[]>([]);

  const installedOptionalViews = useMemo(
    () =>
      optionalCapabilities
        .filter((capability) => installedOptionals.includes(capability.id))
        .map((capability) => ({ ...capability, kind: "plugin" as const })),
    [installedOptionals, optionalCapabilities],
  );

  const allCapabilities = useMemo(
    () => [...capabilities, ...installedOptionalViews],
    [capabilities, installedOptionalViews],
  );

  const filteredCapabilities = useMemo(() => {
    const normalizedQuery = query.trim().toLowerCase();
    return allCapabilities.filter((capability) => {
      const matchesFilter = filter === "all" || capability.kind === filter;
      const haystack = [
        capability.id,
        capability.name,
        capability.label.zh,
        capability.label.en,
        capability.domain.zh,
        capability.domain.en,
        capability.description.zh,
        capability.description.en,
      ]
        .join(" ")
        .toLowerCase();
      return matchesFilter && (!normalizedQuery || haystack.includes(normalizedQuery));
    });
  }, [allCapabilities, filter, query]);

  const selectedCapability = useMemo(() => {
    return (
      allCapabilities.find((capability) => capability.id === selectedID) ??
      filteredCapabilities[0] ??
      allCapabilities[0]
    );
  }, [allCapabilities, filteredCapabilities, selectedID]);

  useEffect(() => {
    if (!selectedID && allCapabilities[0]) {
      setSelectedID(allCapabilities[0].id);
    }
  }, [allCapabilities, selectedID]);

  const enabledCount = allCapabilities.filter((capability) => capability.kind !== "disabled").length;
  const optionalCount = optionalCapabilities.length - installedOptionals.length;
  const disabledCount = allCapabilities.filter((capability) => capability.kind === "disabled").length;
  const domainCount = new Set(allCapabilities.map((capability) => capability.domain.en)).size;
  const healthyCount = allCapabilities.filter((capability) => capability.health === "healthy").length;
  const healthPercent = Math.round((healthyCount / Math.max(allCapabilities.length, 1)) * 100);

  const columns = [
    {
      title: dictionary.capability,
      dataIndex: "label",
      key: "label",
      width: 150,
      render: (_: unknown, record: CapabilityView) => (
        <div className="capability-name-cell">
          <Typography.Text strong>{record.label[language]}</Typography.Text>
          <Typography.Text className="secondary-text">{record.label[language === "zh" ? "en" : "zh"]}</Typography.Text>
        </div>
      ),
    },
    {
      title: dictionary.code,
      dataIndex: "id",
      key: "id",
      width: 130,
      render: (id: string) => <Typography.Text code>{id}</Typography.Text>,
    },
    {
      title: dictionary.domain,
      dataIndex: "domain",
      key: "domain",
      width: 150,
      render: (_: unknown, record: CapabilityView) => <span>{record.domain[language]}</span>,
    },
    {
      title: dictionary.type,
      dataIndex: "kind",
      key: "kind",
      width: 130,
      render: (kind: CapabilityKind) => <Tag className={`kind-tag kind-${kind}`}>{kindLabel(dictionary, kind)}</Tag>,
    },
    {
      title: dictionary.status,
      dataIndex: "kind",
      key: "status",
      width: 110,
      render: (kind: CapabilityKind) => (
        <span className={`status-dot status-${kind === "disabled" ? "disabled" : "enabled"}`}>
          {kind === "disabled" ? dictionary.disabled : dictionary.enabled}
        </span>
      ),
    },
    {
      title: dictionary.version,
      dataIndex: "version",
      key: "version",
      width: 90,
    },
    {
      title: dictionary.actions,
      key: "actions",
      fixed: "right" as const,
      width: 100,
      render: (_: unknown, record: CapabilityView) => (
        <Space size={4}>
          <Button icon={<EyeOutlined />} size="small" type="text" onClick={() => setSelectedID(record.id)} />
          <Button icon={<EditOutlined />} size="small" type="text" />
          <Button icon={<CopyOutlined />} size="small" type="text" />
        </Space>
      ),
    },
  ];

  return (
    <section className="capability-console">
      <div className="page-heading">
        <div>
          <Typography.Title level={1}>{dictionary.pageTitle}</Typography.Title>
          <Typography.Paragraph>{dictionary.pageSubtitle}</Typography.Paragraph>
        </div>
        <Space className="page-actions">
          <Button icon={<DownloadOutlined />}>{dictionary.import}</Button>
          <Button icon={<PlusOutlined />} type="primary">
            {dictionary.addCapability}
          </Button>
        </Space>
      </div>

      {error ? <Alert className="api-alert" type="warning" message={dictionary.apiUnavailable} description={error} showIcon /> : null}

      <div className="summary-band">
        <div className="summary-metrics">
          <Metric label={dictionary.totalCapabilities} value={allCapabilities.length} />
          <Metric label={dictionary.enabled} value={enabledCount} accent />
          <Metric label={dictionary.optional} value={optionalCount} warning />
          <Metric label={dictionary.disabled} value={disabledCount} />
          <Metric label={dictionary.domains} value={domainCount} />
          <Metric label={dictionary.installedPlugins} value={installedOptionals.length} />
        </div>
        <div className="health-panel">
          <div>
            <Typography.Text strong>{dictionary.installHealth}</Typography.Text>
            <div className="health-legend">
              <span className="legend-item healthy">{dictionary.healthy}</span>
              <span className="legend-item warning">{dictionary.warning}</span>
              <span className="legend-item error">{dictionary.error}</span>
            </div>
          </div>
          <Progress percent={healthPercent} type="circle" size={82} strokeColor="var(--success)" />
        </div>
      </div>

      <div className="console-grid">
        <div className="capability-workspace">
          <div className="table-toolbar">
            <Segmented
              value={filter}
              onChange={(value) => setFilter(value as CapabilityFilter)}
              options={[
                { value: "all", label: dictionary.all },
                { value: "core", label: dictionary.core },
                { value: "plugin", label: dictionary.plugin },
                { value: "optional", label: dictionary.optionalTab },
                { value: "disabled", label: dictionary.disabledTab },
              ]}
            />
            <div className="table-actions">
              <Input
                className="capability-search"
                prefix={<SearchOutlined />}
                placeholder={dictionary.searchCapability}
                value={query}
                onChange={(event) => setQuery(event.target.value)}
              />
              <Button icon={<FilterOutlined />}>{dictionary.filter}</Button>
              <Button icon={<ReloadOutlined />} />
            </div>
          </div>

          {loading ? (
            <div className="loading-panel">
              <Spin />
            </div>
          ) : filteredCapabilities.length === 0 ? (
            <Empty />
          ) : (
            <>
              <Table
                className="capability-table"
                columns={columns}
                dataSource={filteredCapabilities}
                pagination={false}
                rowKey="id"
                rowClassName={(record) => (record.id === selectedCapability?.id ? "selected-row" : "")}
                scroll={{ x: 860 }}
                onRow={(record) => ({
                  onClick: () => setSelectedID(record.id),
                })}
              />
              <div className="capability-mobile-list">
                {filteredCapabilities.map((capability) => (
                  <button
                    className={capability.id === selectedCapability?.id ? "mobile-capability-card active" : "mobile-capability-card"}
                    key={capability.id}
                    type="button"
                    onClick={() => setSelectedID(capability.id)}
                  >
                    <span>
                      <strong>{capability.label[language]}</strong>
                      <em>{capability.id}</em>
                    </span>
                    <Tag className={`kind-tag kind-${capability.kind}`}>{kindLabel(dictionary, capability.kind)}</Tag>
                  </button>
                ))}
              </div>
            </>
          )}

          <section className="optional-section">
            <div className="section-title-row">
              <Typography.Title level={3}>{dictionary.optionalCapabilities}</Typography.Title>
              <Typography.Text>{dictionary.marketplaceHint}</Typography.Text>
            </div>
            <div className="optional-grid">
              {optionalCapabilities.map((capability) => {
                const installed = installedOptionals.includes(capability.id);
                return (
                  <article className="optional-card" key={capability.id}>
                    <CloudUploadOutlined />
                    <div>
                      <Typography.Text strong>{capability.label[language]}</Typography.Text>
                      <Typography.Text className="secondary-text">{capability.version}</Typography.Text>
                    </div>
                    <Button
                      size="small"
                      type={installed ? "default" : "primary"}
                      onClick={() =>
                        setInstalledOptionals((current) =>
                          current.includes(capability.id)
                            ? current.filter((id) => id !== capability.id)
                            : [...current, capability.id],
                        )
                      }
                    >
                      {installed ? dictionary.enabled : dictionary.install}
                    </Button>
                  </article>
                );
              })}
            </div>
          </section>
        </div>

        <CapabilityInspector capability={selectedCapability} dictionary={dictionary} language={language} />
      </div>
    </section>
  );
}

function Metric({ label, value, accent, warning }: { label: string; value: number; accent?: boolean; warning?: boolean }) {
  return (
    <div className="metric">
      <Typography.Text>{label}</Typography.Text>
      <strong className={accent ? "accent" : warning ? "warning" : ""}>{value}</strong>
    </div>
  );
}

function CapabilityInspector({
  capability,
  dictionary,
  language,
}: {
  capability?: CapabilityView;
  dictionary: Dictionary;
  language: Language;
}) {
  if (!capability) {
    return <aside className="capability-inspector empty" />;
  }

  const dependencyChain = capability.dependencies.length > 0 ? [...capability.dependencies, capability.id] : [capability.id];

  return (
    <aside className="capability-inspector">
      <div className="inspector-header">
        <div className="inspector-icon">
          <ApiOutlined />
        </div>
        <div>
          <Typography.Title level={3}>{capability.label[language]}</Typography.Title>
          <Tag className={`kind-tag kind-${capability.kind}`}>{kindLabel(dictionary, capability.kind)}</Tag>
        </div>
      </div>

      <dl className="detail-list">
        <div>
          <dt>{dictionary.code}</dt>
          <dd>{capability.id}</dd>
        </div>
        <div>
          <dt>{dictionary.domain}</dt>
          <dd>{capability.domain[language]}</dd>
        </div>
        <div>
          <dt>{dictionary.version}</dt>
          <dd>{capability.version}</dd>
        </div>
        <div>
          <dt>{dictionary.owner}</dt>
          <dd>{capability.owner}</dd>
        </div>
        <div>
          <dt>{dictionary.description}</dt>
          <dd>{capability.description[language]}</dd>
        </div>
      </dl>

      <section className="inspector-section">
        <Typography.Text strong>{dictionary.dependencyChain}</Typography.Text>
        <div className="dependency-chain">
          {dependencyChain.map((id, index) => (
            <span className={id === capability.id ? "dependency-node current" : "dependency-node"} key={`${id}-${index}`}>
              {id}
            </span>
          ))}
        </div>
      </section>

      <section className="inspector-section">
        <div className="section-title-row compact">
          <Typography.Text strong>{dictionary.providedApis}</Typography.Text>
          <Button size="small" type="link">
            {dictionary.viewDocs}
          </Button>
        </div>
        <div className="api-list">
          {capability.apis.length > 0 ? (
            capability.apis.map((api) => (
              <div className="api-row" key={`${api.method}-${api.path}`}>
                <Tag>{api.method}</Tag>
                <span>{api.path}</span>
                <small>{api.summary[language]}</small>
              </div>
            ))
          ) : (
            <Typography.Text className="secondary-text">{dictionary.noDependency}</Typography.Text>
          )}
        </div>
      </section>

      <div className="inspector-actions">
        <Button icon={<StopOutlined />} danger>
          {dictionary.disable}
        </Button>
        <Button icon={<SettingOutlined />}>{dictionary.configure}</Button>
        <Button icon={<FileSearchOutlined />}>{dictionary.viewDocs}</Button>
      </div>
    </aside>
  );
}

function kindLabel(dictionary: Dictionary, kind: CapabilityKind) {
  const labels = {
    core: dictionary.coreCapability,
    plugin: dictionary.pluginCapability,
    optional: dictionary.optionalCapability,
    disabled: dictionary.disabledCapability,
  };
  return labels[kind];
}
