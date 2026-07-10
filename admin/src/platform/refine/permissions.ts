import type { CanParams } from "@refinedev/core";
import { coreResources } from "../resources/registry";

const actionSuffixes: Record<string, "read" | "create" | "update" | "delete"> = {
  list: "read",
  show: "read",
  read: "read",
  create: "create",
  edit: "update",
  update: "update",
  delete: "delete",
  deleteOne: "delete",
};

export function hasPermission(permissions: string[], permission: string, deniedPermissions: string[] = []) {
  if (!permission) {
    return true;
  }
  if (matchesAnyPermission(deniedPermissions, permission)) {
    return false;
  }
  return matchesAnyPermission(permissions, permission);
}

function matchesAnyPermission(permissions: string[], permission: string) {
  return permissions.some((granted) => {
    if (granted === "*" || granted === permission) {
      return true;
    }
    if (granted.endsWith("*")) {
      return permission.startsWith(granted.slice(0, -1));
    }
    return false;
  });
}

export function permissionForRefineAction(params: CanParams) {
  const action = actionSuffixes[params.action] ?? "read";
  const explicitPermission = permissionFromResourceMeta(params);
  if (explicitPermission) {
    return explicitPermission.replace(/:(read|create|update|delete)$/, `:${action}`);
  }

  const resource = params.resource ?? params.params?.resource?.name;
  if (!resource) {
    return "";
  }
  const definition = coreResources.find((item) => item.name === resource);
  if (definition) {
    return definition.permission.replace(/:read$/, `:${action}`);
  }
  return `admin:${camelToKebab(resource)}:${action}`;
}

function permissionFromResourceMeta(params: CanParams) {
  const meta = params.params?.resource?.meta as Record<string, unknown> | undefined;
  const permission = meta?.permission ?? meta?.permissionCode;
  return typeof permission === "string" ? permission : "";
}

function camelToKebab(value: string) {
  return value.replace(/([a-z0-9])([A-Z])/g, "$1-$2").toLowerCase();
}
