"use client";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { apiFetch, apiErrorText } from "@/lib/api";
import { inPersonStatusLabels, type CoachingInPerson } from "@/lib/coaching-in-person";
import { formatDateTime, formatYen } from "@/lib/format";
import type { CoachingContract } from "@/lib/types";
import { useActionLock } from "@/lib/use-action-lock";
import { useCallback, useEffect, useRef, useState } from "react";

export function CoachingInPersonPanel({ contract, token, guardianToken }: { contract: CoachingContract; token: string | null; guardianToken?: string }) {
  const path = guardianToken ? `/v1/guardian/${encodeURIComponent(guardianToken)}/contracts/${contract.id}` : `/v1/coaching/contracts/${contract.id}`;
  const [data, setData] = useState<CoachingInPerson | null>(null); const [error, setError] = useState("");
  const [starts, setStarts] = useState(""); const [venue, setVenue] = useState(""); const [area, setArea] = useState(""); const [note, setNote] = useState(""); const requestKey = useRef(crypto.randomUUID());
  const { pending, acquire, release } = useActionLock();
  const reload = useCallback(async () => { const d = await apiFetch<CoachingInPerson>(`${path}/in-person`, { token }); setData(d); }, [path, token]);
  useEffect(() => { let live = true; const load = () => { apiFetch<CoachingInPerson>(`${path}/in-person`, { token }).then((d) => { if (live) setData(d); }).catch((e) => { if (live) setError(apiErrorText(e, "提供状況を読み込めませんでした")); }); }; load(); const t = window.setInterval(load, 30000); window.addEventListener("focus", load); return () => { live = false; window.clearInterval(t); window.removeEventListener("focus", load); }; }, [path, token]);
  async function action(id: string, name: string) {
    if (!acquire()) return; setError("");
    try { await apiFetch(`${path}/in-person/${id}/${name}`, { method: "POST", token }); await reload(); } catch (e) { setError(apiErrorText(e, "更新できませんでした")); } finally { release(); }
  }
  async function propose() {
    if (!acquire()) return; setError("");
    try { await apiFetch(`${path}/in-person`, { method: "POST", token, body: JSON.stringify({ request_key: requestKey.current, starts_at: new Date(starts).toISOString(), venue_name: venue, venue_area: area, meeting_note: note, participants: 1 }) }); requestKey.current = crypto.randomUUID(); setStarts(""); await reload(); } catch (e) { setError(apiErrorText(e, "日時・地域・会場を確認してください")); } finally { release(); }
  }
  const isCoach = data?.role === "coach"; const canConfirm = data?.role === "guardian" || data?.role === "requester" && !data.minor;
  const writable = data?.fulfillment_state === "open" && ["active", "ended"].includes(contract.status);
  return <section aria-label="対面指導の提供状況" className="space-y-4 rounded-2xl border border-emerald-400/30 bg-emerald-500/10 p-4">
    <h2 className="font-bold">対面指導</h2>
    <p className="text-sm">{contract.venue_area} · {contract.venue_name}<br />料金 {formatYen(contract.price_yen)}（交通費 {formatYen(contract.travel_fee_yen ?? 0)} を含む）</p>
    {data ? <><p className="text-sm">対面 {data.usage.completed} / {data.session_count}回実施済み（予約中 {data.usage.reserved}回）</p>
      <p className="text-xs text-slate-300">双方が確認した対面1回を1単位として未提供分を計算します。期間終了48時間後に未実施分を自動返金します。場所と集合方法はこの専用欄だけに表示します。</p>
      {data.fulfillment_state === "refund_pending" || data.fulfillment_state === "refund_failed" ? <p className="text-sm">未提供分の返金 {formatYen(data.refund_amount_yen ?? 0)}：{data.fulfillment_state === "refund_failed" ? "運営が再確認しています" : "処理中"}</p> : null}
      {data.sessions.map((c) => <article key={c.id} className="space-y-2 border-t border-white/20 pt-3"><p className="text-sm font-semibold">{formatDateTime(c.starts_at)} 〜 {formatDateTime(c.ends_at)}</p><p className="text-sm">{c.venue_area} · {c.venue_name}</p>{c.meeting_note ? <p className="text-xs text-slate-300">集合方法：{c.meeting_note}</p> : null}<p className="text-xs text-slate-300">{inPersonStatusLabels[c.status]} · {c.participants}人</p>
        {writable && c.status === "proposed" && canConfirm ? <Button disabled={pending} onClick={() => void action(c.id, "confirm")}>日時・場所を確認して予約を確定</Button> : null}
        {writable && c.status === "confirmed" && isCoach ? <Button disabled={pending} onClick={() => void action(c.id, "report-completed")}>終了後に実施を報告</Button> : null}
        {writable && c.status === "completion_pending" && canConfirm ? <Button disabled={pending} onClick={() => void action(c.id, "confirm-completed")}>実施したことを確認</Button> : null}
        {writable && !["completed", "cancelled"].includes(c.status) ? <button disabled={pending} onClick={() => void action(c.id, "cancel")} className="block text-xs underline">取り消す・実施されなかった（回数を戻す）</button> : null}
      </article>)}
      {isCoach && contract.status === "active" && data.fulfillment_state === "open" && data.usage.reserved + data.usage.completed < data.session_count ? <div className="space-y-3 border-t border-white/20 pt-3"><h3 className="text-sm font-semibold">対面の候補を登録（{data.session_minutes}分）</h3><label className="block text-sm">日時<Input aria-label="対面の候補日時" type="datetime-local" value={starts} onChange={(e) => setStarts(e.target.value)} disabled={pending} /></label><label className="block text-sm">地域<Input aria-label="対面の地域" value={area} onChange={(e) => setArea(e.target.value)} disabled={pending} /></label><label className="block text-sm">会場<Input aria-label="対面の会場" value={venue} onChange={(e) => setVenue(e.target.value)} disabled={pending} /></label><label className="block text-sm">集合方法<Input aria-label="集合方法" value={note} onChange={(e) => setNote(e.target.value)} disabled={pending} /></label><Button disabled={pending || !starts || !venue || !area} onClick={() => void propose()}>候補を登録する</Button><p className="text-xs text-slate-300">変更は取り消してから登録します。参加者との重複予約はできません。</p></div> : null}
      {data.minor ? <p className="text-xs text-slate-300">未成年の対面は保護者が日時・場所・料金を確認します。</p> : null}
    </> : <p className="text-sm">読み込み中…</p>}
    {error ? <p role="alert" className="text-sm text-red-400">{error}</p> : null}
  </section>;
}
