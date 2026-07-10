import { TreeSelect, type TreeSelectProps } from "antd";
import { type ReactNode, useMemo } from "react";
import { platformPopupContainer } from "./PlatformDropdownPlugin";

export type PlatformTreeSelectOption = {
  value: string;
  label: ReactNode;
  parentValue?: string;
  pathValue?: string;
};

type PlatformTreeSelectProps = Omit<TreeSelectProps, "treeData"> & {
  options: PlatformTreeSelectOption[];
};

type TreeNode = {
  title: ReactNode;
  value: string;
  key: string;
  children?: TreeNode[];
};

export function PlatformTreeSelect({ options, ...treeSelectProps }: PlatformTreeSelectProps) {
  const treeData = useMemo(() => treeDataFromOptions(options), [options]);

  return (
    <TreeSelect
      {...treeSelectProps}
      allowClear={treeSelectProps.allowClear ?? true}
      getPopupContainer={treeSelectProps.getPopupContainer ?? platformPopupContainer}
      maxTagCount={treeSelectProps.maxTagCount ?? "responsive"}
      showSearch={treeSelectProps.showSearch ?? true}
      treeData={treeData}
      treeDefaultExpandAll={treeSelectProps.treeDefaultExpandAll ?? options.length <= 80}
      treeNodeFilterProp="title"
    />
  );
}

function treeDataFromOptions(options: PlatformTreeSelectOption[]) {
  const nodes = new Map<string, TreeNode>();
  const roots: TreeNode[] = [];

  for (const option of options) {
    if (!option.value) {
      continue;
    }
    nodes.set(option.value, {
      title: option.pathValue ? `${option.label} · ${option.pathValue}` : option.label,
      value: option.value,
      key: option.value,
      children: [],
    });
  }

  for (const option of options) {
    const node = nodes.get(option.value);
    if (!node) {
      continue;
    }
    const parentValue = option.parentValue ?? "";
    const parent = parentValue && parentValue !== option.value ? nodes.get(parentValue) : undefined;
    if (parent) {
      parent.children = [...(parent.children ?? []), node];
      continue;
    }
    roots.push(node);
  }

  return pruneEmptyChildren(roots);
}

function pruneEmptyChildren(nodes: TreeNode[]): TreeNode[] {
  return nodes.map((node) => {
    const children = node.children ? pruneEmptyChildren(node.children) : [];
    return children.length > 0 ? { ...node, children } : { title: node.title, value: node.value, key: node.key };
  });
}
