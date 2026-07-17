export type RolePermissionWriteMode = "legacy-generic" | "target-domain" | "readonly";

type RolePermissionSchema = {
  fields: ReadonlyArray<{
    key: string;
    inForm?: boolean;
    readOnly?: boolean;
  }>;
};

const rolePolicyFields = ["permissions", "denyPermissions", "dataScope", "dataScopeOrgCodes", "dataScopeAreaCodes"] as const;

export function resolveRolePermissionWriteMode(schema: RolePermissionSchema | undefined): RolePermissionWriteMode {
  const fields = rolePolicyFields.map((key) => schema?.fields.find((field) => field.key === key));
  if (fields.some((field) => !field)) return "readonly";
  if (fields.every((field) => field?.inForm !== false && field?.readOnly !== true)) return "legacy-generic";
  if (fields.every((field) => field?.inForm !== true && field?.readOnly === true)) return "target-domain";
  return "readonly";
}
