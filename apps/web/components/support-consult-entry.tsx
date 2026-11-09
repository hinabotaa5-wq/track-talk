"use client";

import { coachEventLabels } from "@/lib/coach-recruit";
import { priceHintLabel, type PublicSupportConsult } from "@/lib/support-consult";
import Link from "next/link";

// コーチ詳細の「大会までプラン」。価格の決まった完成品ではなく、無料相談の入口を出す。
// 受け入れ状況は、コーチの対応可能カレンダーから自動で決まる（受付中／受付停止中）。件数や残り枠は出さない。
export function SupportConsultEntry({ coachID, consult }: { coachID: string; consult: PublicSupportConsult }) {
  const open = consult.reception.status === "open";
  const hint = priceHintLabel(consult.price_min_yen, consult.price_max_yen);
  return (
    <div className="space-y-3">
      <p className="text-sm text-slate-200">目標の大会まで、練習や調整を継続してサポートします。まず無料で相談し、大会の日や内容をふまえて、コーチが期間・内容・価格を提案します。</p>
      <p className={`text-sm font-semibold ${open ? "text-emerald-300" : "text-amber-300"}`} role="status">
        {open ? "受付中" : `受付停止中${consult.reception.resume_on ? `（${consult.reception.resume_on}ごろ再開の目安）` : ""}`}
      </p>
      <dl className="space-y-2 text-sm">
        <div><dt className="text-xs text-slate-300">相談を受ける種目</dt><dd>{consult.events.map((e) => coachEventLabels[e]).join("、")}</dd></div>
        {consult.description ? <div><dt className="text-xs text-slate-300">相談で聞きたいこと・サポートできる内容</dt><dd className="whitespace-pre-wrap break-words">{consult.description}</dd></div> : null}
        {hint ? <div><dt className="text-xs text-slate-300">価格の目安</dt><dd>{hint}（実際の価格は、相談のあとの提案で決まります）</dd></div> : null}
      </dl>
      {open ? (
        <Link href={`/coaches/${coachID}/consult`} className="flex min-h-11 w-full items-center justify-center rounded-full bg-blue-600 px-4 text-sm font-semibold text-pure-white hover:bg-blue-500">無料相談をする</Link>
      ) : (
        <p className="text-xs text-slate-300">いまは新しい相談を受け付けていません。再開をお待ちください。</p>
      )}
    </div>
  );
}
