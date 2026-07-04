const API_BASE = import.meta.env.VITE_PLATFORM_API_BASE ?? "/api";

export type PlatformResponse<T> = {
  data?: T;
  error?: {
    code: string;
    message: string;
  };
};

export type CapabilityItem = {
  id: string;
  name: string;
  version: string;
};

export type AdminResourceRecord = {
  id: string;
  code: string;
  name: string;
  status: string;
  description?: string;
  updatedAt: string;
  values?: Record<string, string>;
};

export type AdminResourceInput = {
  code?: string;
  name: string;
  status?: string;
  description?: string;
  values?: Record<string, string>;
};

export type AdminResourceList = {
  resource: string;
  items: AdminResourceRecord[];
};

export type AdminResourceMutation = {
  resource: string;
  record: AdminResourceRecord;
};

export async function request<T>(path: `/${string}`, init?: RequestInit): Promise<T> {
  const response = await fetch(`${API_BASE}${path}`, {
    ...init,
    headers: {
      ...(init?.body ? { "Content-Type": "application/json" } : {}),
      ...init?.headers,
    },
  });
  const payload = (await response.json()) as PlatformResponse<T>;
  if (!response.ok || payload.error) {
    throw new Error(payload.error?.message ?? `HTTP ${response.status}`);
  }
  return payload.data as T;
}

export function listCapabilities() {
  return request<CapabilityItem[]>("/capabilities");
}

export function listAdminResource(resource: string) {
  return request<AdminResourceList>(`/admin/resources/${resource}` as `/${string}`);
}

export function createAdminResource(resource: string, input: AdminResourceInput) {
  return request<AdminResourceMutation>(`/admin/resources/${resource}` as `/${string}`, {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export function updateAdminResource(resource: string, id: string, input: AdminResourceInput) {
  return request<AdminResourceMutation>(`/admin/resources/${resource}/${id}` as `/${string}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export function deleteAdminResource(resource: string, id: string) {
  return request<{ deleted: boolean; resource: string }>(`/admin/resources/${resource}/${id}` as `/${string}`, {
    method: "DELETE",
  });
}
