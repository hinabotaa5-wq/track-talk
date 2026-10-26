"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useEffect, useState, type ReactNode } from "react";
import { ShieldCheck } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { isTotpCode, mfaErrorMessage, mfaGateState, normalizeTotpCode, type AssuranceLevel } from "@/lib/account-security";
import { useActionLock } from "@/lib/use-action-lock";
import { useAuth } from "./auth-provider";

type Levels = { current: AssuranceLevel; next: AssuranceLevel; token: string | null };

// 二段階認証を設定した人には、ログインのたびに認証アプリのコードを求める(パスワードでも Google でも)。
// 管理者・承認済みコーチで未設定の人には、管理・コーチの画面を開く前に設定を求める(API も拒む)。
export function AccountMfaGate({ children }: { children: ReactNode }) {
  const { session, profile } = useAuth();
  const pathname = usePathname();
  const [levels, setLevels] = useState<Levels | null>(null);
  const token = session?.access_token ?? null;

  useEffect(() => {
    if (!token) return;
    let active = true;
    // 認証 SDK は AuthProvider と同じく後から読む(最初の表示を重くしない)。
    void import("@/lib/supabase")
      .then(({ createClient }) => createClient()?.auth.mfa.getAuthenticatorAssuranceLevel())
      .then((result) => {
        if (active && result?.data) setLevels({ current: result.data.currentLevel, next: result.data.nextLevel, token });
      });
    return () => {
      active = false;
    };
  }, [token]);

  // 前のログインの結果は使わない。
  const known = token && levels?.token === token ? levels : null;
  const state = known
    ? mfaGateState({ pathname, currentLevel: known.current, nextLevel: known.next, mfaRequired: profile?.mfa_required })
    : "none";

  if (state === "verify") return <MfaVerify />;
  if (state === "setup") return <MfaSetupRequired pathname={pathname} />;
  return children;
}

function MfaVerify() {
  const router = useRouter();
  const { refreshProfile } = useAuth();
  const [code, setCode] = useState("");
  const [error, setError] = useState("");
  const { pending, acquire, release } = useActionLock();

  async function verify(e: React.FormEvent) {
    e.preventDefault();
    if (!isTotpCode(code)) {
      setError("認証アプリに出ている6桁の数字を入れてください。");
      return;
    }
    const { createClient } = await import("@/lib/supabase");
    const supabase = createClient();
    if (!supabase || !acquire()) return;
    setError("");
    try {
      const { data: factors, error: listError } = await supabase.auth.mfa.listFactors();
      const factor = factors?.totp[0];
      if (listError || !factor) {
        setError(mfaErrorMessage(listError ?? { code: "mfa_factor_not_found" }));
        return;
      }
      const { error: verifyError } = await supabase.auth.mfa.challengeAndVerify({ factorId: factor.id, code: normalizeTotpCode(code) });
      if (verifyError) {
        // 間違えても、新しいコードを入れ直せば何度でも試せる(回数の制限は Supabase 側)。
        setCode("");
        setError(mfaErrorMessage(verifyError));
        return;
      }
      // 確認が済むとセッションが aal2 に変わり、onAuthStateChange でこの画面も外れる。
      await refreshProfile();
      router.refresh();
    } finally {
      release();
    }
  }

  async function signOut() {
    const { createClient } = await import("@/lib/supabase");
    const supabase = createClient();
    await supabase?.auth.signOut({ scope: "local" });
    router.replace("/login");
  }

  return (
    <div className="mx-auto max-w-md space-y-4">
      <h1 className="flex items-center gap-2 text-2xl font-bold">
        <ShieldCheck className="size-6 text-blue-300" aria-hidden />
        二段階認証
      </h1>
      <p className="text-sm text-slate-300">認証アプリに出ている6桁のコードを入れてください。</p>
      <form onSubmit={verify} className="space-y-3">
        <Input
          inputMode="numeric"
          autoComplete="one-time-code"
          placeholder="123456"
          maxLength={8}
          value={code}
          onChange={(e) => setCode(e.target.value)}
          autoFocus
        />
        {error ? <p className="text-sm text-red-400">{error}</p> : null}
        <Button type="submit" className="w-full" disabled={pending}>
          確認する
        </Button>
      </form>
      <p className="text-sm text-slate-300">
        認証アプリを使えなくなったときは、
        <Link href="/contact" className="inline-block text-blue-400 transition active:scale-[0.98]">お問い合わせ</Link>
        から運営に連絡してください。
      </p>
      <button type="button" onClick={signOut} className="text-sm text-slate-400 underline transition active:scale-[0.98]">
        ログアウトする
      </button>
    </div>
  );
}

function MfaSetupRequired({ pathname }: { pathname: string }) {
  return (
    <div className="mx-auto max-w-md space-y-4">
      <h1 className="flex items-center gap-2 text-2xl font-bold">
        <ShieldCheck className="size-6 text-blue-300" aria-hidden />
        二段階認証の設定が必要です
      </h1>
      <p className="text-sm text-slate-300">
        管理者とコーチの画面は、アカウントの乗っ取りを防ぐため、二段階認証を設定してから使えます。認証アプリ(Google Authenticator など)を用意して、設定してください。
      </p>
      <Link
        href={`/me/security?next=${encodeURIComponent(pathname)}`}
        className="flex min-h-11 w-full items-center justify-center rounded-md bg-blue-600 px-4 text-sm font-semibold text-white transition active:scale-[0.98]"
      >
        二段階認証を設定する
      </Link>
    </div>
  );
}
