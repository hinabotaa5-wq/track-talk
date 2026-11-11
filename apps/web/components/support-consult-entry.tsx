"use client";
import { L } from "@/lib/i18n";

import { coachEventLabels } from "@/lib/coach-recruit";
import { priceHintLabel, type PublicSupportConsult } from "@/lib/support-consult";
import Link from "next/link";

// コーチ詳細の「サポートプラン」（1ヶ月）。価格の決まった完成品ではなく、無料相談の入口を出す。
// 受け入れ状況は、コーチが手で切り替える「受付中／受付停止中」だけ。件数や残り枠は出さない。
export function SupportConsultEntry({ coachID, consult }: { coachID: string; consult: PublicSupportConsult }) {
  const open = consult.reception.status === "open";
  const hint = priceHintLabel(consult.price_min_yen, consult.price_max_yen);
  return (
    <div className="space-y-3">
      <p className="text-sm text-slate-200">{L("1ヶ月の間、練習や調整を継続してサポートします。まず無料で相談し、目標や内容をふまえて、コーチが内容・価格を提案します。動画への返答は期間中は回数の制限なく受けられ、メニューは最初に作成し期間中は必要に応じて見直します。")}</p>
      <p className={`text-sm font-semibold ${open ? "text-emerald-300" : "text-amber-300"}`} role="status">
        {open ? L("受付中") : L("受付停止中")}
      </p>
      <dl className="space-y-2 text-sm">
        <div><dt className="text-xs text-slate-300">{L("相談を受ける種目")}</dt><dd>{consult.events.map((e) => coachEventLabels[e]).join(L("、"))}</dd></div>
        {consult.description ? <div><dt className="text-xs text-slate-300">{L("相談で聞きたいこと・サポートできる内容")}</dt><dd className="whitespace-pre-wrap break-words">{consult.description}</dd></div> : null}
        {hint ? <div><dt className="text-xs text-slate-300">{L("価格の目安")}</dt><dd>{L("{hint}（実際の価格は、相談のあとの提案で決まります）", { hint })}</dd></div> : null}
      </dl>
      {open ? (
        <Link href={`/coaches/${coachID}/consult`} className="flex min-h-11 w-full items-center justify-center rounded-full bg-blue-600 px-4 text-sm font-semibold text-pure-white hover:bg-blue-500">{L("無料相談をする")}</Link>
      ) : (
        <p className="text-xs text-slate-300">{L("いまは新しい相談を受け付けていません。再開をお待ちください。")}</p>
      )}
    </div>
  );
}
