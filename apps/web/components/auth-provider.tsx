"use client";

import { createClient } from "@/lib/supabase";
import type { Session } from "@supabase/supabase-js";
import { createContext, useContext, useEffect, useMemo, useRef, useState } from "react";
import type { Profile } from "@/lib/types";
import { apiFetch } from "@/lib/api";
import { clearCompetitions } from "@/lib/competition-list";
import { clearDomesticMajors } from "@/lib/domestic-majors";
import { clearMajorHosts } from "@/lib/major-hosts";
import { clearSavedFeeds } from "@/lib/home-feed-cache";

type AuthState = {
  session: Session | null;
  profile: Profile | null;
  loading: boolean;
  token: string | null;
  refreshProfile: () => Promise<void>;
};

const AuthContext = createContext<AuthState>({
  session: null,
  profile: null,
  loading: true,
  token: null,
  refreshProfile: async () => {},
});

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const supabase = useMemo(() => createClient(), []);
  const [session, setSession] = useState<Session | null>(null);
  const [profile, setProfile] = useState<Profile | null>(null);
  const [loading, setLoading] = useState(Boolean(supabase));

  // 読み込みが重なったら(ログアウト直後に前のトークンの応答が遅れて届くなど)、最後に始めたものの結果だけを使う。
  const profileSeq = useRef(0);
  async function loadProfile(token: string | null) {
    const seq = ++profileSeq.current;
    if (!token) {
      setProfile(null);
      return;
    }
    try {
      const me = await apiFetch<Profile>("/v1/me", { token });
      if (seq === profileSeq.current) setProfile(me);
    } catch {
      if (seq === profileSeq.current) setProfile(null);
    }
  }

  useEffect(() => {
    if (!supabase) {
      return;
    }
    supabase.auth.getSession().then(({ data }) => {
      setSession(data.session);
      loadProfile(data.session?.access_token ?? null).finally(() => setLoading(false));
    });
    const { data } = supabase.auth.onAuthStateChange((_e, next) => {
      setSession(next);
      if (!next) {
        clearCompetitions();
        clearMajorHosts();
        clearDomesticMajors();
        clearSavedFeeds();
      }
      loadProfile(next?.access_token ?? null);
    });
    return () => data.subscription.unsubscribe();
  }, [supabase]);

  return (
    <AuthContext.Provider
      value={{
        session,
        profile,
        loading,
        token: session?.access_token ?? null,
        refreshProfile: () => loadProfile(session?.access_token ?? null),
      }}
    >
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  return useContext(AuthContext);
}
