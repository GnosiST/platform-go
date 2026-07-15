import {
  AdminServiceObjectClient,
  type AdminServiceObjectResponse,
  type AdminServiceObjectRoleRemediation,
  type OrganizationRoleGroupChangeConflictsV1_0_0Item,
  type OrganizationRoleGroupChangeImpactV1_0_0Item,
  type OrganizationRolePoolGetV1_0_0Item,
  type OrganizationRoleGroupChangePrepareV1_0_0Values,
  type UserOrganizationChangePrepareV1_0_0Values,
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

export type OrganizationRolePoolItem = OrganizationRolePoolGetV1_0_0Item;
export type OrganizationChangeImpact = OrganizationRoleGroupChangeImpactV1_0_0Item;
export type OrganizationChangeConflict = OrganizationRoleGroupChangeConflictsV1_0_0Item;
export type OrganizationRoleRemediation = AdminServiceObjectRoleRemediation;

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
