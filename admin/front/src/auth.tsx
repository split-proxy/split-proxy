import { createContext, useContext, useMemo, useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";

const TOKEN_KEY = "admin_access_token";
const USER_KEY = "admin_current_user";

type SessionUser = {
  id: string;
  name: string;
  email: string;
  username: string;
};

type AuthContextValue = {
  token: string | null;
  user: SessionUser | null;
  isAuthenticated: boolean;
  login: (email: string, password: string) => Promise<void>;
  logout: () => void;
};

type GetTokenResponse = {
  token: string;
};

type CurrentUserResponse = {
  username: string;
};

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const { t } = useTranslation();

  const [token, setToken] = useState<string | null>(() =>
    localStorage.getItem(TOKEN_KEY)
  );

  const [user, setUser] = useState<SessionUser | null>(() => {
    const raw = localStorage.getItem(USER_KEY);
    return raw ? JSON.parse(raw) : null;
  });

  const login = async (email: string, password: string) => {
    // 1. Получаем token
    const tokenResponse = await fetch("/api/v1/get_token", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Accept: "application/json",
      },
      body: JSON.stringify({
        username: email,
        password,
      }),
    });

    if (!tokenResponse.ok) {
      throw new Error(t("auth.invalidCredentials"));
    }

    const tokenData: GetTokenResponse = await tokenResponse.json();

    // 2. Используем полученный token для получения текущего пользователя
    const currentUserResponse = await fetch("/api/v1/current_profile", {
      method: "GET",
      headers: {
        Accept: "application/json",
        Authorization: `Token ${tokenData.token}`,
      },
    });

    if (!currentUserResponse.ok) {
      throw new Error(t("auth.invalidCredentials"));
    }

    const currentUser: CurrentUserResponse =
      await currentUserResponse.json();

    // 3. Формируем пользователя приложения
    const nextUser: SessionUser = {
      id: currentUser.username,
      name: currentUser.username,
      email,
      username: currentUser.username,
    };

    // 4. Сохраняем token и пользователя
    localStorage.setItem(TOKEN_KEY, tokenData.token);
    localStorage.setItem(USER_KEY, JSON.stringify(nextUser));

    setToken(tokenData.token);
    setUser(nextUser);
  };

  const logout = () => {
    localStorage.removeItem(TOKEN_KEY);
    localStorage.removeItem(USER_KEY);

    setToken(null);
    setUser(null);
  };

  const value = useMemo(
    () => ({
      token,
      user,
      isAuthenticated: Boolean(token),
      login,
      logout,
    }),
    [token, user]
  );

  return (
    <AuthContext.Provider value={value}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  const value = useContext(AuthContext);

  if (!value) {
    throw new Error("useAuth must be used inside AuthProvider");
  }

  return value;
}
