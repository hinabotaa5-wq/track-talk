"use client";
import { L } from "@/lib/i18n";

import { useEffect } from "react";
import { purchaseNoticeLabel } from "@/lib/minor-purchase";

// 全ユーザーが購入のたびに確認するチェック。画面を開くたび未チェックから始め、APIでも検証する。
// onShown は画面に出した時に呼ぶ（表示した時刻を証跡として残すため）。
export function MinorPurchaseDeclaration({ checked, onChange, onShown, disabled }: { checked: boolean; onChange: (checked: boolean) => void; onShown?: () => void; disabled?: boolean }) {
  useEffect(() => {
    onShown?.();
  }, [onShown]);
  return (
    <div className="space-y-2 rounded-xl border border-amber-400/40 bg-amber-500/10 p-3 text-sm">
      <label className="flex items-start gap-2 font-semibold text-slate-100">
        <input
          type="checkbox"
          className="mt-1 size-4 shrink-0"
          checked={checked}
          disabled={disabled}
          onChange={(e) => onChange(e.target.checked)}
        />
        <span>{L(purchaseNoticeLabel)}</span>
      </label>
    </div>
  );
}
