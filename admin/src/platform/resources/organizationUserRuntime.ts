import type { AdminResourceRecord, AdminResourceSchema } from "../api/client";

export type OrganizationUserRuntimeMode = "legacy" | "target" | "readonly";

const rolePolicyFields = ["permissions", "denyPermissions", "dataScope", "dataScopeOrgCodes", "dataScopeAreaCodes"] as const;

export function resolveOrganizationUserRuntimeMode(
  schema: Pick<AdminResourceSchema, "fields"> | undefined,
): OrganizationUserRuntimeMode {
  const fields = rolePolicyFields.map((key) => schema?.fields.find((field) => field.key === key));
  if (fields.some((field) => !field)) return "readonly";
  if (fields.every((field) => field?.inForm !== false && field?.readOnly !== true)) return "legacy";
  if (fields.every((field) => field?.inForm === false && field.readOnly === true)) return "target";
  return "readonly";
}

export function organizationUserRuntimeCapabilities(mode: OrganizationUserRuntimeMode) {
  return {
    useServiceObjects: mode === "target",
    allowGenericWrite: mode === "legacy",
  };
}

export function dispatchOrganizationUserWrite<T>(
  mode: OrganizationUserRuntimeMode,
  writers: {
    generic: () => Promise<T>;
    target: () => Promise<T>;
    readonly: () => Promise<T>;
  },
) {
  if (mode === "legacy") return writers.generic();
  if (mode === "target") return writers.target();
  return writers.readonly();
}

export function organizationTreeFieldOption(record: AdminResourceRecord) {
  const label = `${record.name} (${record.code})`;
  return {
    value: record.code,
    label: { zh: label, en: label },
    parentValue: record.values?.parentCode ?? "",
  };
}
