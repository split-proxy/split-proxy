import { authStorage } from "../lib/authStorage"
export class ApiError extends Error {
  constructor(
    public status: number,
    message?: string,
  ) {
    super(message);
  }
}

export async function apiFetch<T>(
  url: string,
  options: RequestInit = {},
): Promise<T> {
  const token = localStorage.getItem("admin_access_token");

  const response = await fetch(`/api/v1${url}`, {
    ...options,
    headers: {
      Accept: "application/json",
      ...(options.body
        ? { "Content-Type": "application/json" }
        : {}),
      ...(token
        ? { Authorization: `Token ${token}` }
        : {}),
      ...options.headers,
    },
  });

  if (response.status === 400) {
      throw new Error(await response.text());
  }

  if (response.status === 401) {
      authStorage.removeToken();
      window.location.href = "/login";
      throw new Error("UNAUTHORIZED");
  }

  if (response.status === 403) {
    throw new Error("FORBIDDEN");
  }

  if (!response.ok) {
    throw new ApiError(response.status);
  }

  if (response.status === 204) {
    return undefined as T;
  }

  return response.json();
}
