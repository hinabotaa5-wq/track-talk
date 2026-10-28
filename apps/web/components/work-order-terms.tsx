import type { WorkOrder, WorkOrderTerms } from "@/lib/types";
import { formatDateTime } from "@/lib/format";

// 運営者がコーチにフォーム添削・その追加相談を委託するときの取引条件(フリーランス保護法3条の明示)。
// 引き受ける前は今の条件を、引き受けた後は書き留めた通知を出す。文面は API が作る。
export function WorkOrderTermsView({ terms, recordedAt }: { terms: WorkOrderTerms; recordedAt?: string }) {
  return (
    <details className="rounded-lg border border-white/20 bg-white/5 p-2 text-sm" open={!recordedAt}>
      <summary className="cursor-pointer font-semibold text-white">
        取引条件の通知{recordedAt ? `（${formatDateTime(recordedAt)} に引き受け）` : "（引き受ける前に確認してください）"}
      </summary>
      <dl className="mt-2 space-y-2">
        {terms.items.map((item) => (
          <div key={item.label}>
            <dt className="text-xs text-slate-300">{item.label}</dt>
            <dd className="whitespace-pre-wrap text-slate-100">{item.value}</dd>
          </div>
        ))}
      </dl>
      {recordedAt ? null : <p className="mt-2 text-xs text-slate-300">「引き受ける」を押すと、この条件で業務を受けたことになり、通知として記録されます。</p>}
    </details>
  );
}

export function WorkOrderView({ order }: { order: WorkOrder }) {
  return <WorkOrderTermsView terms={order.terms} recordedAt={order.created_at} />;
}
