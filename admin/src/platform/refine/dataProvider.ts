import type {
  BaseKey,
  BaseRecord,
  CreateParams,
  CrudFilter,
  CrudSort,
  DataProvider,
  DeleteOneParams,
  GetListParams,
  GetOneParams,
  UpdateParams,
} from "@refinedev/core";
import {
  createAdminResource,
  deleteAdminResource,
  queryAdminResource,
  updateAdminResource,
  type AdminResourceInput,
  type AdminResourceQueryCondition,
  type AdminResourceQuerySort,
  type AdminResourceRecord,
} from "../api/client";

type AdminResourceListMeta = {
  keywords?: unknown;
  conditions?: unknown;
};

export const dataProvider: DataProvider = {
  getList: async <TData extends BaseRecord = BaseRecord>(params: GetListParams) => {
    const meta = params.meta as AdminResourceListMeta | undefined;
    const metaKeywords = normalizeKeywords(meta?.keywords);
    const metaConditions = normalizeConditions(meta?.conditions);
    const result = await queryAdminResource(params.resource, {
      keywords: metaKeywords,
      conditions: [
        ...filtersToConditions(params.filters ?? []),
        ...metaConditions,
      ],
      sort: sortersToQuerySort(params.sorters ?? []),
      page: params.pagination?.currentPage ?? 1,
      pageSize: params.pagination?.pageSize ?? 10,
    });
    return {
      data: result.items.map((record) => toRefineRecord(record)) as TData[],
      total: result.total,
    };
  },
  getOne: async <TData extends BaseRecord = BaseRecord>({ resource, id }: GetOneParams) => {
    const result = await queryAdminResource(resource, {
      conditions: [{ field: "id", operator: "=", value: stringifyID(id) }],
      page: 1,
      pageSize: 1,
    });
    const record = result.items[0];
    if (!record) {
      throw new Error(`Resource record not found: ${resource}/${String(id)}`);
    }
    return { data: toRefineRecord(record) as TData };
  },
  create: async <TData extends BaseRecord = BaseRecord, TVariables = Record<string, unknown>>({
    resource,
    variables,
  }: CreateParams<TVariables>) => {
    const result = await createAdminResource(resource, toAdminResourceInput(variables));
    return { data: toRefineRecord(result.record, result.token) as TData };
  },
  update: async <TData extends BaseRecord = BaseRecord, TVariables = Record<string, unknown>>({
    resource,
    id,
    variables,
  }: UpdateParams<TVariables>) => {
    const result = await updateAdminResource(resource, stringifyID(id), toAdminResourceInput(variables));
    return { data: toRefineRecord(result.record, result.token) as TData };
  },
  deleteOne: async <TData extends BaseRecord = BaseRecord, TVariables = Record<string, unknown>>({
    resource,
    id,
  }: DeleteOneParams<TVariables>) => {
    await deleteAdminResource(resource, stringifyID(id));
    return { data: { id } as TData };
  },
  getApiUrl: () => "",
};

function toRefineRecord(record: AdminResourceRecord, issuedToken?: string): BaseRecord {
  return {
    ...record.values,
    ...record,
    values: record.values ?? {},
    ...(issuedToken ? { __platformIssuedToken: issuedToken } : {}),
  };
}

function toAdminResourceInput(variables: unknown): AdminResourceInput {
  const source = isObjectRecord(variables) ? variables : {};
  const input: AdminResourceInput = {
    code: stringValue(source.code),
    name: stringValue(source.name) ?? "",
    status: stringValue(source.status),
    description: stringValue(source.description),
    values: isStringMap(source.values) ? source.values : {},
  };

  for (const [key, value] of Object.entries(source)) {
    if (key === "id" || key === "code" || key === "name" || key === "status" || key === "description" || key === "values") {
      continue;
    }
    if (typeof value === "string") {
      input.values = { ...(input.values ?? {}), [key]: value };
    }
  }

  if (!input.name) {
    input.name = input.code || "Untitled";
  }
  return input;
}

function filtersToConditions(filters: CrudFilter[]): AdminResourceQueryCondition[] {
  return filters.flatMap((filter) => {
    if (!("field" in filter) || filter.value === undefined || filter.value === null || filter.value === "") {
      return [];
    }
    const value = Array.isArray(filter.value) ? filter.value.join(",") : String(filter.value);
    return [{ field: filter.field, operator: refineOperatorToQueryOperator(filter.operator), value }];
  });
}

function normalizeKeywords(value: unknown): string[] | undefined {
  if (!Array.isArray(value)) {
    return undefined;
  }
  const keywords = value.filter((item): item is string => typeof item === "string" && item.trim() !== "");
  return keywords.length > 0 ? keywords : undefined;
}

function normalizeConditions(value: unknown): AdminResourceQueryCondition[] {
  if (!Array.isArray(value)) {
    return [];
  }
  return value.filter(isAdminResourceQueryCondition);
}

function isAdminResourceQueryCondition(value: unknown): value is AdminResourceQueryCondition {
  if (!isObjectRecord(value)) {
    return false;
  }
  return (
    typeof value.field === "string" &&
    isAdminResourceQueryOperator(value.operator) &&
    typeof value.value === "string"
  );
}

function isAdminResourceQueryOperator(value: unknown): value is AdminResourceQueryCondition["operator"] {
  return value === "contains" || value === "=" || value === "!=" || value === ">" || value === ">=" || value === "<" || value === "<=";
}

function refineOperatorToQueryOperator(operator: string): AdminResourceQueryCondition["operator"] {
  switch (operator) {
  case "eq":
    return "=";
  case "ne":
    return "!=";
  case "gt":
    return ">";
  case "gte":
    return ">=";
  case "lt":
    return "<";
  case "lte":
    return "<=";
  default:
    return "contains";
  }
}

function sortersToQuerySort(sorters: CrudSort[]): AdminResourceQuerySort[] {
  return sorters.map((sorter) => ({
    field: sorter.field,
    order: sorter.order === "desc" ? "desc" : "asc",
  }));
}

function stringifyID(id: BaseKey) {
  return String(id).trim();
}

function isObjectRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function isStringMap(value: unknown): value is Record<string, string> {
  return isObjectRecord(value) && Object.values(value).every((item) => typeof item === "string");
}

function stringValue(value: unknown) {
  return typeof value === "string" ? value : undefined;
}
