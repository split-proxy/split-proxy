import { apiFetch } from "@/shared/api/apiClient";
import type { CidrRule, DomainRule } from "@/shared/types";

export type DefaultRoute = {
  worker_group: number;
};

export type PaginatedDomainList = {
  count: number;
  next: string | null;
  previous: string | null;
  results: DomainRule[];
};

export type PaginatedCidrList = {
  count: number;
  next: string | null;
  previous: string | null;
  results: CidrRule[];
};

type PaginationParams = {
  page?: number;
  pageSize?: number;
  search?: string;
};

const buildQuery = ({
  page,
  pageSize,
  search,
}: PaginationParams = {}) => {
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

export const getListDomains = (
  params?: PaginationParams,
): Promise<PaginatedDomainList> =>
  apiFetch(`/routing/domains${buildQuery(params)}`, {
    method: "GET",
  });

export const getListCidr = (
  params?: PaginationParams,
): Promise<PaginatedCidrList> =>
  apiFetch(`/routing/cidr${buildQuery(params)}`, {
    method: "GET",
  });

export const addDomains = (
  patterns: string[],
  worker_group: number,
) =>
  apiFetch("/routing/domains", {
    method: "POST",
    body: JSON.stringify({
      patterns,
      worker_group,
    }),
  });

export const addCidrs = (
  cidrs: string[],
  worker_group: number,
) =>
  apiFetch("/routing/cidr", {
    method: "POST",
    body: JSON.stringify({
      cidrs,
      worker_group,
    }),
  });

export const updateDomain = (
  pattern: string,
  worker_group: number,
) =>
  apiFetch(
    `/routing/domains/${encodeURIComponent(pattern)}`,
    {
      method: "PATCH",
      body: JSON.stringify({
        worker_group,
      }),
    },
  );

export const updateCidr = (
  cidr: string,
  worker_group: number,
) =>
  apiFetch(
    `/routing/cidr/${encodeURIComponent(cidr)}`,
    {
      method: "PATCH",
      body: JSON.stringify({
        worker_group,
      }),
    },
  );

export const deleteDomain = (pattern: string) =>
  apiFetch(
    `/routing/domains/${encodeURIComponent(pattern)}`,
    {
      method: "DELETE",
    },
  );

export const deleteCidr = (cidr: string) =>
  apiFetch(
    `/routing/cidr/${encodeURIComponent(cidr)}`,
    {
      method: "DELETE",
    },
  );

export const getDefaultRoute = (): Promise<DefaultRoute> =>
  apiFetch("/routing/default", {
    method: "GET",
  });

export const updateDefaultRoute = (
  worker_group: number,
): Promise<DefaultRoute> =>
  apiFetch("/routing/default", {
    method: "PATCH",
    body: JSON.stringify({
      worker_group,
    }),
  });
