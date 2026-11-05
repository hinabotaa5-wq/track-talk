import { minorPurchaseDeclarationLabel, minorPurchaseDeclarationNote } from "@/lib/minor-purchase";

// MinorPurchaseDeclaration は、18歳未満の購入のたびに出すチェック。チェックしないと支払いに進めない（API でも確かめる）。
// 毎回チェックなしから始める。18歳以上には出さない（呼ぶ側で出し分ける）。
export function MinorPurchaseDeclaration({ checked, onChange, disabled }: { checked: boolean; onChange: (checked: boolean) => void; disabled?: boolean }) {
  return (
    <div className="space-y-2 rounded-xl border border-amber-400/40 bg-amber-500/10 p-3 text-sm">
      <p className="text-slate-200">{minorPurchaseDeclarationNote}</p>
      <label className="flex items-start gap-2 font-semibold text-slate-100">
        <input
          type="checkbox"
          className="mt-1 size-4 shrink-0"
          checked={checked}
          disabled={disabled}
          onChange={(e) => onChange(e.target.checked)}
        />
        <span>{minorPurchaseDeclarationLabel}</span>
      </label>
    </div>
  );
}
