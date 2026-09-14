import { apiFetch } from '@/shared/api/apiClient'
import { authStorage } from '@/shared/lib/authStorage'
import { WorkerItem, WorkerInput } from '@/shared/types'

type PaginatedWorkerList = {
  count: number;
  next: string | null;
  previous: string | null;
  results: WorkerItem[];
};

export const getWorker = (guid: string): Promise<WorkerItem> => apiFetch(`/worker/${guid}`, {method: "GET"});

export const getWorkers = (): Promise<PaginatedWorkerList> => apiFetch(`/worker`, {method: "GET"});

export const updateWorker = (guid: string, data: WorkerInput): Promise<WorkerItem> => apiFetch(`/worker/${guid}`, {
  method: "PATCH",
  body: JSON.stringify(data),
})

export const removeWorker = (guid: string): Promise<WorkerItem> => apiFetch(`/worker/${guid}`, {method: 'DELETE'})
