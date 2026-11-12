"use client";
import { L } from "@/lib/i18n";

import { CoachAvatar } from "@/components/coach-photo";
import { coachEventLabels } from "@/lib/coach-recruit";
import type { ReviewCoachOption } from "@/lib/types";

// 動画添削を頼むコーチを選ぶ。料金はどのコーチでも同じなので、値段や人気（件数・評価・順位）は出さず、
// 種目・経験年数・記録・紹介文で選んでもらう。並びはサーバーが日替わりで決めた順のまま。
export function ReviewCoachPicker({ coaches, value, onChange, disabled, error, loading }: {
  coaches: ReviewCoachOption[]; value: string; onChange: (id: string) => void; disabled: boolean; error: string; loading: boolean;
}) {
  return <fieldset disabled={disabled} className="space-y-3 rounded-xl border border-white/20 bg-white/5 p-4">
    <legend className="px-1 text-sm font-semibold">{L("コーチを選ぶ")}</legend>
    {loading ? <p className="text-sm" role="status">{L("受付中のコーチを確認中...")}</p>
      : error ? <p role="alert" className="text-sm text-red-400">{error}</p>
      : coaches.length === 0 ? <p className="text-sm text-slate-300">{L("この種目で受付中のコーチはいません。")}</p>
      : <ul className="space-y-2">
        {coaches.map((c) => {
          const picked = c.id === value;
          return <li key={c.id}>
            <label className={`flex cursor-pointer gap-3 rounded-xl border p-3 text-sm transition ${picked ? "border-blue-400 bg-blue-500/20" : "border-white/20 bg-white/5 hover:border-blue-400/50"}`}>
              <input type="radio" name="review-coach" value={c.id} checked={picked} onChange={() => onChange(c.id)} className="mt-1" aria-label={c.nickname || L("コーチ")} />
              <CoachAvatar url={c.photo_url} name={c.nickname} size={48} />
              <span className="min-w-0 flex-1 space-y-1 break-words">
                <span className="block font-semibold text-white">{c.nickname || L("コーチ")}</span>
                <span className="block text-xs text-slate-300">{c.events.map((e) => coachEventLabels[e]).join(L("、"))}{c.years > 0 ? L(" · 競技歴{years}年", { years: c.years }) : ""}</span>
                {c.best_record ? <span className="block">{c.best_record}</span> : null}
                {c.achievements ? <span className="block whitespace-pre-wrap text-slate-300">{c.achievements}</span> : null}
              </span>
            </label>
          </li>;
        })}
      </ul>}
    {value ? <p className="text-xs text-slate-300">{L("受付中のコーチには決済の確認後に依頼が届きます。期限までに添削が届かなかった場合は、お支払いを取り消すか全額を返金します。チケットは返還します。")}</p> : null}
  </fieldset>;
}
