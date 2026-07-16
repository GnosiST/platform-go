import {
  AdminServiceObjectClient,
  type AdminServiceObjectMenuDefinition,
  type AdminServiceObjectMenuParameter,
  type AdminServiceObjectPageButton,
  type AdminServiceObjectResponse,
  type AdminServiceObjectRoleRemediation,
  type OrganizationRoleGroupChangeConflictsV1_0_0Item,
  type OrganizationRoleGroupChangeImpactV1_0_0Item,
  type OrganizationRolePoolGetV1_0_0Item,
  type OrganizationRoleGroupChangePrepareV1_0_0Values,
  type NavigationRoleMenuChangePrepareV1_0_0Values,
  type NavigationRoleMenuChangeImpactV1_0_0Item,
  type RolePermissionChangeImpactV1_0_0Item,
  type RolePermissionChangePrepareV1_0_0Values,
  type RoleStateOrGroupChangeConflictsV1_0_0Item,
  type RoleStateOrGroupChangeImpactV1_0_0Item,
  type RoleStateOrGroupChangePrepareV1_0_0Values,
  type UserOrganizationChangePrepareV1_0_0Values,
  type MenuAssignmentTreeSearchV1_0_0Item,
  type MenuAssignmentTreeHydrateV1_0_0Item,
  type PermissionAssignmentTreeSearchV1_0_0Item,
  type PermissionAssignmentTreeHydrateV1_0_0Item,
} from "../../../../resources/generated/admin-service-object-client";
import { request } from "./client";

const transport = {
  async post<TData, TRequest>(path: string, body: TRequest): Promise<AdminServiceObjectResponse<TData>> {
    const platformPath = path.startsWith("/api/") ? path.slice(4) : path;
    const data = await request<TData>(platformPath as `/${string}`, {
      method: "POST",
      body: JSON.stringify(body),
    });
    return { data };
  },
};

const client = new AdminServiceObjectClient(transport);

export const roleMenuMigrationWriteEnabled = false;

export type OrganizationRolePoolItem = OrganizationRolePoolGetV1_0_0Item;
export type OrganizationChangeImpact = OrganizationRoleGroupChangeImpactV1_0_0Item;
export type OrganizationChangeConflict = OrganizationRoleGroupChangeConflictsV1_0_0Item;
export type OrganizationRoleRemediation = AdminServiceObjectRoleRemediation;
export type RoleChangeImpact = RoleStateOrGroupChangeImpactV1_0_0Item;
export type RolePermissionImpact = RolePermissionChangeImpactV1_0_0Item;
export type RoleMenuImpact = NavigationRoleMenuChangeImpactV1_0_0Item;
export type RoleChangeConflict = RoleStateOrGroupChangeConflictsV1_0_0Item;
export type MenuDefinition = AdminServiceObjectMenuDefinition;
export type MenuParameter = AdminServiceObjectMenuParameter;
export type PageButton = AdminServiceObjectPageButton;
export type MenuAssignmentTreeItem = MenuAssignmentTreeSearchV1_0_0Item;
export type MenuAssignmentTreeHydratedItem = MenuAssignmentTreeHydrateV1_0_0Item;
export type PermissionAssignmentTreeItem = PermissionAssignmentTreeSearchV1_0_0Item;
export type PermissionAssignmentTreeHydratedItem = PermissionAssignmentTreeHydrateV1_0_0Item;

export async function searchMenuAssignmentTree(roleCode: string, query: string, page = 1, pageSize = 100) {
  return requireData(await client.searchMenuAssignmentTree({ arguments: { roleCode, query }, pagination: { page, pageSize } })).items;
}

export async function hydrateMenuAssignmentTree(roleCode: string) {
  return collectPages(async (page, pageSize) => requireData(await client.hydrateMenuAssignmentTree({
    arguments: { roleCode }, pagination: { page, pageSize },
  })));
}

export async function searchPermissionAssignmentTree(roleCode: string, query: string, page = 1, pageSize = 100) {
  return requireData(await client.searchPermissionAssignmentTree({ arguments: { roleCode, query }, pagination: { page, pageSize } })).items;
}

export async function hydratePermissionAssignmentTree(roleCode: string) {
  return collectPages(async (page, pageSize) => requireData(await client.hydratePermissionAssignmentTree({
    arguments: { roleCode }, pagination: { page, pageSize },
  })));
}

export async function getMenuDefinition(menuCode: string) {
  const result = requireData(await client.getMenuDefinition({
    arguments: { menuCode },
    pagination: { page: 1, pageSize: 1 },
  })).items[0];
  if (!result) {
    throw new Error("Menu definition is unavailable");
  }
  return result;
}

export async function createMenuDefinition(definition: MenuDefinition, expectedRevision: number) {
  return requireData(await client.createMenuDefinition({
    arguments: { definition, expectedRevision },
    idempotencyKey: idempotencyKey("menu-definition-create"),
  })).values;
}

export async function replaceMenuDefinition(definition: MenuDefinition, expectedRevision: number) {
  return requireData(await client.replaceMenuDefinition({
    arguments: { definition, expectedRevision },
    idempotencyKey: idempotencyKey("menu-definition-replace"),
  })).values;
}

export async function getRoleMenus(roleCode: string) {
  const result = requireData(await client.getRoleMenus({
    arguments: { roleCode },
    pagination: { page: 1, pageSize: 1 },
  })).items[0];
  if (!result) {
    throw new Error("Role menu assignment is unavailable");
  }
  return result;
}

export async function prepareRoleMenuChange(roleCode: string, menuCodes: string[]) {
  return requireData(await client.prepareRoleMenuChange({
    arguments: { roleCode, menuCodes },
    idempotencyKey: idempotencyKey("role-menus-prepare"),
  })).values as NavigationRoleMenuChangePrepareV1_0_0Values;
}

export async function getRoleMenuChangeImpact(previewId: string) {
  return requireData(await client.getRoleMenuChangeImpact({
    arguments: { previewId },
    pagination: { page: 1, pageSize: 1 },
  })).items[0];
}

export async function replaceRoleMenus(preview: NavigationRoleMenuChangePrepareV1_0_0Values) {
  return requireData(await client.replaceRoleMenus({
    arguments: {
      previewId: preview.previewId,
      expectedRevision: preview.expectedRevision,
      impactHash: preview.impactHash,
    },
    idempotencyKey: idempotencyKey("role-menus-apply"),
  })).values;
}

export async function getOrganizationRolePool(orgUnitCode: string) {
  return collectPages(async (page, pageSize) => requireData(await client.getOrganizationRolePool({
    arguments: { orgUnitCode },
    pagination: { page, pageSize },
  })));
}

export async function prepareOrganizationRoleGroupChange(
  orgUnitCode: string,
  roleGroupCodes: string[],
  remediations: ReadonlyArray<OrganizationRoleRemediation> = [],
) {
  return requireData(await client.prepareOrganizationRoleGroupChange({
    arguments: { orgUnitCode, roleGroupCodes, remediations },
    idempotencyKey: idempotencyKey("org-role-groups-prepare"),
  })).values as OrganizationRoleGroupChangePrepareV1_0_0Values;
}

export async function getOrganizationRoleGroupChangeImpact(previewId: string) {
  return requireData(await client.getOrganizationRoleGroupChangeImpact({
    arguments: { previewId },
    pagination: { page: 1, pageSize: 1 },
  })).items[0];
}

export async function getOrganizationRoleGroupChangeConflicts(previewId: string) {
  return collectPages(async (page, pageSize) => requireData(await client.getOrganizationRoleGroupChangeConflicts({
    arguments: { previewId },
    pagination: { page, pageSize },
  })));
}

export async function replaceOrganizationRoleGroups(preview: OrganizationRoleGroupChangePrepareV1_0_0Values) {
  return requireData(await client.replaceOrganizationRoleGroups({
    arguments: {
      previewId: preview.previewId,
      expectedRevision: preview.expectedRevision,
      impactHash: preview.impactHash,
    },
    idempotencyKey: idempotencyKey("org-role-groups-apply"),
  })).values;
}

export async function prepareUserOrganizationChange(
  userCode: string,
  orgUnitCode: string,
  roleCodes: string[],
  remediations: ReadonlyArray<OrganizationRoleRemediation>,
) {
  return requireData(await client.prepareUserOrganizationChange({
    arguments: { userCode, orgUnitCode, roleCodes, remediations },
    idempotencyKey: idempotencyKey("user-organization-prepare"),
  })).values as UserOrganizationChangePrepareV1_0_0Values;
}

export async function getUserOrganizationChangeImpact(previewId: string) {
  return requireData(await client.getUserOrganizationChangeImpact({
    arguments: { previewId },
    pagination: { page: 1, pageSize: 1 },
  })).items[0];
}

export async function changeUserOrganization(preview: UserOrganizationChangePrepareV1_0_0Values) {
  return requireData(await client.changeUserOrganization({
    arguments: {
      previewId: preview.previewId,
      expectedRevision: preview.expectedRevision,
      impactHash: preview.impactHash,
    },
    idempotencyKey: idempotencyKey("user-organization-apply"),
  })).values;
}

export async function prepareRoleStateOrGroupChange(
  roleCode: string,
  operation: "move" | "disable",
  targetGroupCode?: string,
  remediations: ReadonlyArray<OrganizationRoleRemediation> = [],
) {
  return requireData(await client.prepareRoleStateOrGroupChange({
    arguments: { roleCode, operation, targetGroupCode, remediations },
    idempotencyKey: idempotencyKey(`role-${operation}-prepare`),
  })).values as RoleStateOrGroupChangePrepareV1_0_0Values;
}

export async function getRoleStateOrGroupChangeImpact(previewId: string) {
  return requireData(await client.getRoleStateOrGroupChangeImpact({
    arguments: { previewId },
    pagination: { page: 1, pageSize: 1 },
  })).items[0];
}

export async function getRoleStateOrGroupChangeConflicts(previewId: string) {
  return collectPages(async (page, pageSize) => requireData(await client.getRoleStateOrGroupChangeConflicts({
    arguments: { previewId },
    pagination: { page, pageSize },
  })));
}

export async function applyRoleStateOrGroupChange(
  preview: RoleStateOrGroupChangePrepareV1_0_0Values,
  operation: "move" | "disable",
) {
  const input = {
    arguments: {
      previewId: preview.previewId,
      expectedRevision: preview.expectedRevision,
      impactHash: preview.impactHash,
    },
    idempotencyKey: idempotencyKey(`role-${operation}-apply`),
  };
  return operation === "move"
    ? requireData(await client.moveRole(input)).values
    : requireData(await client.disableRole(input)).values;
}

export type RolePolicyChange = {
  readonly allowPermissionCodes: string[];
  readonly denyPermissionCodes: string[];
  readonly dataScope: string;
  readonly dataScopeOrgCodes: string[];
  readonly dataScopeAreaCodes: string[];
};

export async function prepareRolePermissionChange(roleCode: string, policy: RolePolicyChange) {
  return requireData(await client.prepareRolePermissionChange({
    arguments: { roleCode, ...policy },
    idempotencyKey: idempotencyKey("role-permissions-prepare"),
  })).values as RolePermissionChangePrepareV1_0_0Values;
}

export async function getRolePermissionChangeImpact(previewId: string) {
  return requireData(await client.getRolePermissionChangeImpact({
    arguments: { previewId },
    pagination: { page: 1, pageSize: 1 },
  })).items[0];
}

export async function replaceRolePermissions(preview: RolePermissionChangePrepareV1_0_0Values) {
  return requireData(await client.replaceRolePermissions({
    arguments: {
      previewId: preview.previewId,
      expectedRevision: preview.expectedRevision,
      impactHash: preview.impactHash,
    },
    idempotencyKey: idempotencyKey("role-permissions-apply"),
  })).values;
}

function requireData<TData>(response: AdminServiceObjectResponse<TData>): TData {
  if (response.data) {
    return response.data;
  }
  throw new Error(response.error?.message ?? "Service object response is unavailable");
}

async function collectPages<TItem>(
  loadPage: (page: number, pageSize: number) => Promise<{ readonly items: ReadonlyArray<TItem>; readonly pageSize: number }>,
) {
  const pageSize = 1000;
  const items: TItem[] = [];
  for (let page = 1; ; page += 1) {
    const result = await loadPage(page, pageSize);
    items.push(...result.items);
    if (result.items.length < result.pageSize) {
      return items;
    }
  }
}

function idempotencyKey(operation: string) {
  const suffix = typeof crypto !== "undefined" && "randomUUID" in crypto
    ? crypto.randomUUID()
    : `${Date.now()}-${Math.random().toString(16).slice(2)}`;
  return `${operation}:${suffix}`;
}
