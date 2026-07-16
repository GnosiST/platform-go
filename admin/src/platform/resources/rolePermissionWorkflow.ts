import type { AdminResourceInput, AdminResourceRecord } from "../api/client";
import type { RolePolicyChange } from "../api/organizationRBAC";
import type { RolePermissionWriteMode } from "./rolePermissionWriteMode";

export type RolePermissionAuthorization = {
  role: AdminResourceRecord;
  writeMode: RolePermissionWriteMode;
  allow: string[];
  deny: string[];
  dataScope: string;
  dataScopeOrgCodes: string[];
  dataScopeAreaCodes: string[];
};

type RolePermissionCatalogSources<TRecord> = {
  target: (roleCode: string) => Promise<TRecord[]>;
  generic: () => Promise<TRecord[]>;
};

type RolePermissionWriteClients<TPreview extends { previewId: string }, TImpact> = {
  updateAdminResource: (resource: string, id: string, input: AdminResourceInput) => Promise<unknown>;
  prepare: (roleCode: string, policy: RolePolicyChange) => Promise<TPreview>;
  impact: (previewId: string) => Promise<TImpact | undefined>;
  confirm: (impact: TImpact) => Promise<boolean>;
  replace: (preview: TPreview) => Promise<unknown>;
};

export function loadRolePermissionCatalog<TRecord>(
  writeMode: RolePermissionWriteMode,
  roleCode: string,
  sources: RolePermissionCatalogSources<TRecord>,
) {
  return writeMode === "target-domain" ? sources.target(roleCode) : sources.generic();
}

export async function executeRolePermissionWrite<TPreview extends { previewId: string }, TImpact>(
  authorization: RolePermissionAuthorization,
  canUpdateRole: boolean,
  clients: RolePermissionWriteClients<TPreview, TImpact>,
) {
  if (!canUpdateRole || authorization.role.status !== "enabled" || authorization.writeMode === "readonly") return "blocked" as const;

  if (authorization.writeMode === "legacy-generic") {
    await clients.updateAdminResource("roles", authorization.role.id, legacyRolePermissionInput(authorization));
    return "applied" as const;
  }

  const preview = await clients.prepare(authorization.role.code, {
    allowPermissionCodes: authorization.allow,
    denyPermissionCodes: authorization.deny,
    dataScope: authorization.dataScope,
    dataScopeOrgCodes: authorization.dataScope === "custom_orgs" ? authorization.dataScopeOrgCodes : [],
    dataScopeAreaCodes: authorization.dataScope === "custom_areas" ? authorization.dataScopeAreaCodes : [],
  });
  const impact = await clients.impact(preview.previewId);
  if (!impact || !await clients.confirm(impact)) return "cancelled" as const;
  await clients.replace(preview);
  return "applied" as const;
}

function legacyRolePermissionInput(authorization: RolePermissionAuthorization): AdminResourceInput {
  return {
    code: authorization.role.code,
    name: authorization.role.name,
    status: authorization.role.status,
    description: authorization.role.description,
    values: {
      ...authorization.role.values,
      permissions: uniqueSorted(authorization.allow).join(","),
      denyPermissions: uniqueSorted(authorization.deny).join(","),
      dataScope: authorization.dataScope,
      dataScopeOrgCodes: (authorization.dataScope === "custom_orgs" ? uniqueSorted(authorization.dataScopeOrgCodes) : []).join(","),
      dataScopeAreaCodes: (authorization.dataScope === "custom_areas" ? uniqueSorted(authorization.dataScopeAreaCodes) : []).join(","),
    },
  };
}

function uniqueSorted(values: string[]) {
  return [...new Set(values)].sort();
}
