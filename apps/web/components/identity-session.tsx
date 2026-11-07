"use client";

import { createClient, type SupabaseClient } from "@supabase/supabase-js";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { normalizeTotpCode } from "@/lib/account-security";
import { useActionLock } from "@/lib/use-action-lock";
import { useState, type FormEvent, type ReactNode } from "react";

const api = process.env.NEXT_PUBLIC_IDENTITY_API_URL;
const issuer = process.env.NEXT_PUBLIC_IDENTITY_AUTH_URL;
const key = process.env.NEXT_PUBLIC_IDENTITY_AUTH_PUBLIC_KEY;

export async function identityFetch<T>(path: string, token: string, method = "GET", body?: object): Promise<T> {
  if (!api) throw new Error("専用画面は準備中です");
  const res = await fetch(new URL(path, api), {
    method, headers: { Authorization: `Bearer ${token}`, "Content-Type": "application/json" },
    body: body && JSON.stringify(body), cache: "no-store", credentials: "omit", redirect: "error",
  });
  if (!res.ok) {
    if (res.status === 401) throw new Error("専用画面にログインし直してください");
    if (res.status === 403) throw new Error("この操作を行う権限がありません");
    if (res.status === 404) throw new Error("対象の登録情報が見つかりません");
    if (res.status === 409 || res.status === 422) throw new Error("入力内容と公開条件を確かめてください");
    throw new Error("専用画面に接続できません。時間をおいてもう一度お試しください");
  }
  return res.status === 204 ? undefined as T : await res.json() as T;
}

export function IdentitySession({ children }: { children: (token: string) => ReactNode }) {
  const [client] = useState<SupabaseClient | null>(() => {
    if (!api || !issuer || !key) return null;
    try {
      const vaultAPI = new URL(api);
      const vaultIssuer = new URL(issuer);
      const mainAPI = new URL(process.env.NEXT_PUBLIC_API_BASE_URL ?? "http://localhost:8080");
      const mainIssuer = process.env.NEXT_PUBLIC_SUPABASE_URL;
      if (vaultAPI.protocol !== "https:" || vaultIssuer.protocol !== "https:" ||
          vaultAPI.origin === mainAPI.origin || (mainIssuer && vaultIssuer.origin === new URL(mainIssuer).origin)) return null;
      return createClient(issuer, key, { auth: { persistSession: false, autoRefreshToken: false, detectSessionInUrl: false } });
    } catch {
      return null;
    }
  });
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [factor, setFactor] = useState<{ id: string; setupKey?: string }>();
  const [code, setCode] = useState("");
  const [token, setToken] = useState("");
  const [error, setError] = useState("");
  const { pending, acquire, release } = useActionLock();

  async function login(event: FormEvent) {
    event.preventDefault();
    if (!client || !acquire()) return;
    setError("");
    try {
      const login = await client.auth.signInWithPassword({ email, password });
      if (login.error) throw login.error;
      const factors = await client.auth.mfa.listFactors();
      if (factors.error) throw factors.error;
      const totp = factors.data.totp[0];
      if (totp) {
        setFactor({ id: totp.id });
        return;
      }
      const enrollment = await client.auth.mfa.enroll({ factorType: "totp", friendlyName: `TrackTalk ${crypto.randomUUID().slice(0, 8)}` });
      if (enrollment.error) throw enrollment.error;
      setFactor({ id: enrollment.data.id, setupKey: enrollment.data.totp.secret });
    } catch {
      setError("専用アカウントでログインできませんでした");
    } finally {
      setPassword("");
      release();
    }
  }

  async function verify(event: FormEvent) {
    event.preventDefault();
    if (!client || !factor || !acquire()) return;
    setError("");
    try {
      const verified = await client.auth.mfa.challengeAndVerify({ factorId: factor.id, code: normalizeTotpCode(code) });
      if (verified.error) throw verified.error;
      setToken(verified.data.access_token);
      setFactor(undefined);
    } catch {
      setError("認証アプリのコードを確かめてください");
    } finally {
      setCode("");
      release();
    }
  }

  function signOut() {
    setToken("");
    setFactor(undefined);
    setError("");
    void client?.auth.signOut({ scope: "local" });
  }

  if (!client) return <p>身元情報の専用画面は準備中です。</p>;
  if (token) return <>{children(token)}<Button type="button" variant="secondary" onClick={signOut}>専用画面からログアウト</Button></>;
  if (factor) return (
    <form method="post" className="space-y-2" onSubmit={verify}>
      {factor.setupKey && <p>認証アプリに次の設定キーを登録してください。<code className="block break-all">{factor.setupKey}</code></p>}
      <label className="block">認証アプリのコード<Input autoComplete="one-time-code" inputMode="numeric" value={code} onChange={(e) => setCode(e.target.value)} required /></label>
      <Button disabled={pending}>認証する</Button>
      <Button type="button" variant="secondary" onClick={signOut}>最初に戻る</Button>
      {error && <p role="alert">{error}</p>}
    </form>
  );
  return (
    <form method="post" className="space-y-2" onSubmit={login}>
      <p>運営から案内された身元情報専用アカウントでログインしてください。</p>
      <label className="block">メールアドレス<Input type="email" autoComplete="username" value={email} onChange={(e) => setEmail(e.target.value)} required /></label>
      <label className="block">パスワード<Input type="password" autoComplete="current-password" value={password} onChange={(e) => setPassword(e.target.value)} required /></label>
      <Button disabled={pending}>専用画面にログイン</Button>
      {error && <p role="alert">{error}</p>}
    </form>
  );
}
