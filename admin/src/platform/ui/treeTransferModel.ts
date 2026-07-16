export type TreeTransferNode = {
  key: string;
  parentKey?: string;
  kind: "branch" | "leaf";
  label: string;
  code?: string;
  status?: string;
  disabledReason?: string;
  availableDisabledReason?: string;
  childCount?: number;
};

export type TreeTransferIndex = {
  readonly byKey: ReadonlyMap<string, TreeTransferNode>;
  readonly childrenByParent: ReadonlyMap<string, readonly string[]>;
  readonly leafDescendantsByBranch: ReadonlyMap<string, readonly string[]>;
  readonly leafKeys: ReadonlySet<string>;
};

export function buildTreeTransferIndex(nodes: readonly TreeTransferNode[]): TreeTransferIndex {
  const byKey = new Map<string, TreeTransferNode>();
  const children = new Map<string, string[]>();
  const leafKeys = new Set<string>();
  for (const node of nodes) {
    byKey.set(node.key, node);
    const parent = node.parentKey ?? "";
    const siblings = children.get(parent) ?? [];
    siblings.push(node.key);
    children.set(parent, siblings);
    if (node.kind === "leaf") leafKeys.add(node.key);
  }

  const leafDescendantsByBranch = new Map<string, readonly string[]>();
  const visit = (key: string, path = new Set<string>()): readonly string[] => {
    const cached = leafDescendantsByBranch.get(key);
    if (cached) return cached;
    if (path.has(key)) return [];
    const node = byKey.get(key);
    if (!node) return [];
    if (node.kind === "leaf") return [key];
    const nextPath = new Set(path).add(key);
    const descendants: string[] = [];
    for (const child of children.get(key) ?? []) descendants.push(...visit(child, nextPath));
    const result = uniqueSorted(descendants);
    leafDescendantsByBranch.set(key, result);
    return result;
  };
  for (const node of nodes) if (node.kind === "branch") visit(node.key);

  return {
    byKey,
    childrenByParent: new Map([...children].map(([key, value]) => [key, uniqueSorted(value)])),
    leafDescendantsByBranch,
    leafKeys,
  };
}

export function leafValues(index: TreeTransferIndex, values: readonly string[]): string[] {
  return uniqueSorted(values.filter((key) => index.leafKeys.has(key)));
}

export function filteredNodeKeys(index: TreeTransferIndex, query: string): Set<string> {
  const normalized = query.trim().toLocaleLowerCase();
  if (!normalized) return new Set(index.byKey.keys());
  const matches = new Set<string>();
  for (const node of index.byKey.values()) {
    if (!`${node.label} ${node.code ?? ""}`.toLocaleLowerCase().includes(normalized)) continue;
    matches.add(node.key);
    let parent = node.parentKey;
    while (parent) {
      if (matches.has(parent)) break;
      matches.add(parent);
      parent = index.byKey.get(parent)?.parentKey;
    }
  }
  return matches;
}

export function deriveTreeTransferSelection(
  index: TreeTransferIndex,
  selectedLeafKeys: readonly string[],
  visibleKeys: ReadonlySet<string>,
  eligibleLeafKeys: ReadonlySet<string>,
): { checkedKeys: string[]; halfCheckedKeys: string[] } {
  const selected = new Set(selectedLeafKeys.filter((key) => index.leafKeys.has(key)));
  const checked = new Set<string>([...selected].filter((key) => visibleKeys.has(key)));
  const half = new Set<string>();
  for (const [branchKey, descendants] of index.leafDescendantsByBranch) {
    if (!visibleKeys.has(branchKey)) continue;
    const eligible = descendants.filter((key) => eligibleLeafKeys.has(key));
    if (eligible.length === 0) continue;
    let selectedCount = 0;
    for (const key of eligible) if (selected.has(key)) selectedCount += 1;
    if (selectedCount === eligible.length) checked.add(branchKey);
    else if (selectedCount > 0) half.add(branchKey);
  }
  return { checkedKeys: uniqueSorted([...checked]), halfCheckedKeys: uniqueSorted([...half]) };
}

function uniqueSorted(values: readonly string[]): string[] {
  return [...new Set(values)].sort();
}
