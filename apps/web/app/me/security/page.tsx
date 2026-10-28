"use client";

import { useAuth } from "@/components/auth-provider";
import { useConfirm } from "@/components/confirm-dialog";
import { MfaEnrollSteps } from "@/components/mfa-enroll-steps";
import { Button } from "@/components/ui/button";
import { isTotpCode, mfaErrorMessage, normalizeTotpCode } from "@/lib/account-security";
import { safeNextPath } from "@/lib/safe-next";
import { createClient } from "@/lib/supabase";
import { useActionLock } from "@/lib/use-action-lock";
import { useRouter } from "next/navigation";
import { useCallback, useEffect, useState } from "react";

type Factor = { id: string; friendly_name?: string; created_at: string };
type Enrolling = { factorId: string; qr: string; secret: string };

// マイページの「二段階認証」。認証アプリ(TOTP)を登録・解除する。
export default function SecurityPage() {
  const { session, loading, profile, refreshProfile } = useAuth();
  const router = useRouter();
  const confirm = useConfirm();
  const [factors, setFactors] = useState<Factor[] | null>(null);
  const [enrolling, setEnrolling] = useState<Enrolling | null>(null);
  const [code, setCode] = useState("");
  const [error, setError] = useState("");
  const [message, setMessage] = useState("");
  const { pending, acquire, release } = useActionLock();

  useEffect(() => {
    if (!loading && !session) router.replace("/login?next=/me/security");
  }, [loading, session, router]);

  const load = useCallback(async () => {
    const supabase = createClient();
    if (!supabase) return;
    const { data, error: err } = await supabase.auth.mfa.listFactors();
    if (err) {
      setError(mfaErrorMessage(err));
      return;
    }
    setFactors(data.totp);
  }, []);

  const userId = session?.user.id;
  useEffect(() => {
    if (!userId) return;
    let active = true;
    void createClient()?.auth.mfa.listFactors().then(({ data, error: err }) => {
      if (!active) return;
      if (err) setError(mfaErrorMessage(err));
      else setFactors(data.totp);
    });
    return () => {
      active = false;
    };
  }, [userId]);

  async function startEnroll() {
    const supabase = createClient();
    if (!supabase || !acquire()) return;
    setError("");
    setMessage("");
    try {
      // 途中でやめた登録(確認前の設定)は消さずに残す。消すと「二段階認証を解除しました」のメールが飛ぶ。
      // 名前は設定ごとに変える(同じ名前だと作れない)。
      const enroll = () => supabase.auth.mfa.enroll({ factorType: "totp", friendlyName: `TrackTalk ${crypto.randomUUID().slice(0, 8)}` });
      let { data, error: err } = await enroll();
      if (err?.code === "too_many_enrolled_mfa_factors") {
        // 途中でやめた登録がたまって上限に達したときだけ、確認前のものを片づけてやり直す。
        const { data: all } = await supabase.auth.mfa.listFactors();
        for (const f of all?.all ?? []) {
          if (f.status === "unverified") await supabase.auth.mfa.unenroll({ factorId: f.id });
        }
        ({ data, error: err } = await enroll());
      }
      if (err || !data) {
        setError(mfaErrorMessage(err ?? {}));
        return;
      }
      setEnrolling({ factorId: data.id, qr: data.totp.qr_code, secret: data.totp.secret });
      setCode("");
    } finally {
      release();
    }
  }

  async function finishEnroll(e: React.FormEvent) {
    e.preventDefault();
    if (!enrolling) return;
    if (!isTotpCode(code)) {
      setError("認証アプリに出ている6桁の数字を入れてください。");
      return;
    }
    const supabase = createClient();
    if (!supabase || !acquire()) return;
    setError("");
    try {
      const { error: err } = await supabase.auth.mfa.challengeAndVerify({ factorId: enrolling.factorId, code: normalizeTotpCode(code) });
      if (err) {
        setCode("");
        setError(mfaErrorMessage(err));
        return;
      }
      setEnrolling(null);
      setMessage("二段階認証を設定しました。次からログインのときに、認証アプリのコードも入れます。");
      await load();
      await refreshProfile();
      const next = new URLSearchParams(window.location.search).get("next");
      if (next) router.replace(safeNextPath(next));
    } finally {
      release();
    }
  }

  async function remove(factor: Factor) {
    const ok = await confirm({
      title: "二段階認証を解除しますか？",
      body: "解除すると、パスワードだけでログインできるようになります。管理者・コーチは、設定し直すまで管理・コーチの画面と操作が使えなくなります。",
      confirmLabel: "解除する",
    });
    if (!ok) return;
    const supabase = createClient();
    if (!supabase || !acquire()) return;
    setError("");
    setMessage("");
    try {
      const { error: err } = await supabase.auth.mfa.unenroll({ factorId: factor.id });
      if (err) {
        setError(mfaErrorMessage(err));
        return;
      }
      // 解除しても手元のセッションは aal2 のままなので、取り直して段階を下げる。
      await supabase.auth.refreshSession();
      setMessage("二段階認証を解除しました。");
      await load();
    } finally {
      release();
    }
  }

  if (loading || !session) return <p className="text-sm text-slate-200">読み込み中...</p>;

  return (
    <div className="mx-auto max-w-md space-y-4">
      <h1 className="text-2xl font-bold">二段階認証</h1>
      <p className="text-sm text-slate-300">
        ログインのときに、パスワードに加えて認証アプリ(Google Authenticator・Microsoft Authenticator など)の6桁のコードを入れます。パスワードが漏れても、他人にログインされにくくなります。
      </p>
      {profile?.mfa_required ? (
        <p className="rounded-md border border-amber-300/40 bg-amber-400/10 p-3 text-sm text-amber-100">
          管理者・コーチの画面と操作は、二段階認証を設定してから使えます。
        </p>
      ) : null}
      <section className="space-y-3 rounded-2xl border border-white/20 glass bg-white/10 p-4">
        {factors === null ? (
          <p className="text-sm text-slate-300">読み込み中...</p>
        ) : factors.length > 0 ? (
          <>
            <p className="text-sm font-semibold text-emerald-300">設定済み</p>
            {factors.map((f) => (
              <div key={f.id} className="flex items-center justify-between gap-3">
                <span className="text-sm text-slate-200">{f.friendly_name || "認証アプリ"}</span>
                <Button type="button" variant="secondary" onClick={() => remove(f)} disabled={pending}>
                  解除する
                </Button>
              </div>
            ))}
          </>
        ) : enrolling ? (
          <MfaEnrollSteps qr={enrolling.qr} secret={enrolling.secret} code={code} onCode={setCode} pending={pending} onSubmit={finishEnroll} />
        ) : (
          <>
            <p className="text-sm text-slate-300">まだ設定していません。</p>
            <Button type="button" className="w-full" onClick={startEnroll} disabled={pending}>
              設定を始める
            </Button>
          </>
        )}
        {error ? <p className="text-sm text-red-400">{error}</p> : null}
        {message ? <p className="text-sm text-emerald-300">{message}</p> : null}
      </section>
    </div>
  );
}
