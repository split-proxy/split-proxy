import { apiFetch } from '@/shared/api/apiClient'
import { authStorage } from '@/shared/lib/authStorage'
import { ProxyItem, ProxyInput } from '@/shared/types'

type PaginatedProxyList = {
  count: number;
  next: string | null;
  previous: string | null;
  results: ProxyItem[];
};

export const getProxy = (id: number): Promise<ProxyItem> => apiFetch(`/proxy/${id}`, {method: "GET"});

export const getProxies = (): Promise<PaginatedProxyList> => apiFetch(`/proxy`, {method: "GET"});

export const createProxy = (data: ProxyInput): Promise<ProxyItem> => apiFetch(`/proxy`, {
  method: "POST",
  body: JSON.stringify(data),
})

export const updateProxy = (id: number, data: ProxyInput): Promise<ProxyItem> => apiFetch(`/proxy/${id}`, {
  method: "PATCH",
  body: JSON.stringify(data),
})

export const removeProxy = (id: number): Promise<ProxyItem> => apiFetch(`/proxy/${id}`, {method: 'DELETE'})
