"use client";

import type { Session } from "@supabase/supabase-js";
import { createContext, useCallback, useContext, useEffect, useRef, useState } from "react";
import type { Profile } from "@/lib/types";
import { apiFetch } from "@/lib/api";
import { clearCompetitions } from "@/lib/competition-list";
import { clearDomesticMajors } from "@/lib/domestic-majors";
import { clearMajorHosts } from "@/lib/major-hosts";
import { clearSavedFeeds } from "@/lib/home-feed-cache";
import { refreshPushSubscription } from "@/lib/push";
import { clearPersonalStorage } from "@/lib/sign-out";

type AuthState = {
  session: Session | null;
  profile: Profile | null;
  profileLoading: boolean;
  loading: boolean;
  sessionLoading: boolean;
  token: string | null;
  refreshProfile: () => Promise<void>;
  // プロフィールが未取得のまま止まっているときだけ取り直す(取得済み・取得中なら何もしない)。
  ensureProfile: () => void;
  pushStatus: "checking" | "on" | "off" | "failed";
  setPushEnabled: (enabled: boolean) => void;
};

// /v1/me が返らないまま止まったら打ち切る(復帰直後の端末では、通信が戻らず待ち続けることがある)。
const PROFILE_TIMEOUT_MS = 15_000;
// 一時的な失敗のあとの取り直しの間隔。使い切ったら、アプリの復帰・通信の回復・マイページの表示で取り直す。
const PROFILE_RETRY_DELAYS_MS = [1_000, 3_000, 10_000];

function isTransientProfileError(err: unknown) {
  const status = (err as { status?: unknown } | null)?.status;
  if (typeof status !== "number") return true;
  return status === 0 || status === 408 || status === 429 || status >= 500;
}

const AuthContext = createContext<AuthState>({
  session: null,
  profile: null,
  profileLoading: false,
  loading: true,
  sessionLoading: true,
  token: null,
  refreshProfile: async () => {},
  ensureProfile: () => {},
  pushStatus: "checking",
  setPushEnabled: () => {},
});

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [session, setSession] = useState<Session | null>(null);
  const [profile, setProfile] = useState<Profile | null>(null);
  const [profileLoading, setProfileLoading] = useState(false);
  const [loading, setLoading] = useState(Boolean(
    process.env.NEXT_PUBLIC_SUPABASE_URL && process.env.NEXT_PUBLIC_SUPABASE_ANON_KEY,
  ));

  const [sessionLoading, setSessionLoading] = useState(loading);
  const [pushStatus, setPushStatus] = useState<AuthState["pushStatus"]>("checking");

  // 読み込みが重なったら(ログアウト直後に前のトークンの応答が遅れて届くなど)、最後に始めたものの結果だけを使う。
  const profileSeq = useRef(0);
  const profileRef = useRef<Profile | null>(null);
  const tokenRef = useRef<string | null>(null);
  const userIdRef = useRef<string | null>(null);
  const profileInFlight = useRef(false);
  // 一時的な失敗で取り直しを使い切り、プロフィールが無いまま止まっている。
  const profileStalled = useRef(false);
  const retryTimer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  function applyProfile(next: Profile | null) {
    profileRef.current = next;
    setProfile(next);
  }
  // 一時的な失敗(通信できない・応答が返らない・サーバー側の失敗)のあとは、間をあけて取り直す。
  // 失敗のたびにプロフィールを捨てると、アプリの再起動(認証イベントの再発生)まで管理画面の入口が出なくなる(#504)。
  // ref と state の更新関数だけを使うので、描画をまたいで同じ関数でよい。
  const loadProfile = useCallback(async function load(token: string | null, userId: string | null = userIdRef.current, attempt = 0): Promise<void> {
    const seq = ++profileSeq.current;
    clearTimeout(retryTimer.current);
    profileStalled.current = false;
    tokenRef.current = token;
    userIdRef.current = token ? userId : null;
    if (!token) {
      profileInFlight.current = false;
      applyProfile(null);
      setProfileLoading(false);
      return;
    }
    // 別のアカウントの取得済みプロフィールは持ち越さない。
    if (profileRef.current && userId && profileRef.current.id !== userId) applyProfile(null);
    profileInFlight.current = true;
    setProfileLoading(true);
    const controller = new AbortController();
    const timeout = setTimeout(() => controller.abort(), PROFILE_TIMEOUT_MS);
    try {
      const me = await apiFetch<Profile>("/v1/me", { token, signal: controller.signal });
      if (seq !== profileSeq.current) return;
      applyProfile(me);
    } catch (err) {
      if (seq !== profileSeq.current) return;
      if (!isTransientProfileError(err)) {
        applyProfile(null);
      } else if (attempt < PROFILE_RETRY_DELAYS_MS.length) {
        // 取り直しの間は確認中のまま。同じアカウントの取得済みプロフィールは消さない。
        retryTimer.current = setTimeout(() => {
          if (seq === profileSeq.current) void load(token, userId, attempt + 1);
        }, PROFILE_RETRY_DELAYS_MS[attempt]);
        return;
      } else {
        profileStalled.current = true;
      }
    } finally {
      clearTimeout(timeout);
    }
    profileInFlight.current = false;
    setProfileLoading(false);
  }, []);
  // 一時的な失敗で取り直しを使い切ったあと、プロフィールが無いまま止まっているときだけ取り直す。
  // 認証エラーなどは認証イベント(トークンの更新・再ログイン)で取り直すので、ここでは繰り返さない。
  const ensureProfile = useCallback(() => {
    const token = tokenRef.current;
    if (!token || profileInFlight.current || !profileStalled.current) return;
    void loadProfile(token, userIdRef.current);
  }, [loadProfile]);

  // アプリの復帰(バックグラウンドからの復帰・bfcache の復元・通信の回復)で、未取得のプロフィールを取り直す。
  useEffect(() => {
    const onVisible = () => { if (document.visibilityState === "visible") ensureProfile(); };
    document.addEventListener("visibilitychange", onVisible);
    window.addEventListener("pageshow", ensureProfile);
    window.addEventListener("online", ensureProfile);
    return () => {
      document.removeEventListener("visibilitychange", onVisible);
      window.removeEventListener("pageshow", ensureProfile);
      window.removeEventListener("online", ensureProfile);
    };
  }, [ensureProfile]);

  useEffect(() => {
    let active = true;
    let pushSyncTimer: ReturnType<typeof setTimeout> | undefined;
    let pushSyncSeq = 0;
    let unsubscribe: (() => void) | undefined;
    const invalidateProfile = () => { profileSeq.current++; clearTimeout(retryTimer.current); };
    // 認証 SDK のダウンロードを、タブや検索の操作準備の前提にしない。
    void import("@/lib/supabase").then(({ createClient }) => {
      if (!active) return;
      const supabase = createClient();
      if (!supabase) {
        setLoading(false);
        setSessionLoading(false);
        return;
      }
      // INITIAL_SESSION もここで受け取る。getSession と重ねてプロフィールを二重取得しない。
      const { data } = supabase.auth.onAuthStateChange((event, next) => {
        if (!active) return;
        setSession(next);
        setSessionLoading(false);
        // 通知の鍵を入れ替えたあとも届くよう、アプリを開いたときとログインしたときに購読を確かめる。
        if (!next || event === "INITIAL_SESSION" || event === "SIGNED_IN") {
          clearTimeout(pushSyncTimer);
          const seq = ++pushSyncSeq;
          const userId = next?.user.id ?? null;
          setPushStatus("checking");
          // 認証 SDK のコールバックを抜けてから、現在のセッションを読み直す。
          pushSyncTimer = setTimeout(() => {
            void refreshPushSubscription(userId).then(
              (enabled) => { if (active && seq === pushSyncSeq) setPushStatus(enabled ? "on" : "off"); },
              () => { if (active && seq === pushSyncSeq) setPushStatus("failed"); },
            );
          }, 0);
        }
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
        void loadProfile(next?.access_token ?? null, next?.user?.id ?? null).finally(() => {
          if (active) {
          setLoading(false);
          setSessionLoading(false);
        }
        });
      });
      unsubscribe = () => data.subscription.unsubscribe();
    }).catch(() => {
      if (active) {
          setLoading(false);
          setSessionLoading(false);
        }
    });
    return () => {
      active = false;
      clearTimeout(pushSyncTimer);
      invalidateProfile();
      unsubscribe?.();
    };
  }, [loadProfile]);

  return (
    <AuthContext.Provider
      value={{
        session,
        profile,
        profileLoading,
        loading,
        sessionLoading,
        pushStatus,
        setPushEnabled: (enabled) => setPushStatus(enabled ? "on" : "off"),
        token: session?.access_token ?? null,
        refreshProfile: () => loadProfile(session?.access_token ?? null, session?.user?.id ?? null),
        ensureProfile,
      }}
    >
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  return useContext(AuthContext);
}
