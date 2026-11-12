"use client";
import { L } from "@/lib/i18n";

import { Button } from "@/components/ui/button";
import type { ConfirmRow } from "@/lib/purchase-confirm";

// PurchaseConfirm は、支払い画面へ進む前の最終確認（本体 #569）。確定ボタンを押すまで、何も送らず・何も確保しない。
export function PurchaseConfirm({ rows, confirmLabel, onConfirm, onBack, pending, confirmDisabled, children }: {
  rows: ConfirmRow[];
  confirmLabel: string;
  onConfirm: () => void;
  onBack: () => void;
  pending?: boolean;
  confirmDisabled?: boolean;
  children?: React.ReactNode;
}) {
  return (
    <section aria-labelledby="purchase-confirm-heading" className="space-y-3 rounded-xl border border-blue-400/40 bg-blue-500/10 p-4 text-sm">
      <h2 id="purchase-confirm-heading" className="font-bold">{L("お申込み内容の確認")}</h2>
      <dl className="space-y-2">
        {rows.map((row) => (
          <div key={row.label} className="space-y-0.5">
            <dt className="text-xs font-semibold text-slate-300">{row.label}</dt>
            <dd className="text-slate-100">{row.value}</dd>
          </div>
        ))}
      </dl>
      {children}
      <div className="flex flex-col gap-2">
        <Button type="button" className="w-full" disabled={pending || confirmDisabled} onClick={onConfirm}>{confirmLabel}</Button>
        <Button type="button" variant="secondary" className="w-full" disabled={pending} onClick={onBack}>{L("戻って直す")}</Button>
      </div>
    </section>
  );
}
