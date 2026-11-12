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

const returnReasons = {
  video_unviewable: "動画を確認できない",
  wrong_event: "対象の競技・種目が違う",
  medical: "医療上の判断を求める内容",
  other: "その他",
} as const;

export function CoachWithdrawForm({
  requestID,
  token,
  onWithdrawn,
}: {
  requestID: string;
  token: string | null;
  onWithdrawn: (request: CoachReviewRequest) => void;
}) {
  const [reasonCode, setReasonCode] = useState("");
  const [reason, setReason] = useState("");
  const [error, setError] = useState("");
  const { pending, acquire, release } = useActionLock();
  const confirmDialog = useConfirm();
  const over = reason.length > maxWithdrawReasonChars;

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    if (!reasonCode) {
      setError(L("返却理由を選んでください"));
      return;
    }
    if (over) {
      setError(L("{maxWithdrawReasonChars}文字以内で書いてください", { maxWithdrawReasonChars }));
      return;
    }
    const ok = await confirmDialog({
      title: L("この依頼を返却しますか？"),
      body: L("返却すると依頼はすぐに終了し、取り消せません。依頼者に知らせ、代金は取り消し・返金します。"),
      confirmLabel: L("返却する"),
      cancelLabel: L("やめる"),
    });
    if (!ok || !acquire()) return;
    setError("");
    try {
      onWithdrawn(
        await apiFetch<CoachReviewRequest>(`/v1/coach/requests/${requestID}/withdraw`, {
          method: "POST",
          token,
          body: JSON.stringify({ reason_code: reasonCode, reason: reason.trim() || null }),
        }),
      );
    } catch (err) {
      setError(err instanceof ApiRequestError ? err.fields?.reason_code || err.fields?.reason || err.message : L("送信に失敗しました"));
    } finally {
      release();
    }
  }

  return (
    <details className="rounded-xl border border-white/20 glass bg-white/10 p-3 text-sm">
      <summary className="cursor-pointer font-semibold">{L("この依頼を返却する")}</summary>
      <form onSubmit={onSubmit} className="mt-3 space-y-3">
        <p className="text-xs text-slate-300">{L("動画を確認できない場合や、添削として扱えない依頼を終了し、お支払いを全額返金・取消します。チケットは返還します。")}</p>
        <label className="block space-y-1">
          <span className="text-xs text-slate-300">{L("返却理由")}</span>
          <select className="w-full rounded-md border border-white/25 bg-white/10 p-2" value={reasonCode} onChange={(e) => setReasonCode(e.target.value)} required disabled={pending}>
            <option value="">{L("選んでください")}</option>
            {Object.entries(returnReasons).map(([code, label]) => <option key={code} value={code}>{L(label)}</option>)}
          </select>
        </label>
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
        <Button type="submit" variant="secondary" className="w-full" disabled={pending || !reasonCode}>{L("返却する")}</Button>
      </form>
    </details>
  );
}

// 依頼者に見せる、コーチが返却して依頼が終わったことの説明。
export function ReviewWithdrawalNotice({ withdrawal, paid }: { withdrawal: ReviewWithdrawal; paid: boolean }) {
  const code = withdrawal.reason_code;
  const reasonLabel = code ? returnReasons[code as keyof typeof returnReasons] : undefined;
  return (
    <section className="space-y-2 rounded-xl border border-amber-400/40 bg-amber-500/15 p-3 text-sm">
      <p className="font-semibold text-white">{L("担当のコーチの都合で、この依頼は終了しました")}</p>
      <p className="text-xs text-slate-300">{formatDateTime(withdrawal.withdrawn_at)}</p>
      {reasonLabel ? <p>{L(reasonLabel)}</p> : null}
      {withdrawal.reason ? <p className="whitespace-pre-wrap break-words">{L("コーチからのひとこと: {reason}", { reason: withdrawal.reason })}</p> : null}
      <p className="text-slate-100">{L("{v0}別のコーチには回りません。必要なら、コーチを選んで申し込み直せます。", { v0: paid ? L("お支払いは取り消します（PayPay は返金します）。") : "" })}</p>
    </section>
  );
}
