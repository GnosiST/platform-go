const API_BASE = import.meta.env.VITE_PLATFORM_API_BASE ?? "http://127.0.0.1:9200/api";

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

export async function request<T>(path: `/${string}`): Promise<T> {
  const response = await fetch(`${API_BASE}${path}`);
  const payload = (await response.json()) as PlatformResponse<T>;
  if (!response.ok || payload.error) {
    throw new Error(payload.error?.message ?? `HTTP ${response.status}`);
  }
  return payload.data as T;
}

export function listCapabilities() {
  return request<CapabilityItem[]>("/capabilities");
}
