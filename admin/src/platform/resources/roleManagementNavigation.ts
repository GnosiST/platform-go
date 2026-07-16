const ROLES_ROUTE = "/roles";
const ROLE_GROUPS_ROUTE = "/role-groups";

export type RoleManagementNavigationTitle = {
  readonly zh: string;
  readonly en: string;
};

export type RoleManagementNavigationResource = {
  readonly route: string;
  readonly title: RoleManagementNavigationTitle;
};

export type ProjectedRoleManagementNavigationResource<T extends RoleManagementNavigationResource> =
  Omit<T, "title"> & { title: RoleManagementNavigationTitle };

export function selectRoleManagementNavigationResource<T extends RoleManagementNavigationResource>(
  resources: readonly T[],
): T | undefined {
  return resources.find((resource) => resource.route === ROLES_ROUTE)
    ?? resources.find((resource) => resource.route === ROLE_GROUPS_ROUTE);
}

export function projectRoleManagementNavigation<T extends RoleManagementNavigationResource>(
  resources: readonly T[],
  title: RoleManagementNavigationTitle,
): ProjectedRoleManagementNavigationResource<T>[] {
  const firstRoleIndex = resources.findIndex((resource) => isRoleManagementRoute(resource.route));
  if (firstRoleIndex === -1) return [...resources];

  const selected = selectRoleManagementNavigationResource(resources);
  if (!selected) return [...resources];

  const projectedSelected: ProjectedRoleManagementNavigationResource<T> = { ...selected, title: { ...title } };
  const projected: ProjectedRoleManagementNavigationResource<T>[] = [];
  resources.forEach((resource, index) => {
    if (index === firstRoleIndex) projected.push(projectedSelected);
    if (!isRoleManagementRoute(resource.route)) projected.push(resource);
  });
  return projected;
}

export function resolveRoleManagementActiveRoute(
  route: string,
  projectedResources: readonly RoleManagementNavigationResource[],
): string {
  if (!isRoleManagementRoute(route)) return route;
  return selectRoleManagementNavigationResource(projectedResources)?.route ?? route;
}

function isRoleManagementRoute(route: string): boolean {
  return route === ROLES_ROUTE || route === ROLE_GROUPS_ROUTE;
}
