"use client";
import { useState, type FormEvent } from "react";
import { apiFetch, apiErrorText } from "@/lib/api";
import { formatYen } from "@/lib/format";
import type { CoachApplication } from "@/lib/types";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { useActionLock } from "@/lib/use-action-lock";

export function CoachNominationSettings({ coach, token, onSaved }: { coach: CoachApplication; token: string | null; onSaved: (c: CoachApplication) => void }) {
  const [fee, setFee] = useState(coach.nomination_fee_yen == null ? "" : String(coach.nomination_fee_yen));
  const [error, setError] = useState("");
  const [saved, setSaved] = useState(false);
  const { pending, acquire, release } = useActionLock();
  async function submit(e: FormEvent) {
    e.preventDefault(); if (!acquire()) return;
    setError(""); setSaved(false);
    try {
      const result = await apiFetch<{ application: CoachApplication }>("/v1/coach/me", { method:"PATCH", token,
        body:JSON.stringify({ nomination_fee_yen:fee.trim() === "" ? null : Number(fee) }) });
      onSaved(result.application); setSaved(true);
    } catch (e) { setError(apiErrorText(e, "指名料を保存できませんでした")); } finally { release(); }
  }
  return <form onSubmit={submit} className="space-y-3 rounded-xl border border-white/20 bg-white/5 p-4">
    <h2 className="font-bold">動画添削の指名料</h2>
    <p className="text-sm text-slate-200">基本料金1,200円に上乗せする指名料を設定します。空欄は指名受付なし、0円は追加料金なしで指名を受け付けます。</p>
    <label className="block space-y-1 text-sm"><span>指名料（円・100円単位）</span><Input aria-label="指名料" type="number" min={0} max={9998700} step={100} value={fee} disabled={pending} onChange={(e) => { setFee(e.target.value); setSaved(false); }} /></label>
    {fee !== "" && Number.isFinite(Number(fee)) ? <p className="text-sm">利用者の合計：{formatYen(1200 + Number(fee))}／回<br />コーチ報酬：{formatYen(840 + Math.floor(Number(fee) * 0.8))}／回</p> : null}
    <p className="text-xs text-slate-300">基本料金の70％と指名料の80％が報酬になります。料金変更は新しい依頼から適用されます。受付中で、担当種目と受付上限の条件を満たすと指名候補に表示されます。</p>
    {error ? <p role="alert" className="text-sm text-red-400">{error}</p> : null}
    {saved ? <p role="status" className="text-sm text-blue-300">指名料を保存しました。</p> : null}
    <Button type="submit" disabled={pending}>{pending ? "保存中…" : "指名料を保存する"}</Button>
  </form>;
}
