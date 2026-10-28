"use client";

import { PageLoading } from "@/components/page-loading";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { apiFetch, apiErrorText } from "@/lib/api";
import { callStatusLabels, type CoachingSupport } from "@/lib/coaching-support";
import { formatDateTime, formatYen } from "@/lib/format";
import type { CoachingContract } from "@/lib/types";
import { useActionLock } from "@/lib/use-action-lock";
import { useCallback, useEffect, useRef, useState } from "react";

export function CoachingSupportPanel({ contract, token, guardianToken }: { contract: CoachingContract; token: string | null; guardianToken?: string }) {
  const path = guardianToken ? `/v1/guardian/${encodeURIComponent(guardianToken)}/contracts/${contract.id}` : `/v1/coaching/contracts/${contract.id}`;
  const [data, setData] = useState<CoachingSupport | null>(null); const [error, setError] = useState("");
  const [starts, setStarts] = useState(""); const [url, setURL] = useState(""); const requestKey = useRef(crypto.randomUUID());
  const { pending, acquire, release } = useActionLock();
  const reload = useCallback(async () => { const d = await apiFetch<CoachingSupport>(`${path}/support`, { token }); setData(d); }, [path, token]);
  useEffect(() => { let live = true; const load = () => { apiFetch<CoachingSupport>(`${path}/support`, { token }).then((d) => { if (live) setData(d); }).catch((e) => { if (live) setError(apiErrorText(e,"提供状況を読み込めませんでした")); }); }; load(); const t = window.setInterval(load,30000); window.addEventListener("focus",load); return () => { live = false; window.clearInterval(t); window.removeEventListener("focus",load); }; }, [path,token]);
  async function action(id: string, name: string) {
    if (!acquire()) return; setError("");
    try { await apiFetch(`${path}/calls/${id}/${name}`, { method:"POST",token }); await reload(); } catch (e) { setError(apiErrorText(e,"更新できませんでした")); } finally { release(); }
  }
  async function propose() {
    if (!acquire()) return; setError("");
    try { await apiFetch(`${path}/calls`, { method:"POST",token,body:JSON.stringify({ request_key:requestKey.current,starts_at:new Date(starts).toISOString(),join_url:url }) }); requestKey.current = crypto.randomUUID(); setStarts(""); setURL(""); await reload(); } catch (e) { setError(apiErrorText(e,"通話の日時と参加リンクを確認してください")); } finally { release(); }
  }
  const isCoach = data?.role === "coach"; const canConfirm = data?.role === "guardian" || data?.role === "requester" && !data.minor;
  const writable = data?.fulfillment_state === "open" && ["active","ended"].includes(contract.status);
  return <section aria-label="大会サポートの提供状況" className="space-y-4 rounded-2xl border border-blue-400/30 bg-blue-500/10 p-4">
    <h2 className="font-bold">大会までのサポート・通話</h2>
    <p className="text-sm">{contract.competition_name} · 大会 {contract.competition_date}<br />支援開始 {contract.support_start_date}</p>
    {data ? <><p className="text-sm">動画添削 {data.usage.video_answered} / {data.video_reviews}回回答済み（提出済み {data.usage.video_submitted}回）<br />通話 {data.usage.calls_completed} / {data.call_count}回実施済み（予約中 {data.usage.calls_reserved}回）</p>
      <p className="text-xs text-slate-300">動画への回答1組と、双方が確認した通話1回を各1単位として未提供分を計算します。終了48時間後に未提供分を自動返金します。メニューや事務連絡は回数に含みません。</p>
      {data.fulfillment_state === "refund_pending" || data.fulfillment_state === "refund_failed" ? <p className="text-sm">未提供分の返金 {formatYen(data.refund_amount_yen ?? 0)}：{data.fulfillment_state === "refund_failed" ? "運営が再確認しています" : "処理中"}</p> : null}
      {data.calls.map((c) => <article key={c.id} className="space-y-2 border-t border-white/20 pt-3"><p className="text-sm font-semibold">{formatDateTime(c.starts_at)} 〜 {formatDateTime(c.ends_at)}</p><p className="text-xs text-slate-300">{callStatusLabels[c.status]}</p>
        {c.status === "confirmed" || c.status === "completion_pending" ? <a href={c.join_url} target="_blank" rel="noopener noreferrer" className="block text-blue-300 underline">通話に参加する（外部サービス）</a> : null}
        {writable && c.status === "proposed" && canConfirm ? <Button disabled={pending} onClick={() => void action(c.id,"confirm")}>日時を確認して予約を確定</Button> : null}
        {writable && c.status === "confirmed" && isCoach ? <Button disabled={pending} onClick={() => void action(c.id,"report-completed")}>通話終了後に実施を報告</Button> : null}
        {writable && c.status === "completion_pending" && canConfirm ? <Button disabled={pending} onClick={() => void action(c.id,"confirm-completed")}>通話を実施したことを確認</Button> : null}
        {writable && !["completed","cancelled"].includes(c.status) ? <button disabled={pending} onClick={() => void action(c.id,"cancel")} className="block text-xs underline">取り消す・実施されなかった（回数を戻す）</button> : null}
      </article>)}
      {isCoach && contract.status === "active" && data.fulfillment_state === "open" && data.usage.calls_reserved + data.usage.calls_completed < data.call_count ? <div className="space-y-3 border-t border-white/20 pt-3"><h3 className="text-sm font-semibold">通話の候補を登録（{data.call_minutes}分）</h3><label className="block text-sm">日時<Input aria-label="通話の候補日時" type="datetime-local" value={starts} onChange={(e) => setStarts(e.target.value)} disabled={pending} /></label><label className="block text-sm">参加リンク<Input aria-label="通話の参加リンク" type="url" value={url} onChange={(e) => setURL(e.target.value)} disabled={pending} placeholder="Google Meet・Zoom・Teams" /></label><Button disabled={pending || !starts || !url} onClick={() => void propose()}>候補を登録する</Button><p className="text-xs text-slate-300">日時変更は取り消してから登録します。参加者との重複予約はできません。</p></div> : null}
      {data.minor ? <p className="text-xs text-slate-300">未成年の通話は保護者が日時と実施を確認します。保護者も通話に参加できます。</p> : null}
    </> : <PageLoading label="読み込み中…" />}
    {error ? <p role="alert" className="text-sm text-red-400">{error}</p> : null}
  </section>;
}
