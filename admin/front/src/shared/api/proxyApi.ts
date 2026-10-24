import { apiFetch } from "@/shared/api/apiClient";
import type { ProxyItem, ProxyInput } from "@/shared/types";

export type PaginatedProxyList = {
  count: number;
  next: string | null;
  previous: string | null;
  results: ProxyItem[];
};

export type ProxyListParams = {
  page?: number;
  pageSize?: number;
  search?: string;
};

const buildQuery = ({
  page,
  pageSize,
  search,
}: ProxyListParams = {}): string => {
  const params = new URLSearchParams();

  if (page !== undefined) {
    params.set("page", String(page));
  }

  if (pageSize !== undefined) {
    params.set("page_size", String(pageSize));
  }

  if (search?.trim()) {
    params.set("search", search.trim());
  }

  const query = params.toString();

  return query ? `?${query}` : "";
};

export const getProxy = (id: number): Promise<ProxyItem> =>
  apiFetch(`/proxy/${id}`, {
    method: "GET",
  });

export const getProxies = (
  params?: ProxyListParams,
): Promise<PaginatedProxyList> =>
  apiFetch(`/proxy${buildQuery(params)}`, {
    method: "GET",
  });

export const createProxy = (
  data: ProxyInput,
): Promise<ProxyItem> =>
  apiFetch("/proxy", {
    method: "POST",
    body: JSON.stringify(data),
  });

export const updateProxy = (
  id: number,
  data: ProxyInput,
): Promise<ProxyItem> =>
  apiFetch(`/proxy/${id}`, {
    method: "PATCH",
    body: JSON.stringify(data),
  });

export const removeProxy = (id: number): Promise<ProxyItem> =>
  apiFetch(`/proxy/${id}`, {
    method: "DELETE",
  });
