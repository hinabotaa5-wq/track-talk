"use client";

import { CoachAvatar } from "@/components/coach-photo";
import { coachEventLabels } from "@/lib/coach-recruit";
import type { ReviewCoachOption } from "@/lib/types";

// 動画添削を頼むコーチを選ぶ。料金はどのコーチでも同じなので、値段や人気（件数・評価・順位）は出さず、
// 種目・経験年数・記録・紹介文で選んでもらう。並びはサーバーが日替わりで決めた順のまま。
export function ReviewCoachPicker({ coaches, value, onChange, disabled, error, loading }: {
  coaches: ReviewCoachOption[]; value: string; onChange: (id: string) => void; disabled: boolean; error: string; loading: boolean;
}) {
  return <fieldset disabled={disabled} className="space-y-3 rounded-xl border border-white/20 bg-white/5 p-4">
    <legend className="px-1 text-sm font-semibold">コーチを選ぶ</legend>
    <p className="text-sm text-slate-200">選んだコーチに依頼が届きます。料金はどのコーチでも同じです。並び順は毎日入れ替わります。</p>
    {loading ? <p className="text-sm" role="status">受付中のコーチを確認中...</p>
      : error ? <p role="alert" className="text-sm text-red-400">{error}</p>
      : coaches.length === 0 ? <p className="text-sm text-slate-300">この種目でいま受付中のコーチはいません。時間をおいて開き直してください。</p>
      : <ul className="space-y-2">
        {coaches.map((c) => {
          const picked = c.id === value;
          return <li key={c.id}>
            <label className={`flex cursor-pointer gap-3 rounded-xl border p-3 text-sm transition ${picked ? "border-blue-400 bg-blue-500/20" : "border-white/20 bg-white/5 hover:border-blue-400/50"}`}>
              <input type="radio" name="review-coach" value={c.id} checked={picked} onChange={() => onChange(c.id)} className="mt-1" aria-label={c.nickname || "コーチ"} />
              <CoachAvatar url={c.photo_url} name={c.nickname} size={48} />
              <span className="min-w-0 flex-1 space-y-1 break-words">
                <span className="block font-semibold text-white">{c.nickname || "コーチ"}</span>
                <span className="block text-xs text-slate-300">{c.events.map((e) => coachEventLabels[e]).join("、")}{c.years > 0 ? ` · 競技歴${c.years}年` : ""}</span>
                {c.best_record ? <span className="block">{c.best_record}</span> : null}
                {c.achievements ? <span className="block whitespace-pre-wrap text-slate-300">{c.achievements}</span> : null}
              </span>
            </label>
          </li>;
        })}
      </ul>}
    {value ? <p className="text-xs text-slate-300">選んだコーチが引き受けられない場合や、期限内に回答がない場合は、別のコーチには回さず依頼を終了し、代金を取り消すか返金します（チケットも戻します）。別のコーチを選んで申し込み直せます。</p> : null}
  </fieldset>;
}
