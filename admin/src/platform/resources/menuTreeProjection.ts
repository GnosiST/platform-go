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
  const occupiedKeys = new Set([...catalogCodes, ...historicalCodes]);
  const missingParentKeyByCode = new Map<string, string>();
  const missingParentCodes = [...new Set(records
    .map((record) => record.parentCode)
    .filter((code): code is string => typeof code === "string" && code.length > 0 && !catalogCodes.has(code)))]
    .sort();
  for (const code of missingParentCodes) {
    missingParentKeyByCode.set(code, reserveProjectionKey(code, `menu-parent:${code}`, occupiedKeys));
  }
  const nodes: MenuTreeProjectionNode[] = [
    ...missingParentCodes.map((code) => ({
      key: missingParentKeyByCode.get(code) as string,
      kind: "branch" as const,
      label: code,
      code,
    })),
    ...records.map<MenuTreeProjectionNode>((record) => ({
      key: record.code,
      parentKey: record.parentCode ? missingParentKeyByCode.get(record.parentCode) ?? record.parentCode : undefined,
      kind: record.nodeType === "page" ? "leaf" : "branch",
      label: record.name || record.code,
      code: record.code,
      status: record.status,
      availableDisabledReason: record.status === "enabled" ? undefined : labels.disabledReason,
    })),
  ];
  if (missingCodes.length === 0) return nodes;

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

function reserveProjectionKey(preferred: string, fallback: string, occupiedKeys: Set<string>) {
  let key = occupiedKeys.has(preferred) ? fallback : preferred;
  let suffix = 0;
  while (occupiedKeys.has(key)) {
    suffix += 1;
    key = `${fallback}:${suffix}`;
  }
  occupiedKeys.add(key);
  return key;
}

export function pageMenuCodes(nodes: MenuTreeProjectionNode[], values: string[]) {
  const pageCodes = new Set(nodes.filter((node) => node.kind === "leaf" && node.code).map((node) => node.code as string));
  return [...new Set(values.filter((code) => pageCodes.has(code)))].sort();
}
