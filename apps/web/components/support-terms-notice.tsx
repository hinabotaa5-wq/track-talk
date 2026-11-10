import { L } from "@/lib/i18n";
import { supportTermsLines } from "@/lib/support-consult";

// 大会までプランの提案画面・最終確認画面に出す、途中で終わる場合のお金とチャットの扱い。
export function SupportTermsNotice({ priceYen, days }: { priceYen: number; days: number }) {
  return (
    <section aria-label={L("途中で終わる場合と返金")} className="space-y-1.5 rounded-xl border border-slate-600 bg-slate-800/60 p-3 text-xs text-slate-200">
      <h4 className="text-sm font-bold text-slate-100">{L("途中で終わる場合と返金")}</h4>
      <dl className="space-y-1.5">
        {supportTermsLines(priceYen, days).map((l) => (
          <div key={l.label}>
            <dt className="font-semibold text-slate-100">{l.label}</dt>
            <dd>{l.text}</dd>
          </div>
        ))}
      </dl>
    </section>
  );
}
