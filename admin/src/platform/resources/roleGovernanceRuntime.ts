import type {
  AdminResourceInput,
  AdminResourceRecord,
  AdminResourceSchema,
} from "../api/client";
import type { Language } from "../i18n";
import type { AdminTreeWorkbenchNode } from "../ui";
import { resolveRolePermissionWriteMode, type RolePermissionWriteMode } from "./rolePermissionWriteMode";

export type RoleGroupWriteMode = "legacy" | "target" | "readonly";

export type RoleGovernanceRuntime = {
  groupWriteMode: RoleGroupWriteMode;
  permissionWriteMode: RolePermissionWriteMode;
  roleLifecycleTargetEnabled: boolean;
  roleMenuTargetEnabled: boolean;
};

type MetadataEditor = {
  kind: "group" | "role";
  record?: AdminResourceRecord;
  groupCode?: string;
};

type MetadataValues = {
  code: string;
  name: string;
  description?: string;
  scopeType?: string;
  tenantCode?: string;
  groupCode?: string;
  sortOrder?: number;
};

type AssignmentSearch<T> = (roleCode: string, query: string, page: number, pageSize: number) => Promise<readonly T[]>;

const ROLE_ASSIGNMENT_PAGE_SIZE = 100;

export function resolveRoleGovernanceRuntime(
  roleGroupSchema: Pick<AdminResourceSchema, "fields">,
  roleSchema: Pick<AdminResourceSchema, "fields">,
  menuSchema: Pick<AdminResourceSchema, "fields">,
): RoleGovernanceRuntime {
  const groupWriteMode = resolveRoleGroupWriteMode(roleGroupSchema);
  const permissionWriteMode = resolveRolePermissionWriteMode(roleSchema);
  const targetIdentityRuntime = groupWriteMode === "target" && permissionWriteMode === "target-domain";
  const targetMenuSchema = menuSchema.fields.find((field) => field.key === "nodeType")?.required === true;
  return {
    groupWriteMode,
    permissionWriteMode,
    roleLifecycleTargetEnabled: targetIdentityRuntime,
    roleMenuTargetEnabled: targetIdentityRuntime && targetMenuSchema,
  };
}

export function buildRoleGovernanceMetadataInput(
  editor: MetadataEditor,
  values: MetadataValues,
  schema: Pick<AdminResourceSchema, "fields">,
  groupWriteMode: RoleGroupWriteMode,
): AdminResourceInput {
  const candidateValues: Record<string, string> = { ...(editor.record?.values ?? {}) };
  if (editor.kind === "group") {
    candidateValues.parentCode = editor.record?.values?.parentCode ?? "";
    candidateValues.sortOrder = String(values.sortOrder ?? 0);
    if (groupWriteMode === "target") {
      candidateValues.scopeType = values.scopeType ?? "tenant";
      candidateValues.tenantCode = values.scopeType === "platform" ? "" : values.tenantCode ?? "";
    }
  } else {
    candidateValues.groupCode = values.groupCode ?? editor.groupCode ?? editor.record?.values?.groupCode ?? "";
  }
  const writableKeys = new Set(schema.fields
    .filter((field) => field.source === "values" && !field.readOnly && field.sensitivity === "public")
    .map((field) => field.key));
  const filteredValues = Object.fromEntries(Object.entries(candidateValues).filter(([key]) => writableKeys.has(key)));
  return {
    code: editor.record?.code ?? values.code,
    name: values.name,
    status: editor.record?.status ?? "enabled",
    description: values.description,
    values: filteredValues,
  };
}

export async function loadRoleAssignmentSearchPages<T>(roleCode: string, search: AssignmentSearch<T>): Promise<T[]> {
  const records: T[] = [];
  for (let page = 1; ; page += 1) {
    const items = await search(roleCode, "", page, ROLE_ASSIGNMENT_PAGE_SIZE);
    records.push(...items);
    if (items.length < ROLE_ASSIGNMENT_PAGE_SIZE) return records;
  }
}

export function localizedGovernanceName(record: AdminResourceRecord, language: Language) {
  const localizedKeys = language === "zh" ? ["nameZh", "titleZh"] : ["nameEn", "titleEn"];
  for (const key of localizedKeys) {
    const value = record.values?.[key]?.trim();
    if (value) return value;
  }
  return record.name || record.code;
}

export function localizedGovernanceDescription(record: AdminResourceRecord, language: Language) {
  const localizedKey = language === "zh" ? "descriptionZh" : "descriptionEn";
  return record.values?.[localizedKey]?.trim() || record.description || "";
}

export function projectRoleGovernanceTree(
  groups: AdminResourceRecord[],
  roles: AdminResourceRecord[],
  search: string,
  language: Language,
  uncategorizedLabel: string,
): AdminTreeWorkbenchNode[] {
  const normalized = search.trim().toLocaleLowerCase();
  const groupsByCode = new Map(groups.map((group) => [group.code, group]));
  const rolesByGroup = new Map<string, AdminResourceRecord[]>();
  for (const role of roles) {
    const groupCode = role.values?.groupCode?.trim() ?? "";
    rolesByGroup.set(groupCode, [...(rolesByGroup.get(groupCode) ?? []), role]);
  }

  const visibleGroupCodes = new Set<string>();
  for (const group of groups) {
    const groupMatches = matchesRecord(group, normalized, language);
    const matchingRoles = (rolesByGroup.get(group.code) ?? []).filter((role) => matchesRecord(role, normalized, language));
    if (!normalized || groupMatches || matchingRoles.length > 0) visibleGroupCodes.add(group.code);
  }
  for (const [groupCode, groupedRoles] of rolesByGroup) {
    if (!groupsByCode.has(groupCode) && (!normalized || groupedRoles.some((role) => matchesRecord(role, normalized, language)))) {
      visibleGroupCodes.add(groupCode);
    }
  }
  for (const code of [...visibleGroupCodes]) {
    let parentCode = groupsByCode.get(code)?.values?.parentCode?.trim() ?? "";
    const visited = new Set<string>();
    while (parentCode && !visited.has(parentCode)) {
      visited.add(parentCode);
      visibleGroupCodes.add(parentCode);
      parentCode = groupsByCode.get(parentCode)?.values?.parentCode?.trim() ?? "";
    }
  }

  const placeholderCodes = new Set<string>();
  for (const code of visibleGroupCodes) {
    if (code && !groupsByCode.has(code)) placeholderCodes.add(code);
  }
  const ungroupedVisible = visibleGroupCodes.has("");
  const nodes: AdminTreeWorkbenchNode[] = [];
  for (const code of [...placeholderCodes].sort()) {
    nodes.push({ key: groupNodeKey(code), kind: "group", label: code, childCount: rolesByGroup.get(code)?.length ?? 0, selectable: false });
  }
  if (ungroupedVisible) {
    nodes.push({ key: groupNodeKey(""), kind: "group", label: uncategorizedLabel, childCount: rolesByGroup.get("")?.length ?? 0, selectable: false });
  }
  for (const group of groups) {
    if (!visibleGroupCodes.has(group.code)) continue;
    const parentCode = group.values?.parentCode?.trim() ?? "";
    const groupedRoles = rolesByGroup.get(group.code) ?? [];
    nodes.push({
      key: groupNodeKey(group.code),
      parentKey: parentCode && parentCode !== group.code ? groupNodeKey(parentCode) : undefined,
      kind: "group",
      label: `${localizedGovernanceName(group, language)} (${group.code})`,
      childCount: groupedRoles.length,
    });
    for (const role of groupedRoles) {
      if (normalized && !matchesRecord(group, normalized, language) && !matchesRecord(role, normalized, language)) continue;
      nodes.push(roleNode(role, group.code, language));
    }
  }
  for (const code of [...placeholderCodes, ...(ungroupedVisible ? [""] : [])]) {
    for (const role of rolesByGroup.get(code) ?? []) {
      if (!normalized || matchesRecord(role, normalized, language)) nodes.push(roleNode(role, code, language));
    }
  }
  return nodes;
}

function resolveRoleGroupWriteMode(schema: Pick<AdminResourceSchema, "fields">): RoleGroupWriteMode {
  const scopeType = schema.fields.find((field) => field.key === "scopeType");
  const tenantCode = schema.fields.find((field) => field.key === "tenantCode");
  if (!scopeType || !tenantCode) return "readonly";
  if (scopeType.required && !scopeType.readOnly && scopeType.inForm !== false && !tenantCode.readOnly && tenantCode.inForm !== false) return "target";
  if (scopeType.readOnly && scopeType.inForm === false && tenantCode.readOnly && tenantCode.inForm === false) return "legacy";
  return "readonly";
}

function matchesRecord(record: AdminResourceRecord, normalized: string, language: Language) {
  if (!normalized) return true;
  return `${localizedGovernanceName(record, language)} ${record.name} ${record.code}`.toLocaleLowerCase().includes(normalized);
}

function roleNode(role: AdminResourceRecord, groupCode: string, language: Language): AdminTreeWorkbenchNode {
  return {
    key: `role:${role.code}`,
    parentKey: groupNodeKey(groupCode),
    kind: "item",
    label: `${localizedGovernanceName(role, language)} (${role.code})`,
    status: role.status,
    isLeaf: true,
  };
}

function groupNodeKey(code: string) {
  return `group:${code || "__uncategorized__"}`;
}
