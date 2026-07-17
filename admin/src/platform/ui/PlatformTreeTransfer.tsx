import { DeleteOutlined, PlusOutlined, SearchOutlined } from "@ant-design/icons";
import { Button, Empty, Grid, Input, Space, Tabs, Tooltip, Tree, Typography, type TreeDataNode, type TreeProps } from "antd";
import { useEffect, useMemo, useState, type RefObject } from "react";
import { buildTreeTransferIndex, deriveTreeTransferSelection, filteredNodeKeys, leafValues, type TreeTransferIndex, type TreeTransferNode } from "./treeTransferModel";
import { treeTransferRootKeys } from "./treeTransferProjection";

export type PlatformTreeTransferNode = TreeTransferNode;

export type PlatformTreeTransferLabels = {
  available: string;
  selected: string;
  search: string;
  empty: string;
  selectAllFiltered: string;
  clear: string;
  selectedCount: (count: number) => string;
  disabledReason: (reason: string) => string;
};

type PlatformTreeTransferProps = {
  ariaLabel: string;
  nodes: PlatformTreeTransferNode[];
  value: string[];
  labels: PlatformTreeTransferLabels;
  revision?: number;
  readOnly?: boolean;
  readOnlyMessage?: string;
  showReadOnlyMessage?: boolean;
  returnFocusRef?: RefObject<HTMLElement>;
  onChange: (keys: string[]) => void;
  onLoadChildren?: (node: PlatformTreeTransferNode) => Promise<void>;
};

export function PlatformTreeTransfer({
  ariaLabel,
  nodes,
  value,
  labels,
  revision,
  readOnly = false,
  readOnlyMessage,
  showReadOnlyMessage = true,
  returnFocusRef,
  onChange,
  onLoadChildren,
}: PlatformTreeTransferProps) {
  const screens = Grid.useBreakpoint();
  const mobile = !screens.md;
  const [search, setSearch] = useState("");
  const index = useMemo(() => buildTreeTransferIndex(nodes), [nodes]);
  const normalizedValue = useMemo(() => leafValues(index, value), [index, value]);
  const valueSet = useMemo(() => new Set(normalizedValue), [normalizedValue]);
  const mutableLeafKeys = useMemo(() => nodes.filter((node) => node.kind === "leaf" && !node.disabledReason).map((node) => node.key), [nodes]);
  const mutableLeafKeySet = useMemo(() => new Set(mutableLeafKeys), [mutableLeafKeys]);
  const selectableLeafKeys = useMemo(() => nodes.filter((node) => node.kind === "leaf" && !node.disabledReason && !node.availableDisabledReason).map((node) => node.key), [nodes]);
  const selectableLeafKeySet = useMemo(() => new Set(selectableLeafKeys), [selectableLeafKeys]);
  const selectedEligibleLeafKeySet = useMemo(() => new Set(normalizedValue.filter((key) => mutableLeafKeySet.has(key))), [mutableLeafKeySet, normalizedValue]);
  const filteredKeys = useMemo(() => filteredNodeKeys(index, search), [index, search]);
  const availableTreeSelection = useMemo(() => deriveTreeTransferSelection(index, normalizedValue, filteredKeys, selectableLeafKeySet), [filteredKeys, index, normalizedValue, selectableLeafKeySet]);
  const selectedTreeSelection = useMemo(() => deriveTreeTransferSelection(index, normalizedValue, filteredKeys, selectedEligibleLeafKeySet), [filteredKeys, index, normalizedValue, selectedEligibleLeafKeySet]);
  const availableTree = useMemo(() => transferTreeData(index, filteredKeys, valueSet, labels, false), [filteredKeys, index, labels, valueSet]);
  const selectedTree = useMemo(() => transferTreeData(index, filteredKeys, valueSet, labels, true), [filteredKeys, index, labels, valueSet]);
  useEffect(() => () => {
    returnFocusRef?.current?.focus({ preventScroll: true });
  }, [returnFocusRef]);

  const onCheck = (visibleLeafKeys: string[], allowedLeafKeys: Set<string>, branchAction: "toggle" | "remove"): NonNullable<TreeProps["onCheck"]> => (_checked, info) => {
    if (readOnly) return;
    const key = String(info.node.key);
    const node = nodes.find((candidate) => candidate.key === key);
    if (!node) return;
    const mutableVisibleSet = new Set(visibleLeafKeys.filter((candidate) => allowedLeafKeys.has(candidate)));
    const targets = node.kind === "leaf"
      ? mutableVisibleSet.has(key) ? [key] : []
      : (index.leafDescendantsByBranch.get(key) ?? []).filter((candidate) => mutableVisibleSet.has(candidate));
    if (targets.length === 0) return;
    const preserved = value.filter((key) => !mutableVisibleSet.has(key));
    const nextVisible = new Set(normalizedValue.filter((candidate) => mutableVisibleSet.has(candidate)));
    const shouldAdd = node.kind === "leaf"
      ? info.checked
      : branchAction === "toggle" && targets.some((candidate) => !nextVisible.has(candidate));
    for (const target of targets) {
      if (shouldAdd) nextVisible.add(target);
      else nextVisible.delete(target);
    }
    onChange(leafValues(index, [...preserved, ...nextVisible]));
  };

  const filteredSelectableLeafKeys = selectableLeafKeys.filter((key) => filteredKeys.has(key));
  const preservedDisabled = value.filter((key) => !mutableLeafKeySet.has(key));

  const loadData: TreeProps["loadData"] = onLoadChildren
    ? async (treeNode) => {
        const node = nodes.find((candidate) => candidate.key === String(treeNode.key));
        if (node) await onLoadChildren(node);
      }
    : undefined;

  const availablePane = (
    <TransferPane
      className="available"
      ariaLabel={`${ariaLabel}: ${labels.available}`}
      checkedKeys={availableTreeSelection.checkedKeys}
      halfCheckedKeys={availableTreeSelection.halfCheckedKeys}
      empty={labels.empty}
      loadData={loadData}
      readOnly={readOnly}
      treeData={availableTree}
      virtual={nodes.length >= 50}
      onCheck={onCheck(selectableLeafKeys.filter((key) => filteredKeys.has(key)), selectableLeafKeySet, "toggle")}
    />
  );
  const selectedPane = (
    <TransferPane
      className="selected"
      ariaLabel={`${ariaLabel}: ${labels.selected}`}
      checkedKeys={selectedTreeSelection.checkedKeys}
      halfCheckedKeys={selectedTreeSelection.halfCheckedKeys}
      empty={labels.empty}
      loadData={loadData}
      readOnly={readOnly}
      treeData={selectedTree}
      virtual={normalizedValue.length >= 50}
      onCheck={onCheck(normalizedValue.filter((key) => filteredKeys.has(key)), mutableLeafKeySet, "remove")}
    />
  );

  return (
    <section className="platform-tree-transfer" aria-label={ariaLabel} data-revision={revision ?? 0}>
      <div className="platform-tree-transfer-toolbar">
        <Input
          aria-label={labels.search}
          autoComplete="off"
          prefix={<SearchOutlined aria-hidden />}
          value={search}
          onChange={(event) => setSearch(event.target.value)}
        />
        <Space size={6} wrap>
          <Button
            disabled={readOnly || filteredSelectableLeafKeys.length === 0}
            icon={<PlusOutlined />}
            onClick={() => onChange(uniqueSorted([...normalizedValue, ...filteredSelectableLeafKeys]))}
          >
            {labels.selectAllFiltered}
          </Button>
          <Button disabled={readOnly || normalizedValue.every((key) => !mutableLeafKeySet.has(key))} icon={<DeleteOutlined />} onClick={() => onChange(leafValues(index, preservedDisabled))}>
            {labels.clear}
          </Button>
        </Space>
      </div>
      {showReadOnlyMessage && readOnlyMessage ? <Typography.Text className="platform-tree-transfer-readonly" type="secondary">{readOnlyMessage}</Typography.Text> : null}
      <div className="platform-tree-transfer-count" aria-live="polite">{labels.selectedCount(normalizedValue.length)}</div>
      {mobile ? (
        <Tabs
          className="platform-tree-transfer-mobile-tabs"
          items={[
            { key: "available", label: labels.available, children: availablePane },
            { key: "selected", label: `${labels.selected} (${normalizedValue.length})`, children: selectedPane },
          ]}
        />
      ) : (
        <div className="platform-tree-transfer-panes">
          <div><Typography.Text strong>{labels.available}</Typography.Text>{availablePane}</div>
          <div><Typography.Text strong>{labels.selected} ({normalizedValue.length})</Typography.Text>{selectedPane}</div>
        </div>
      )}
    </section>
  );
}

function TransferPane({
  className,
  ariaLabel,
  treeData,
  checkedKeys,
  halfCheckedKeys,
  readOnly,
  virtual,
  empty,
  loadData,
  onCheck,
}: {
  className: string;
  ariaLabel: string;
  treeData: TreeDataNode[];
  checkedKeys: string[];
  halfCheckedKeys: string[];
  readOnly: boolean;
  virtual: boolean;
  empty: string;
  loadData?: TreeProps["loadData"];
  onCheck: NonNullable<TreeProps["onCheck"]>;
}) {
  return (
    <div className={`platform-tree-transfer-pane ${className}`} data-virtualized={virtual ? "true" : "false"}>
      {treeData.length === 0 ? <Empty description={empty} image={Empty.PRESENTED_IMAGE_SIMPLE} /> : (
        <Tree
          aria-label={ariaLabel}
          blockNode
          checkable
          checkedKeys={{ checked: checkedKeys, halfChecked: halfCheckedKeys }}
          checkStrictly
          disabled={readOnly}
          height={virtual ? 420 : undefined}
          loadData={loadData}
          selectable={false}
          showLine={{ showLeafIcon: false }}
          treeData={treeData}
          virtual={virtual}
          onCheck={onCheck}
        />
      )}
    </div>
  );
}

function transferTreeData(
  index: TreeTransferIndex,
  filteredKeys: Set<string>,
  selectedKeys: Set<string>,
  labels: PlatformTreeTransferLabels,
  selectedOnly: boolean,
): TreeDataNode[] {
  const build = (node: PlatformTreeTransferNode): TreeDataNode => {
    const children = (index.childrenByParent.get(node.key) ?? [])
      .map((key) => index.byKey.get(key))
      .filter((child): child is PlatformTreeTransferNode => child !== undefined)
      .filter((child) => filteredKeys.has(child.key) && !(selectedOnly && child.kind === "leaf" && !selectedKeys.has(child.key)));
    const unavailable = Boolean(node.disabledReason || !selectedOnly && node.availableDisabledReason);
    const unavailableReason = node.disabledReason ?? node.availableDisabledReason;
    return {
      key: node.key,
      title: (
        <span className="platform-tree-transfer-node">
          <span title={node.label}>{node.label}</span>
          {node.code ? <Typography.Text code>{node.code}</Typography.Text> : null}
          {unavailableReason ? <Tooltip title={labels.disabledReason(unavailableReason)}><Typography.Text type="danger">{unavailableReason}</Typography.Text></Tooltip> : null}
        </span>
      ),
      disableCheckbox: node.kind !== "leaf" && children.length === 0 || unavailable,
      disabled: unavailable,
      isLeaf: node.kind === "leaf",
      children: children.length > 0 ? children.map(build) : undefined,
    };
  };
  return treeTransferRootKeys([...index.byKey.values()])
    .map((key) => index.byKey.get(key))
    .filter((node): node is PlatformTreeTransferNode => node !== undefined)
    .filter((node) => filteredKeys.has(node.key) && !(selectedOnly && node.kind === "leaf" && !selectedKeys.has(node.key)))
    .map(build)
    .filter((node) => !selectedOnly || Boolean(node.children?.length) || selectedKeys.has(String(node.key)));
}

function uniqueSorted(values: string[]) {
  return [...new Set(values)].sort();
}
