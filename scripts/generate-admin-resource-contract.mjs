import fs from "node:fs";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(__dirname, "..");
const manifestPath = path.join(repoRoot, "resources", "admin-resources.json");
const generatedDir = path.join(repoRoot, "resources", "generated");
const generatedPath = path.join(generatedDir, "admin-resource-contract.json");

const manifest = JSON.parse(fs.readFileSync(manifestPath, "utf8"));
const resources = manifest.resources ?? [];

function loadCapabilityResourceContract() {
  const result = spawnSync("go", ["run", "./cmd/platform-contracts", "admin-resources", "--stdout"], {
    cwd: repoRoot,
    encoding: "utf8",
    env: process.env,
  });
  if (result.status !== 0) {
    throw new Error(`go run ./cmd/platform-contracts admin-resources --stdout failed\n${result.stdout}${result.stderr}`);
  }
  return JSON.parse(result.stdout);
}

function uniqueSorted(values) {
  return Array.from(new Set(values.filter(Boolean))).sort();
}

function permissionValues(resource) {
  return Object.values(resource.permissions ?? {}).filter(Boolean).sort();
}

function routeKey(route) {
  return `${route.method} ${route.path}`;
}

function defaultFilterableField(field) {
  return Boolean(
    field.filter ||
      field.filterable ||
      field.search ||
      field.searchable ||
      field.key === "status" ||
      ["select", "multiselect", "switch", "datetime", "number"].includes(field.type),
  );
}

function defaultSortableField(field) {
  return Boolean(
    field.sort ||
      field.sortable ||
      field.key === "id" ||
      field.key === "updatedAt" ||
      field.type === "datetime" ||
      field.type === "number" ||
      (field.table && field.type !== "textarea" && field.type !== "multiselect"),
  );
}

function defaultLocalizableField(field) {
  return Boolean(field.localize || field.localizable || field.key === "name" || field.key === "description");
}

function normalizedFieldPolicy(field) {
  return {
    sensitivity: field.sensitivity ?? "public",
    storageMode: field.storageMode ?? "plain",
    responseMode: field.responseMode ?? "full",
    exportMode: field.exportMode ?? "full",
  };
}

function normalizeField(field) {
  return { ...field, ...normalizedFieldPolicy(field) };
}

function schemaKeys(resource, listName, predicate) {
  const fields = resource.schema?.fields ?? [];
  const declared = resource.schema?.[listName];
  if (Array.isArray(declared)) {
    return [...declared];
  }
  return fields.filter(predicate).map((field) => field.key);
}

function runtimeRoute(resource, route) {
  const legacyBase = resource.apiBase ?? `/api/admin/${resource.name}`;
  const runtimeResource = resource.code ?? resource.refine?.resource ?? resource.name;
  const runtimeBase = `/api/admin/resources/${runtimeResource}`;
  if (route.path === `${legacyBase}/queries`) {
    return { ...route, path: `${runtimeBase}/query` };
  }
  if (route.path === legacyBase) {
    return { ...route, path: runtimeBase };
  }
  if (route.path === `${legacyBase}/:id`) {
    return { ...route, path: `${runtimeBase}/:id` };
  }
  return route;
}

function auditActionBase(resource) {
  const parts = String(resource).split("-").filter(Boolean);
  const last = parts.pop() ?? resource;
  const singularLast = last.endsWith("ies") ? `${last.slice(0, -3)}y` : last.endsWith("s") ? last.slice(0, -1) : last;
  return [...parts, singularLast].join("_");
}

function capabilityRoutes(resource) {
  const base = `/api/admin/${resource.resource}`;
  const auditBase = auditActionBase(resource.resource);
  const policyReviewAuditBase = resource.resource === "policy-reviews" ? "policy-review" : auditBase;
  const routes = [
    {
      method: "POST",
      path: `${base}/queries`,
      permission: resource.permissions?.read,
    },
    {
      method: "POST",
      path: base,
      permission: resource.permissions?.create,
      auditAction: `${auditBase}.create`,
    },
    {
      method: "PUT",
      path: `${base}/:id`,
      permission: resource.permissions?.update,
      auditAction: `${auditBase}.update`,
    },
    {
      method: "DELETE",
      path: `${base}/:id`,
      permission: resource.permissions?.delete,
      auditAction: `${auditBase}.delete`,
    },
  ];
  if (resource.resource === "policy-reviews" && resource.permissions?.update) {
    for (const action of ["request", "approve", "reject"]) {
      routes.push({
        method: "POST",
        path: `${base}/:id/${action}`,
        permission: resource.permissions.update,
        auditAction: `${policyReviewAuditBase}.${action}`,
      });
    }
  }
  const policyReviewExportPermission = resource.permissionPrefix ? `${resource.permissionPrefix}:export` : "";
  if (resource.resource === "policy-reviews" && policyReviewExportPermission) {
    routes.push({
      method: "GET",
      path: `${base}/export`,
      permission: policyReviewExportPermission,
      auditAction: `${policyReviewAuditBase}.export`,
    });
  }
  return routes.filter((route) => route.permission);
}

function capabilityField(field) {
  return {
    key: field.key,
    label: field.label,
    type: field.type,
    source: field.source,
    group: field.group,
    help: field.help,
    required: field.required === true,
    readOnly: field.readOnly === true,
    search: field.searchable === true,
    filter: field.filterable === true,
    sort: field.sortable === true,
    localize: field.localizable === true,
    table: field.inTable === true,
    form: field.inForm === true,
    detail: field.inDetail === true,
    width: field.width,
    ...normalizedFieldPolicy(field),
    ...(Array.isArray(field.options) && field.options.length > 0 ? { options: field.options } : {}),
    ...(field.relation ? { relation: field.relation } : {}),
    ...(field.validation && Object.keys(field.validation).length > 0 ? { validation: field.validation } : {}),
  };
}

function capabilityResourceToManifestResource(resource) {
  const fields = (resource.fields ?? []).map(capabilityField);
  return {
    name: resource.resource,
    code: resource.resource,
    label: resource.title,
    group: resource.menu?.group ?? resource.capabilityId,
    menu: {
      parentCode: resource.menu?.parent ?? null,
      path: resource.menu?.route,
      icon: resource.menu?.icon,
      sortOrder: resource.menu?.order ?? 0,
      visible: true,
      hidden: false,
      external: resource.menu?.external === true,
      keepAlive: resource.menu?.cache === true,
    },
    refine: {
      resource: resource.resource,
      list: resource.menu?.route,
      component: "ResourceTablePage",
    },
    apiBase: `/api/admin/${resource.resource}`,
    permissions: resource.permissions ?? {},
    actions: resource.actions ?? [],
    panels: resource.panels ?? [],
    routes: capabilityRoutes(resource),
    schema: {
      formGroups: resource.formGroups ?? [],
      formLayout: resource.formLayout ?? defaultFormLayout(fields),
      fields,
      search: resource.searchFields ?? fields.filter((field) => field.search).map((field) => field.key),
      table: fields.filter((field) => field.table).map((field) => field.key),
      form: fields.filter((field) => field.form).map((field) => field.key),
      localizedFields: fields.filter((field) => field.localize).map((field) => field.key),
    },
    codegen: {
      mode: "custom",
      reason: "Capability resource runs through the generic admin resource engine; source-writing scaffold is disabled.",
      capabilityId: resource.capabilityId,
    },
  };
}

function mergedResources() {
  const staticKeys = new Set(resources.flatMap((resource) => [resource.name, resource.code].filter(Boolean)));
  const capabilityContract = loadCapabilityResourceContract();
  const capabilityResources = (capabilityContract.resources ?? [])
    .filter((resource) => resource.resource && !staticKeys.has(resource.resource))
    .map(capabilityResourceToManifestResource);
  return [...resources, ...capabilityResources];
}

function normalizeResource(resource) {
  const routes = [...(resource.routes ?? [])].map((route) => runtimeRoute(resource, route));
  const permissionCodes = uniqueSorted([
    ...permissionValues(resource),
    ...routes.map((route) => route.permission),
    ...(resource.actions ?? []).map((action) => action.permission),
  ]);
  return {
    name: resource.name,
    code: resource.code,
    label: resource.label,
    group: resource.group,
    menu: {
      parentCode: resource.menu?.parentCode ?? null,
      path: resource.menu?.path,
      icon: resource.menu?.icon,
      sortOrder: resource.menu?.sortOrder ?? 0,
      visible: resource.menu?.visible !== false,
      hidden: resource.menu?.hidden === true,
      external: resource.menu?.external === true,
      keepAlive: resource.menu?.keepAlive === true,
    },
    refine: {
      resource: resource.refine?.resource ?? resource.name,
      list: resource.refine?.list ?? resource.menu?.path,
      component: resource.refine?.component ?? "ResourceTablePage",
    },
    apiBase: resource.apiBase,
    permissions: resource.permissions ?? {},
    permissionCodes,
    actions: resource.actions ?? [],
    panels: resource.panels ?? [],
    routes: routes
      .map((route) => ({
        method: route.method,
        path: route.path,
        permission: route.permission,
        ...(route.auditAction ? { auditAction: route.auditAction } : {}),
      }))
      .sort((a, b) => routeKey(a).localeCompare(routeKey(b))),
    schema: {
      formGroups: resource.schema?.formGroups ?? [],
      formLayout: normalizeFormLayout(resource.schema?.formLayout, resource.schema?.fields ?? []),
      fields: [...(resource.schema?.fields ?? [])].map(normalizeField).sort((a, b) => a.key.localeCompare(b.key)),
      search: [...(resource.schema?.search ?? [])].sort(),
      filter: schemaKeys(resource, "filter", defaultFilterableField).sort(),
      sort: schemaKeys(resource, "sort", defaultSortableField).sort(),
      table: resource.schema?.table ?? [],
      form: resource.schema?.form ?? [],
      localizedFields: schemaKeys(resource, "localizedFields", defaultLocalizableField).sort(),
    },
    codegen: resource.codegen ?? { mode: "unknown" },
  };
}

function defaultFormLayout(fields) {
  const formFieldCount = fields.filter((field) => field.form !== false && field.readOnly !== true).length;
  return formFieldCount >= 6 ? "two-column-density" : "grouped-sections";
}

function normalizeFormLayout(layout, fields) {
  if (["single-column", "grouped-sections", "two-column-density"].includes(layout)) {
    return layout;
  }
  return defaultFormLayout(fields);
}

const normalizedResources = mergedResources().map(normalizeResource).sort((a, b) => a.name.localeCompare(b.name));

const contract = {
  generatedBy: "scripts/generate-admin-resource-contract.mjs",
  source: "resources/admin-resources.json + capability.Manifest.Admin.Resources",
  sourceVersion: manifest.version,
  updatedAt: manifest.updatedAt,
  stack: manifest.stack,
  database: manifest.database,
  resourceCount: normalizedResources.length,
  groups: uniqueSorted(normalizedResources.map((resource) => resource.group)),
  permissions: uniqueSorted(normalizedResources.flatMap((resource) => resource.permissionCodes)),
  menus: normalizedResources
    .map((resource) => ({
      resource: resource.name,
      code: resource.code,
      label: resource.label,
      ...resource.menu,
      permission: resource.permissions.read ?? resource.permissionCodes[0],
    }))
    .sort((a, b) => a.sortOrder - b.sortOrder || a.code.localeCompare(b.code)),
  routes: normalizedResources
    .flatMap((resource) =>
      resource.routes.map((route) => ({
        resource: resource.name,
        method: route.method,
        path: route.path,
        permission: route.permission,
        ...(route.auditAction ? { auditAction: route.auditAction } : {}),
      })),
    )
    .sort((a, b) => routeKey(a).localeCompare(routeKey(b))),
  frontend: normalizedResources
    .map((resource) => ({
      resource: resource.refine.resource,
      name: resource.name,
      route: resource.refine.list,
      component: resource.refine.component,
      label: resource.label,
      group: resource.group,
      permission: resource.permissions.read ?? resource.permissionCodes[0],
    }))
    .sort((a, b) => a.route.localeCompare(b.route)),
  schemas: Object.fromEntries(
    normalizedResources.map((resource) => [
      resource.name,
      {
        formGroups: resource.schema.formGroups,
        fields: resource.schema.fields,
        search: resource.schema.search,
        filter: resource.schema.filter,
        sort: resource.schema.sort,
        table: resource.schema.table,
        form: resource.schema.form,
        localizedFields: resource.schema.localizedFields,
        actions: resource.actions,
        panels: resource.panels,
      },
    ]),
  ),
  codegenBuckets: Object.fromEntries(
    uniqueSorted(normalizedResources.map((resource) => resource.codegen?.mode ?? "unknown")).map((mode) => [
      mode,
      normalizedResources
        .filter((resource) => (resource.codegen?.mode ?? "unknown") === mode)
        .map((resource) => resource.name)
        .sort(),
    ]),
  ),
  resources: normalizedResources,
};

const output = `${JSON.stringify(contract, null, 2)}\n`;
if (process.argv.includes("--stdout")) {
  process.stdout.write(output);
} else {
  fs.mkdirSync(generatedDir, { recursive: true });
  fs.writeFileSync(generatedPath, output);
  console.log(`Generated ${path.relative(repoRoot, generatedPath)}`);
}
