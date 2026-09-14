import { apiFetch } from '@/shared/api/apiClient'
import { authStorage } from '@/shared/lib/authStorage'
import { GroupItem, GroupInput } from '@/shared/types'

type PaginatedGroupList = {
  count: number;
  next: string | null;
  previous: string | null;
  results: GroupItem[];
};

export const getGroup = (id: number): Promise<GroupItem> => apiFetch(`/worker_group/${id}`, {method: "GET"});

export const getGroups = (): Promise<PaginatedGroupList> => apiFetch(`/worker_group`, {method: "GET"});

export const createGroup = (data: GroupInput): Promise<GroupItem> => apiFetch(`/worker_group`, {
  method: "POST",
  body: JSON.stringify(data),
})

export const updateGroup = (id: number, data: GroupInput): Promise<GroupItem> => apiFetch(`/worker_group/${id}`, {
  method: "PATCH",
  body: JSON.stringify(data),
})

export const removeGroup = (id: number): Promise<GroupItem> => apiFetch(`/worker_group/${id}`, {method: 'DELETE'})
