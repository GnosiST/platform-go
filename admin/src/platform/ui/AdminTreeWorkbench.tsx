import { SearchOutlined } from "@ant-design/icons";
import { Empty, Input, Spin, Tag, Tree, Typography, type TreeDataNode, type TreeProps } from "antd";
import { useEffect, useMemo, useState, type ReactNode } from "react";
import { AdminListPanel } from "./AdminPrimitives";

export type AdminTreeWorkbenchNode = {
  key: string;
  parentKey?: string;
  kind: "group" | "item";
  label: ReactNode;
  subtitle?: ReactNode;
  meta?: ReactNode;
  searchText?: string;
  status?: string;
  statusLabel?: ReactNode;
  disabledReason?: string;
  childCount?: number;
  selectable?: boolean;
  isLeaf?: boolean;
};

type AdminTreeWorkbenchProps = {
  ariaLabel: string;
  title: ReactNode;
  searchLabel: string;
  searchPlaceholder: string;
  emptyText: ReactNode;
  nodes: AdminTreeWorkbenchNode[];
  selectedKey?: string;
  searchValue: string;
  loading?: boolean;
  actions?: ReactNode;
  detail: ReactNode;
  onSearchChange: (value: string) => void;
  onSelect: (key: string) => void;
  onLoadChildren?: (node: AdminTreeWorkbenchNode) => Promise<void>;
};

export function AdminTreeWorkbench({
  ariaLabel,
  title,
  searchLabel,
  searchPlaceholder,
  emptyText,
  nodes,
  selectedKey,
  searchValue,
  loading = false,
  actions,
  detail,
  onSearchChange,
  onSelect,
  onLoadChildren,
}: AdminTreeWorkbenchProps) {
  const nodeByKey = useMemo(() => new Map(nodes.map((node) => [node.key, node])), [nodes]);
  const treeData = useMemo(() => workbenchTreeData(nodes, selectedKey), [nodes, selectedKey]);
  const expandedKeys = useMemo(() => treeData.map((node) => String(node.key)), [treeData]);
  const [activeKey, setActiveKey] = useState<string | null>(null);
  const virtual = nodes.length >= 50;
  const loadData: TreeProps["loadData"] = onLoadChildren
    ? async (treeNode) => {
        const node = nodeByKey.get(String(treeNode.key));
        if (node) {
          await onLoadChildren(node);
        }
      }
    : undefined;

  useEffect(() => {
    setActiveKey(selectedKey || firstTreeKey(treeData));
  }, [selectedKey, treeData]);

  return (
    <section className="admin-tree-workbench" aria-label={ariaLabel}>
      <AdminListPanel
        className="admin-tree-workbench-navigation"
        title={title}
        actions={actions}
        toolbar={(
          <Input
            aria-label={searchLabel}
            autoComplete="off"
            prefix={<SearchOutlined aria-hidden />}
            placeholder={searchPlaceholder}
            value={searchValue}
            onChange={(event) => onSearchChange(event.target.value)}
          />
        )}
      >
        <div className="admin-tree-workbench-tree" data-virtualized={virtual ? "true" : "false"}>
          {loading ? (
            <div className="loading-panel" aria-live="polite"><Spin size="small" /></div>
          ) : treeData.length === 0 ? (
            <Empty description={emptyText} image={Empty.PRESENTED_IMAGE_SIMPLE} />
          ) : (
            <Tree
              activeKey={activeKey}
              aria-label={ariaLabel}
              blockNode
              defaultExpandedKeys={expandedKeys}
              height={virtual ? 520 : undefined}
              loadData={loadData}
              selectedKeys={selectedKey ? [selectedKey] : []}
              showLine={{ showLeafIcon: false }}
              treeData={treeData}
              virtual={virtual}
              onActiveChange={(key) => setActiveKey(String(key))}
              onSelect={(keys) => {
                const key = keys[0];
                if (key !== undefined) {
                  onSelect(String(key));
                }
              }}
            />
          )}
        </div>
      </AdminListPanel>
      <div className="admin-tree-workbench-detail">{detail}</div>
    </section>
  );
}

function firstTreeKey(treeData: TreeDataNode[]) {
  return treeData[0] ? String(treeData[0].key) : null;
}

type WorkbenchTreeDataNode = TreeDataNode & {
  "aria-level": number;
  "aria-selected": boolean;
  children?: WorkbenchTreeDataNode[];
};

function workbenchTreeData(nodes: AdminTreeWorkbenchNode[], selectedKey?: string): WorkbenchTreeDataNode[] {
  const childrenByParent = new Map<string, AdminTreeWorkbenchNode[]>();
  for (const node of nodes) {
    const parentKey = node.parentKey ?? "";
    childrenByParent.set(parentKey, [...(childrenByParent.get(parentKey) ?? []), node]);
  }

  const build = (node: AdminTreeWorkbenchNode, depth: number): WorkbenchTreeDataNode => {
    const children = childrenByParent.get(node.key) ?? [];
    const title = typeof node.label === "string" ? node.label : node.searchText;
    return {
      key: node.key,
      title: (
        <span className="admin-tree-workbench-node" data-kind={node.kind}>
          <span className="admin-tree-workbench-node-marker" aria-hidden />
          <span className="admin-tree-workbench-node-copy">
            <span className="admin-tree-workbench-node-main">
              <span className="admin-tree-workbench-node-label" title={title}>{node.label}</span>
              {node.kind === "group" && typeof node.childCount === "number" ? (
                <Typography.Text className="admin-tree-workbench-node-count" type="secondary">{node.childCount}</Typography.Text>
              ) : null}
            </span>
            {node.subtitle ? <span className="admin-tree-workbench-node-subtitle">{node.subtitle}</span> : null}
            {node.meta || node.status || node.disabledReason ? (
              <span className="admin-tree-workbench-node-meta">
                {node.status ? <Tag>{node.statusLabel ?? node.status}</Tag> : null}
                {node.meta}
                {node.disabledReason ? <Typography.Text type="danger">{node.disabledReason}</Typography.Text> : null}
              </span>
            ) : null}
          </span>
        </span>
      ),
      disabled: Boolean(node.disabledReason),
      isLeaf: node.isLeaf ?? node.kind === "item",
      selectable: node.selectable ?? true,
      "aria-selected": node.key === selectedKey,
      "aria-level": depth,
      children: children.length > 0 ? children.map((child) => build(child, depth + 1)) : undefined,
    };
  };

  return (childrenByParent.get("") ?? []).map((node) => build(node, 1));
}
