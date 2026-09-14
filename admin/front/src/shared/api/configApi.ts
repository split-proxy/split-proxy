import { apiFetch } from "@/shared/api/apiClient";

export type AppConfig = {
  default_group_name: string;
};

export const getConfig = (): Promise<AppConfig> =>
  apiFetch("/config", {
    method: "GET",
  });
