import { L } from "@/lib/i18n";
import type { WorkOrder, WorkOrderTerms } from "@/lib/types";
import { formatDateTime } from "@/lib/format";

// 運営者がコーチにフォーム添削・その追加相談を委託するときの取引条件(フリーランス保護法3条の明示)。
// 引き受ける前は今の条件を、引き受けた後は書き留めた通知を出す。文面は API が作る。
// 引き受けた通知は、お知らせ(ベル)にも宛先付きで届き、その知らせからこの場所(#work-order)が開く。
export function WorkOrderTermsView({ terms, recordedAt, deliveredAt }: { terms: WorkOrderTerms; recordedAt?: string; deliveredAt?: string }) {
  return (
    <details id="work-order" className="scroll-mt-20 rounded-lg border border-white/20 bg-white/5 p-2 text-sm" open={!recordedAt}>
      <summary className="cursor-pointer font-semibold text-white">{L("取引条件{v0}", { v0: recordedAt ? L("（{at}に引き受け）", { at: formatDateTime(recordedAt) }) : L("（引き受ける前に確認してください）") })}</summary>
      <dl className="mt-2 space-y-1">
        {terms.items.map((item) => (
          <div key={item.label}>
            <dt className="sr-only">{item.label}</dt>
            <dd className="whitespace-pre-wrap text-slate-100">{item.label}: {item.value}</dd>
          </div>
        ))}
      </dl>
      {deliveredAt ? <p className="mt-2 text-xs text-slate-300">{L("この通知は {v0} にお知らせで届けました。", { v0: formatDateTime(deliveredAt) })}</p> : null}
      {recordedAt ? null : <p className="mt-2 text-xs text-slate-300">{L("「引き受ける」を押すと、この条件で業務を受けたことになり、通知として記録され、お知らせにも届きます。")}</p>}
    </details>
  );
}

export function WorkOrderView({ order }: { order: WorkOrder }) {
  return <WorkOrderTermsView terms={order.terms} recordedAt={order.created_at} deliveredAt={order.delivered_at} />;
}
