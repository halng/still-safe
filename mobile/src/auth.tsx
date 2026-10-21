import { createContext, ReactNode, useCallback, useContext, useEffect, useMemo, useState } from "react";
import { api } from "./api";
import { clearToken, getToken, setToken } from "./session";
import type { Me } from "./types";

type AuthValue = {
  ready: boolean;
  token: string | null;
  me: Me | null;
  signIn: (token: string) => Promise<void>;
  refresh: () => Promise<void>;
  signOut: () => Promise<void>;
};

const AuthContext = createContext<AuthValue | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [ready, setReady] = useState(false);
  const [token, setTokenState] = useState<string | null>(null);
  const [me, setMe] = useState<Me | null>(null);

  const refresh = useCallback(async () => {
    const current = token ?? (await getToken());
    if (!current) return;
    const next = await api.me(current);
    setMe(next);
  }, [token]);

  const signIn = useCallback(async (nextToken: string) => {
    await setToken(nextToken);
    const next = await api.me(nextToken);
    setTokenState(nextToken);
    setMe(next);
  }, []);

  const signOut = useCallback(async () => {
    const current = token;
    setTokenState(null);
    setMe(null);
    await clearToken();
    if (current) {
      try {
        await api.logout(current);
      } catch {
        // The local session is already gone.
      }
    }
  }, [token]);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      const stored = await getToken();
      if (!stored) {
        if (!cancelled) setReady(true);
        return;
      }
      try {
        const next = await api.me(stored);
        if (cancelled) return;
        setTokenState(stored);
        setMe(next);
      } catch {
        await clearToken();
      } finally {
        if (!cancelled) setReady(true);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  const value = useMemo(
    () => ({ ready, token, me, signIn, refresh, signOut }),
    [ready, token, me, signIn, refresh, signOut],
  );
  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthValue {
  const value = useContext(AuthContext);
  if (!value) throw new Error("AuthProvider is missing");
  return value;
}
