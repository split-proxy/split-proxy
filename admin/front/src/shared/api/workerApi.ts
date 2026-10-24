import { apiFetch } from '@/shared/api/apiClient'
import { authStorage } from '@/shared/lib/authStorage'
import { WorkerItem, WorkerInput } from '@/shared/types'

type PaginatedWorkerList = {
  count: number;
  next: string | null;
  previous: string | null;
  results: WorkerItem[];
};

export interface WorkerFilters {
  guid?: string;
  group?: number | "none";
  page?: number;
  page_size?: number;
}

export const getWorkers = (
  params?: WorkerFilters,
): Promise<PaginatedWorkerList> => {
  const query = new URLSearchParams();

  if (params?.guid) {
    query.set("guid", params.guid);
  }

  if (params?.group !== undefined) {
    query.set("group", String(params.group));
  }

  if (params?.page !== undefined) {
    query.set("page", String(params.page));
  }

  if (params?.page_size !== undefined) {
    query.set("page_size", String(params.page_size));
  }

  const queryString = query.toString();
  const url = queryString ? `/worker?${queryString}` : "/worker";

  return apiFetch(url, { method: "GET" });
};

export const getWorker = (guid: string): Promise<WorkerItem> => apiFetch(`/worker/${guid}`, {method: "GET"});

export const updateWorker = (
  guid: string,
  data: { group?: number | null },
) =>
  apiFetch(`/worker/${encodeURIComponent(guid)}`, {
    method: "PATCH",
    body: JSON.stringify(data),
  });

export const removeWorker = (guid: string): Promise<WorkerItem> => apiFetch(`/worker/${guid}`, {method: 'DELETE'})
