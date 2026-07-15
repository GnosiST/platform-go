export type MenuTreeProjectionRecord = {
  code: string;
  name: string;
  status: string;
  nodeType: string;
  parentCode?: string;
};

export type MenuTreeProjectionLabels = {
  historicalLabel: string;
  disabledReason: string;
  missingReason: string;
};

export type MenuTreeProjectionNode = {
  key: string;
  parentKey?: string;
  kind: "branch" | "leaf";
  label: string;
  code?: string;
  status?: string;
  availableDisabledReason?: string;
};

export function projectMenuTreeNodes(
  records: MenuTreeProjectionRecord[],
  historicalCodes: string[],
  labels: MenuTreeProjectionLabels,
): MenuTreeProjectionNode[] {
  const catalogCodes = new Set(records.map((record) => record.code));
  const missingCodes = [...new Set(historicalCodes.filter((code) => !catalogCodes.has(code)))].sort();
  const nodes: MenuTreeProjectionNode[] = records.map((record) => ({
    key: record.code,
    parentKey: record.parentCode || undefined,
    kind: record.nodeType === "page" ? "leaf" : "branch",
    label: record.name || record.code,
    code: record.code,
    status: record.status,
    availableDisabledReason: record.status === "enabled" ? undefined : labels.disabledReason,
  }));
  if (missingCodes.length === 0) return nodes;

  const occupiedKeys = new Set([...catalogCodes, ...historicalCodes]);
  let historicalBranchKey = "menu-history";
  let suffix = 0;
  while (occupiedKeys.has(historicalBranchKey)) {
    suffix += 1;
    historicalBranchKey = `menu-history:${suffix}`;
  }
  nodes.push({ key: historicalBranchKey, kind: "branch", label: labels.historicalLabel });
  nodes.push(...missingCodes.map((code) => ({
    key: code,
    parentKey: historicalBranchKey,
    kind: "leaf" as const,
    label: code,
    code,
    availableDisabledReason: labels.missingReason,
  })));
  return nodes;
}

export function pageMenuCodes(nodes: MenuTreeProjectionNode[], values: string[]) {
  const pageCodes = new Set(nodes.filter((node) => node.kind === "leaf" && node.code).map((node) => node.code as string));
  return [...new Set(values.filter((code) => pageCodes.has(code)))].sort();
}
