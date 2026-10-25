"use client";

import type { Session } from "@supabase/supabase-js";
import { createContext, useContext, useEffect, useRef, useState } from "react";
import type { Profile } from "@/lib/types";
import { apiFetch } from "@/lib/api";
import { clearCompetitions } from "@/lib/competition-list";
import { clearDomesticMajors } from "@/lib/domestic-majors";
import { clearMajorHosts } from "@/lib/major-hosts";
import { clearSavedFeeds } from "@/lib/home-feed-cache";
import { clearPersonalStorage } from "@/lib/sign-out";

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
  const [session, setSession] = useState<Session | null>(null);
  const [profile, setProfile] = useState<Profile | null>(null);
  const [loading, setLoading] = useState(Boolean(
    process.env.NEXT_PUBLIC_SUPABASE_URL && process.env.NEXT_PUBLIC_SUPABASE_ANON_KEY,
  ));

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
    let active = true;
    let unsubscribe: (() => void) | undefined;
    const invalidateProfile = () => { profileSeq.current++; };
    // 認証 SDK のダウンロードを、タブや検索の操作準備の前提にしない。
    void import("@/lib/supabase").then(({ createClient }) => {
      if (!active) return;
      const supabase = createClient();
      if (!supabase) {
        setLoading(false);
        return;
      }
      // INITIAL_SESSION もここで受け取る。getSession と重ねてプロフィールを二重取得しない。
      const { data } = supabase.auth.onAuthStateChange((_e, next) => {
        if (!active) return;
        setSession(next);
        if (!next) {
          clearCompetitions();
          clearMajorHosts();
          clearDomesticMajors();
          clearSavedFeeds();
          try {
            clearPersonalStorage(window.localStorage);
          } catch {
            // 保存領域が使えない端末では、消すものもない。
          }
        }
        void loadProfile(next?.access_token ?? null).finally(() => {
          if (active) setLoading(false);
        });
      });
      unsubscribe = () => data.subscription.unsubscribe();
    }).catch(() => {
      if (active) setLoading(false);
    });
    return () => {
      active = false;
      invalidateProfile();
      unsubscribe?.();
    };
  }, []);

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
