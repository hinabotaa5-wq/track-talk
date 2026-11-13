import { L } from "@/lib/i18n";
import type { WorkOrder, WorkOrderTerms } from "@/lib/types";

// 運営者がコーチにフォーム添削・その追加相談を委託するときの取引条件(フリーランス保護法3条の明示)。
// 引き受ける前は今の条件を、引き受けた後は書き留めた通知を出す。文面は API が作る。
// 画面の最下部に置き、初期状態は折りたたみ(本体 #625)。配信記録の表示は出さない。
export function WorkOrderTermsView({ terms, offered = false }: { terms: WorkOrderTerms; offered?: boolean }) {
  return (
    <details id="work-order" className="scroll-mt-20 rounded-lg border border-white/20 bg-white/5 p-2 text-sm">
      <summary className="cursor-pointer font-semibold text-white">{L("取引条件")}</summary>
      <dl className="mt-2 space-y-1">
        {terms.items.map((item) => (
          <div key={item.label}>
            <dt className="sr-only">{item.label}</dt>
            <dd className="whitespace-pre-wrap text-slate-100">{item.label}: {item.value}</dd>
          </div>
        ))}
      </dl>
      {offered ? <p className="mt-2 text-xs text-slate-300">{L("「引き受ける」を押すと、この条件で業務を受けたことになり、通知として記録され、お知らせにも届きます。")}</p> : null}
    </details>
  );
}

export function WorkOrderView({ order }: { order: WorkOrder }) {
  return <WorkOrderTermsView terms={order.terms} />;
}
