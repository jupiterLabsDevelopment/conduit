import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from "react";

import { ApiClient, AuthUser, LoginResponse, apiClient } from "../lib/api";

interface AuthState {
  user: AuthUser | null;
  token: string | null;
  loading: boolean;
  api: ApiClient;
  login: (email: string, password: string) => Promise<void>;
  bootstrap: (email: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
}

const STORAGE_KEY = "conduit.auth";

const AuthContext = createContext<AuthState | undefined>(undefined);

type AuthProviderProps = {
  children: ReactNode;
};

export const AuthProvider = ({ children }: AuthProviderProps) => {
  const [user, setUser] = useState<AuthUser | null>(null);
  const [token, setToken] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    apiClient.setToken(token);
  }, [token]);

  useEffect(() => {
    const storedRaw = localStorage.getItem(STORAGE_KEY);
    if (!storedRaw) {
      setLoading(false);
      return;
    }
    try {
      const stored = JSON.parse(storedRaw) as LoginResponse;
      setUser(stored.user);
      setToken(stored.token);
    } catch (err) {
      console.warn("Failed to parse stored auth state", err);
      localStorage.removeItem(STORAGE_KEY);
    } finally {
      setLoading(false);
    }
  }, []);

  const persist = useCallback((data: LoginResponse | null) => {
    if (!data) {
      localStorage.removeItem(STORAGE_KEY);
      return;
    }
    localStorage.setItem(STORAGE_KEY, JSON.stringify(data));
  }, []);

  const login = useCallback(async (email: string, password: string) => {
    const res = await apiClient.login(email, password);
    setUser(res.user);
    setToken(res.token);
    persist(res);
  }, [persist]);

  const bootstrap = useCallback(async (email: string, password: string) => {
    await apiClient.bootstrap(email, password);
  }, []);

  const logout = useCallback(async () => {
    try {
      await apiClient.logout();
    } catch (err) {
      console.warn("Failed to revoke session during logout", err);
    } finally {
      setUser(null);
      setToken(null);
      persist(null);
    }
  }, [persist]);

  const value = useMemo<AuthState>(() => ({
    user,
    token,
    loading,
    api: apiClient,
    login,
    bootstrap,
    logout,
  }), [user, token, loading, login, bootstrap, logout]);

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
};

export const useAuth = (): AuthState => {
  const ctx = useContext(AuthContext);
  if (!ctx) {
    throw new Error("useAuth must be used within AuthProvider");
  }
  return ctx;
};
