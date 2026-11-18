"use client";
import { L } from "@/lib/i18n";

import { LoadingStopwatch } from "@/components/loading-stopwatch";
import { Button } from "@/components/ui/button";
import type { ConfirmRow } from "@/lib/purchase-confirm";

// PurchaseConfirm は、支払い画面へ進む前の最終確認（本体 #569）。確定ボタンを押すまで、何も送らず・何も確保しない。
// 確定を押してから画面が移るまで（pending のあいだ）は、ボタンの下でストップウォッチを回し、pendingLabel の文言を出す（本体 #917）。
// pendingLabel を渡さないときは「処理しています」。待ちが見えるよう、遅らせずに pending になった最初の描画から出す。
export function PurchaseConfirm({ rows, confirmLabel, onConfirm, onBack, pending, confirmDisabled, pendingLabel, children }: {
  rows: ConfirmRow[];
  confirmLabel: string;
  onConfirm: () => void;
  onBack: () => void;
  pending?: boolean;
  confirmDisabled?: boolean;
  pendingLabel?: string;
  children?: React.ReactNode;
}) {
  return (
    <section aria-labelledby="purchase-confirm-heading" className="space-y-3 rounded-xl border border-blue-400/40 bg-blue-500/10 p-4 text-sm">
      <h2 id="purchase-confirm-heading" className="font-bold">{L("お申込み内容の確認")}</h2>
      <dl className="space-y-2">
        {rows.map((row) => (
          <div key={row.label} className="space-y-0.5">
            <dt className="text-xs font-semibold text-slate-300">{row.label}</dt>
            <dd className={row.multiline ? "text-slate-100 whitespace-pre-wrap break-words" : "text-slate-100"}>{row.value}</dd>
          </div>
        ))}
      </dl>
      {children}
      <div className="flex flex-col gap-2">
        <Button type="button" className="w-full" disabled={pending || confirmDisabled} onClick={onConfirm}>{confirmLabel}</Button>
        <Button type="button" variant="secondary" className="w-full" disabled={pending} onClick={onBack}>{L("戻って直す")}</Button>
      </div>
      {pending ? (
        <div role="status" aria-busy="true" className="flex flex-col items-center gap-2 pt-2 text-slate-200">
          <LoadingStopwatch />
          <p className="text-sm font-semibold">{pendingLabel ?? L("処理しています")}</p>
        </div>
      ) : null}
    </section>
  );
}
