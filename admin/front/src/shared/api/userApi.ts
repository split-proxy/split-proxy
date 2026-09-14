import { apiFetch } from '@/shared/api/apiClient'
import { authStorage } from '@/shared/lib/authStorage'
import { User, UserInput } from '@/shared/types'

type PaginatedProfileList = {
  count: number;
  next: string | null;
  previous: string | null;
  results: User[];
};


export const getUser = (id: number): Promise<User> => apiFetch(`/profile/${id}`, {method: "GET"});

export const getUsers = (): Promise<PaginatedProfileList> => apiFetch(`/profile`, {method: "GET"})

export const createUser = (data: UserInput): Promise<User> =>  apiFetch(
  `/profile`,
  {
    method: 'POST',
    body: JSON.stringify(data),
  }
)

export const updateUser = (id: number, data: UserInput): Promise<User> =>  apiFetch(
  `/profile/${id}`,
  {
    method: 'PATCH',
    body: JSON.stringify(data),
  }
)


export const removeUser = (id: number): Promise<User> => apiFetch(`/profile/${id}`, {method: 'DELETE'})
