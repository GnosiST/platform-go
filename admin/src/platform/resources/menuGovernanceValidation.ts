const FORBIDDEN_PARAMETER_WORD = /\b(?:select|insert|update|delete|drop|alter|create|truncate|merge|exec|execute|union|datasource|shard|database|schema|sql|script|expression)\b/i;

const FORBIDDEN_PARAMETER_MARKERS = [
  "<script",
  "</script",
  "javascript:",
  "vbscript:",
  "data:text/html",
  "eval(",
  "function(",
  "${",
  "#{",
  "@{",
  "{{",
  "}}",
  "/:",
  "/{",
  "/*",
] as const;

const FORBIDDEN_PARAMETER_KEYS = new Set([
  "datasource",
  "shard",
  "database",
  "schema",
  "sql",
  "script",
  "expression",
  "route-template",
  "physical-database",
  "physical-schema",
  "physical-routing",
]);

export function isForbiddenMenuParameterKey(value: string) {
  return FORBIDDEN_PARAMETER_KEYS.has(value.trim().toLocaleLowerCase());
}

export function isForbiddenMenuParameterStringValue(value: string) {
  const normalized = value.trim().toLocaleLowerCase();
  return FORBIDDEN_PARAMETER_WORD.test(normalized) || FORBIDDEN_PARAMETER_MARKERS.some((marker) => normalized.includes(marker));
}

export function isSafeInternalMenuRoute(route: string) {
  return route.startsWith("/") && !route.startsWith("//") && !/[?#{}:*]/.test(route);
}
