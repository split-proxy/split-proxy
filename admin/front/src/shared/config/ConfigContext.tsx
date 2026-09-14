import {
  createContext,
  useContext,
  useEffect,
  useState,
  type ReactNode,
} from "react";

import {
  getConfig,
  type AppConfig,
} from "@/shared/api/configApi";

type ConfigContextValue = {
  config: AppConfig | null;
  loading: boolean;
};

const ConfigContext =
  createContext<ConfigContextValue | undefined>(
    undefined,
  );

export function ConfigProvider({
  children,
}: {
  children: ReactNode;
}) {
  const [config, setConfig] =
    useState<AppConfig | null>(null);

  const [loading, setLoading] =
    useState(true);

  useEffect(() => {
    getConfig()
      .then(setConfig)
      .finally(() => {
        setLoading(false);
      });
  }, []);

  return (
    <ConfigContext.Provider
      value={{
        config,
        loading,
      }}
    >
      {children}
    </ConfigContext.Provider>
  );
}

export function useConfig() {
  const context =
    useContext(ConfigContext);

  if (!context) {
    throw new Error(
      "useConfig must be used inside ConfigProvider",
    );
  }

  return context;
}
