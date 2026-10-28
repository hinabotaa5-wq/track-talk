"use client";

import { useState } from "react";
import { CoachAvatar } from "@/components/coach-photo";
import { formatYen } from "@/lib/format";
import type { ReviewNominee } from "@/lib/types";

export function ReviewNomination({ coaches, value, onChange, disabled, error, loading }: {
  coaches: ReviewNominee[]; value: string; onChange: (id: string) => void; disabled: boolean; error: string; loading: boolean;
}) {
  const [expanded, setExpanded] = useState(false);
  const selected = coaches.find((c) => c.id === value);
  return <fieldset disabled={disabled} className="space-y-3 rounded-xl border border-white/20 bg-white/5 p-4">
    <legend className="px-1 text-sm font-semibold">コーチの指名（任意）</legend>
    <p className="text-sm text-slate-200">通常は、対応できるコーチにおまかせで届きます。</p>
    <label className="flex min-h-11 items-center gap-2 text-sm">
      <input type="checkbox" checked={expanded} onChange={(e) => { setExpanded(e.target.checked); if (!e.target.checked) onChange(""); }} />
      コーチを選んで指名料を追加する
    </label>
    {expanded ? <div className="space-y-3">
      {loading ? <p className="text-sm" role="status">指名できるコーチを確認中...</p> : error ? <p role="alert" className="text-sm text-red-400">{error}</p> : coaches.length === 0 ? <p className="text-sm text-slate-300">いま指名できるコーチはいません。おまかせで依頼できます。</p> : <label className="block space-y-2 text-sm">
        <span>指名するコーチ</span>
        <select aria-label="指名するコーチ" value={value} onChange={(e) => onChange(e.target.value)} className="min-h-12 w-full rounded-md border border-white/25 bg-slate-900 px-3 text-base text-white">
          <option value="">おまかせ（指名料なし）</option>
          {coaches.map((c) => <option key={c.id} value={c.id}>{c.nickname || "コーチ"}：合計{formatYen(c.total_price_yen)}（指名料＋{formatYen(c.nomination_fee_yen)}）</option>)}
        </select>
      </label>}
      {selected ? <div className="space-y-2 text-sm">
        <div className="flex items-center gap-3"><CoachAvatar url={selected.photo_url} name={selected.nickname} size={48} /><p className="font-semibold">{selected.nickname || "コーチ"}</p></div>
        <p>{selected.best_record}</p><p className="whitespace-pre-wrap text-slate-300">{selected.achievements}</p>
        <p className="text-xs text-slate-300">指名したコーチだけに依頼します。引き受けられない場合や期限内に回答がない場合は、代金・指名料を取り消すか返金し、確保したチケットも戻します。</p>
      </div> : null}
    </div> : null}
  </fieldset>;
}
