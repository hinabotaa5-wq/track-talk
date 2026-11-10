"use client";
import { L } from "@/lib/i18n";

import { useConfirm } from "@/components/confirm-dialog";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { formatDateTime } from "@/lib/format";
import { maxWithdrawReasonChars } from "@/lib/review";
import type { CoachReviewRequest, ReviewWithdrawal } from "@/lib/types";
import { useActionLock } from "@/lib/use-action-lock";
import { useState, type FormEvent } from "react";

// 引受後・返却前のコーチ都合の辞退。辞退した依頼はその場で終わり、別のコーチには回らない。
// 支払いは期限切れと同じく取り消し（PayPay は返金）、チケットは残高に戻る。

// コーチが引き受けた依頼を辞退するフォーム。添削中と、引受後に差し戻して再提出を待っている間に使える。
export function CoachWithdrawForm({
  requestID,
  token,
  onWithdrawn,
}: {
  requestID: string;
  token: string | null;
  onWithdrawn: (request: CoachReviewRequest) => void;
}) {
  const [reason, setReason] = useState("");
  const [error, setError] = useState("");
  const { pending, acquire, release } = useActionLock();
  const confirmDialog = useConfirm();
  const over = reason.length > maxWithdrawReasonChars;

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    if (over) {
      setError(L("{maxWithdrawReasonChars}文字以内で書いてください", { maxWithdrawReasonChars }));
      return;
    }
    const ok = await confirmDialog({
      title: L("この依頼を辞退しますか？"),
      body: L("辞退すると依頼はすぐに終了し、取り消せません。依頼者に知らせ、代金は取り消し・返金します。"),
      confirmLabel: L("辞退する"),
      cancelLabel: L("やめる"),
    });
    if (!ok || !acquire()) return;
    setError("");
    try {
      onWithdrawn(
        await apiFetch<CoachReviewRequest>(`/v1/coach/requests/${requestID}/withdraw`, {
          method: "POST",
          token,
          body: JSON.stringify({ reason: reason.trim() || null }),
        }),
      );
    } catch (err) {
      setError(err instanceof ApiRequestError ? err.fields?.reason || err.message : L("送信に失敗しました"));
    } finally {
      release();
    }
  }

  return (
    <details className="rounded-xl border border-white/20 glass bg-white/10 p-3 text-sm">
      <summary className="cursor-pointer font-semibold">{L("この依頼を辞退する")}</summary>
      <form onSubmit={onSubmit} className="mt-3 space-y-3">
        <p className="text-xs text-slate-300">{L("体調不良などで返却できなくなったときに使います。辞退すると依頼はすぐに終了し、依頼者に知らせます。代金は取り消し（PayPay は返金）、別のコーチには回りません。")}</p>
        <label className="block space-y-1">
          <span className="text-xs text-slate-300">{L("依頼者へのひとこと（任意）")}</span>
          <Textarea
            rows={2}
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            disabled={pending}
            placeholder={L("例: 体調を崩してしまい、期限内に見られなくなりました")}
            aria-invalid={over}
          />
          <span className={`block text-right text-xs ${over ? "text-red-400" : "text-slate-300"}`}>
            {reason.length}/{maxWithdrawReasonChars}
          </span>
        </label>
        {error ? <p className="text-sm text-red-400">{error}</p> : null}
        <Button type="submit" variant="secondary" className="w-full" disabled={pending}>{L("辞退する")}</Button>
      </form>
    </details>
  );
}

// 依頼者に見せる、コーチが辞退して依頼が終わったことの説明。
export function ReviewWithdrawalNotice({ withdrawal, paid }: { withdrawal: ReviewWithdrawal; paid: boolean }) {
  return (
    <section className="space-y-2 rounded-xl border border-amber-400/40 bg-amber-500/15 p-3 text-sm">
      <p className="font-semibold text-white">{L("担当のコーチの都合で、この依頼は終了しました")}</p>
      <p className="text-xs text-slate-300">{formatDateTime(withdrawal.withdrawn_at)}</p>
      {withdrawal.reason ? <p className="whitespace-pre-wrap break-words">{L("コーチからのひとこと: {reason}", { reason: withdrawal.reason })}</p> : null}
      <p className="text-slate-100">{L("{v0}別のコーチには回りません。必要なら、コーチを選んで申し込み直せます。", { v0: paid ? L("お支払いは取り消します（PayPay は返金します）。") : "" })}</p>
    </section>
  );
}
