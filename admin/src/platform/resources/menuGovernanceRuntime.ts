import type {
  AdminResourceInput,
  AdminResourceRecord,
  AdminResourceSchema,
} from "../api/client";
import type { MenuDefinition } from "../api/organizationRBAC";
import type { MenuDirectoryLabel } from "../i18n";

export type MenuGovernanceWriteMode = "legacy" | "target";

export function resolveMenuGovernanceWriteMode(
  schema: Pick<AdminResourceSchema, "fields">,
): MenuGovernanceWriteMode {
  return schema.fields.find((field) => field.key === "nodeType")?.required ? "target" : "legacy";
}

export function projectMenuGovernanceRecords(
  records: AdminResourceRecord[],
  mode: MenuGovernanceWriteMode,
  directoryLabels: Record<string, MenuDirectoryLabel> = {},
): AdminResourceRecord[] {
  if (mode === "target") return records;
  const existingCodes = new Set(records.map((record) => record.code));
  const missingDirectories = new Set<string>();
  for (const record of records) {
    let directoryCode = legacyParentCode(record);
    while (directoryCode) {
      if (!existingCodes.has(directoryCode)) missingDirectories.add(directoryCode);
      directoryCode = parentDirectoryCode(directoryCode);
    }
  }
  const directories = [...missingDirectories]
    .sort((left, right) => directoryDepth(left) - directoryDepth(right) || left.localeCompare(right))
    .map((code) => legacyDirectoryRecord(code, directoryLabels));
  return [...directories, ...records];
}

export function isSyntheticLegacyDirectory(record: AdminResourceRecord) {
  return record.values?.syntheticLegacyDirectory === "true";
}

export function legacyMenuWriteInput(
  record: AdminResourceRecord,
  definition: MenuDefinition,
  schema: Pick<AdminResourceSchema, "fields">,
): AdminResourceInput {
  const node = definition.node;
  const editableValues: Record<string, string> = {
    nodeType: node.nodeType,
    parent: node.parentCode,
    parentCode: node.parentCode,
    route: node.route,
    componentKey: node.componentKey,
    resourceCode: node.resourceCode,
    resource: record.values?.resource || node.resourceCode,
    isExternal: String(node.external),
    externalUrl: node.externalUrl,
    openMode: node.openMode,
    parameters: JSON.stringify(node.parameters),
    cacheEnabled: String(node.cacheEnabled),
    hidden: String(node.hidden),
    activeMenuCode: node.activeMenuCode,
    breadcrumbVisible: String(node.breadcrumbVisible),
    icon: node.icon,
    order: String(node.sortOrder),
    titleZh: node.titleZh,
    titleEn: node.titleEn,
    nameZh: node.titleZh,
    nameEn: node.titleEn,
    descriptionZh: node.descriptionZh,
    descriptionEn: node.descriptionEn,
  };
  const candidateValues = { ...record.values, ...editableValues };
  const values = Object.fromEntries(schema.fields
    .filter((field) => field.source === "values" && !field.readOnly && field.sensitivity === "public")
    .flatMap((field) => Object.hasOwn(candidateValues, field.key) ? [[field.key, candidateValues[field.key] ?? ""]] : []));
  return {
    code: record.code,
    name: node.titleEn || node.titleZh || definition.name,
    status: node.status,
    description: node.descriptionEn || node.descriptionZh || definition.description,
    values,
  };
}

function legacyDirectoryRecord(code: string, directoryLabels: Record<string, MenuDirectoryLabel>): AdminResourceRecord {
  const fallback = humanizeDirectoryCode(code);
  const label = directoryLabels[code] ?? { zh: fallback, en: fallback };
  return {
    id: `legacy-directory:${code}`,
    code,
    name: label.en,
    status: "enabled",
    updatedAt: "",
    values: {
      nodeType: "directory",
      parentCode: parentDirectoryCode(code),
      titleZh: label.zh,
      titleEn: label.en,
      syntheticLegacyDirectory: "true",
    },
  };
}

function legacyParentCode(record: AdminResourceRecord) {
  return record.values?.parentCode?.trim() || record.values?.parent?.trim() || "";
}

function parentDirectoryCode(code: string) {
  const segments = code.split("/").map((segment) => segment.trim()).filter(Boolean);
  return segments.length > 1 ? segments.slice(0, -1).join("/") : "";
}

function directoryDepth(code: string) {
  return code.split("/").filter(Boolean).length;
}

function humanizeDirectoryCode(code: string) {
  const segment = code.split("/").filter(Boolean).at(-1) ?? code;
  return segment.replace(/[-_]+/g, " ").replace(/\b\w/g, (character) => character.toUpperCase());
}
