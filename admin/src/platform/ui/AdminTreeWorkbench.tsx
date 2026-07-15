import { SearchOutlined } from "@ant-design/icons";
import { Empty, Input, Spin, Tree, Typography, type TreeDataNode, type TreeProps } from "antd";
import { useMemo, type ReactNode } from "react";
import { AdminListPanel } from "./AdminPrimitives";

export type AdminTreeWorkbenchNode = {
  key: string;
  parentKey?: string;
  kind: "group" | "item";
  label: ReactNode;
  searchText?: string;
  status?: string;
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
  const treeData = useMemo(() => workbenchTreeData(nodes), [nodes]);
  const expandedKeys = useMemo(() => treeData.map((node) => String(node.key)), [treeData]);
  const virtual = nodes.length >= 50;
  const loadData: TreeProps["loadData"] = onLoadChildren
    ? async (treeNode) => {
        const node = nodeByKey.get(String(treeNode.key));
        if (node) {
          await onLoadChildren(node);
        }
      }
    : undefined;

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
              aria-label={ariaLabel}
              blockNode
              defaultExpandedKeys={expandedKeys}
              height={virtual ? 520 : undefined}
              loadData={loadData}
              selectedKeys={selectedKey ? [selectedKey] : []}
              showLine={{ showLeafIcon: false }}
              treeData={treeData}
              virtual={virtual}
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

function workbenchTreeData(nodes: AdminTreeWorkbenchNode[]): TreeDataNode[] {
  const childrenByParent = new Map<string, AdminTreeWorkbenchNode[]>();
  for (const node of nodes) {
    const parentKey = node.parentKey ?? "";
    childrenByParent.set(parentKey, [...(childrenByParent.get(parentKey) ?? []), node]);
  }

  const build = (node: AdminTreeWorkbenchNode): TreeDataNode => {
    const children = childrenByParent.get(node.key) ?? [];
    return {
      key: node.key,
      title: (
        <span className="admin-tree-workbench-node">
          <span className="admin-tree-workbench-node-label">{node.label}</span>
          {node.kind === "group" && typeof node.childCount === "number" ? (
            <Typography.Text className="admin-tree-workbench-node-count" type="secondary">{node.childCount}</Typography.Text>
          ) : null}
          {node.disabledReason ? <Typography.Text type="danger">{node.disabledReason}</Typography.Text> : null}
        </span>
      ),
      disabled: Boolean(node.disabledReason),
      isLeaf: node.isLeaf ?? node.kind === "item",
      selectable: node.selectable ?? true,
      children: children.length > 0 ? children.map(build) : undefined,
    };
  };

  return (childrenByParent.get("") ?? []).map(build);
}
