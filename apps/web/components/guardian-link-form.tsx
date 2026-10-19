"use client";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { monthlyLimitLabel } from "@/lib/push";
import type { CoachingGuardian, CoachingMe } from "@/lib/types";
import { useActionLock } from "@/lib/use-action-lock";
import { useEffect, useState, type FormEvent } from "react";

const guardianLabels: Record<CoachingGuardian["status"], string> = {
  pending: "保護者の同意待ち",
  consented: "保護者が同意済み",
  declined: "保護者が同意しませんでした",
  revoked: "保護者が同意を取り消しました",
};

// 18歳未満の人が、保護者に同意のリンクをメールで送る。パーソナル指導は、コーチが引き受けたあとのここで初めて聞く。
// リンクは API が保護者のメールあてに送り、本人の画面には出さない（本人だけで同意できないように）。
export function GuardianLinkForm({ token }: { token: string | null }) {
  const [guardian, setGuardian] = useState<CoachingGuardian | null | undefined>(undefined);
  const [email, setEmail] = useState("");
  const [emailError, setEmailError] = useState("");
  const [message, setMessage] = useState("");
  const { pending, acquire, release } = useActionLock();

  useEffect(() => {
    if (!token) return;
    apiFetch<CoachingMe>("/v1/me/coaching", { token })
      .then((me) => setGuardian(me.guardian))
      .catch(() => setGuardian(null));
  }, [token]);

  if (guardian === undefined) return null;

  // 保護者が一度答えたあとは、前と同じアドレスにしか送れない。
  const emailLocked = Boolean(guardian && guardian.status !== "pending" && guardian.guardian_email);

  async function onSend(event: FormEvent) {
    event.preventDefault();
    if (!acquire()) return;
    setMessage("");
    setEmailError("");
    try {
      const next = await apiFetch<CoachingGuardian>("/v1/me/guardian-link", {
        method: "POST",
        token,
        body: JSON.stringify(emailLocked ? {} : { guardian_email: email.trim() }),
      });
      setGuardian(next);
      setEmail("");
      setMessage(`${next.guardian_email ?? "保護者"} にメールを送りました。届いたメールのリンクから、保護者が同意してお支払いをします。`);
    } catch (err) {
      if (err instanceof ApiRequestError && err.fields?.guardian_email) {
        setEmailError(err.fields.guardian_email);
      } else {
        setMessage(err instanceof ApiRequestError ? err.message : "メールを送れませんでした");
      }
    } finally {
      release();
    }
  }

  return (
    <section className="space-y-3 rounded-2xl border border-white/20 glass bg-white/10 p-4 text-sm">
      <h2 className="font-bold text-white">保護者に知らせる</h2>
      <p className="text-slate-200">
        {guardian ? guardianLabels[guardian.status] : "まだ保護者にメールを送っていません"}
        {guardian?.status === "consented" ? `（${monthlyLimitLabel(guardian.monthly_limit_yen)}）` : null}
      </p>
      {guardian?.guardian_email ? <p className="text-slate-300">送り先: {guardian.guardian_email}</p> : null}
      {guardian?.status === "consented" ? null : (
        <form onSubmit={onSend} className="space-y-2">
          {emailLocked ? (
            <p className="text-xs text-slate-400">
              保護者が一度答えたあとは、送り先のアドレスは変えられません。変えたいときはお問い合わせください。
            </p>
          ) : (
            <>
              <p className="text-slate-300">
                保護者のメールアドレスに、同意とお支払いのページのリンクを TrackTalk から送ります。自分のアドレスではなく、保護者のアドレスを入力してください。
              </p>
              <Input
                type="email"
                autoComplete="off"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                placeholder={guardian?.guardian_email ?? "parent@example.com"}
                aria-label="保護者のメールアドレス"
              />
              {emailError ? <p className="text-red-400">{emailError}</p> : null}
            </>
          )}
          <Button type="submit" className="w-full" disabled={pending || (!emailLocked && !email.trim())}>
            {guardian?.guardian_email ? "保護者にメールを送り直す" : "保護者にメールを送る"}
          </Button>
          {guardian ? <p className="text-xs text-slate-400">送り直すと、前に送ったリンクは使えなくなります。メールは1日5回まで送れます。</p> : null}
        </form>
      )}
      {message ? <p className="text-slate-300">{message}</p> : null}
    </section>
  );
}
