"use client";
import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { apiFetch, apiErrorText } from "@/lib/api";
import { Button } from "./ui/button";
import { useConfirm } from "./confirm-dialog";

type Settings = { paused: boolean; restrict_to_posts: boolean; allowed_post_ids: string[]; monthly_minutes_limit: number; last_tick_at: string | null };
type Data = { configured: boolean; enabled: boolean; estimated_minutes: number; settings: Settings; connections: { id: string; public_name: string; post_id: string; title: string; state: string }[]; bans: { user_id: string; post_id: string; nickname: string | null; reason: string }[] };

export function VoiceModerationActions({ connectionId, token, onDone }: { connectionId: string; token: string | null; onDone?: () => void }) {
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState("");
  const confirm = useConfirm();
  async function act(action: string) {
    if (busy || !(await confirm({ title: action === "ban" ? "この実況ルームへの参加を停止しますか？" : action === "mute" ? "この参加者のマイクをOFFにしますか？" : "この参加者を退出させますか？", body: "音声は保存していないため、後から再生できません。", confirmLabel: "実行する" }))) return;
    setBusy(true); setMessage("");
    try { await apiFetch(`/v1/admin/voice/connections/${connectionId}`, { token, method: "POST", body: JSON.stringify({ action, reason: "運営による対応" }) }); setMessage("対応しました"); onDone?.(); }
    catch (e) { setMessage(apiErrorText(e, "操作できませんでした")); }
    finally { setBusy(false); }
  }
  return <div className="space-y-2"><div className="flex flex-wrap gap-2"><Button variant="secondary" disabled={busy} onClick={() => void act("mute")}>マイクOFF</Button><Button variant="secondary" disabled={busy} onClick={() => void act("remove")}>退出させる</Button><Button variant="danger" disabled={busy} onClick={() => void act("ban")}>参加停止</Button></div>{message ? <p role="status" className="text-sm">{message}</p> : null}</div>;
}

export function MeetVoiceAdmin({ token }: { token: string | null }) {
  const [data, setData] = useState<Data | null>(null);
  const [settings, setSettings] = useState<Settings | null>(null);
  const [ids, setIds] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const confirm = useConfirm();
  const load = useCallback(async () => {
    try { const next = await apiFetch<Data>("/v1/admin/voice", { token }); setData(next); setSettings(next.settings); setIds(next.settings.allowed_post_ids.join("\n")); }
    catch (e) { setError(apiErrorText(e, "音声通話の管理情報を取得できませんでした")); }
  }, [token]);
  useEffect(() => { const timer = setTimeout(() => void load(), 0); return () => clearTimeout(timer); }, [load]);
  async function save(paused: boolean) {
    if (!settings || busy) return;
    if (paused && !(await confirm({ title: "音声通話を全体停止しますか？", body: "新規参加を止め、接続中の通話も終了します。コメント・動画は利用できます。", confirmLabel: "全体停止する" }))) return;
    setBusy(true); setError("");
    try {
      const allowed = ids.split(/[\s,]+/).filter(Boolean);
      await apiFetch("/v1/admin/voice", { token, method: "PUT", body: JSON.stringify({ paused, restrict_to_posts: settings.restrict_to_posts, allowed_post_ids: allowed, monthly_minutes_limit: settings.monthly_minutes_limit }) });
      await load();
    } catch (e) { setError(apiErrorText(e, "設定を保存できませんでした")); }
    finally { setBusy(false); }
  }
  async function unban(user: string, post: string) {
    if (!(await confirm({ title: "参加停止を解除しますか？", confirmLabel: "解除する" }))) return;
    try { await apiFetch(`/v1/admin/voice/bans/${user}/remove`, { method: "POST", token, body: JSON.stringify({ post_id: post }) }); await load(); }
    catch (e) { setError(apiErrorText(e, "解除できませんでした")); }
  }
  return <section className="space-y-4" aria-label="音声通話の管理"><h2 className="text-lg font-bold">音声通話</h2><p className="text-sm text-slate-300">音声は保存していません。通報された発言を後から再生できません。ミュート解除は本人だけが操作します。</p>
    {error ? <p role="alert" className="text-sm text-red-300">{error}</p> : null}
    {data && settings ? <>
      <div className="space-y-3 rounded-xl border border-white/20 p-4"><p className="text-sm">{!data.enabled ? "環境設定で無効" : !data.configured ? "接続設定が未完了" : settings.paused ? "全体停止中" : "提供設定は有効"}</p><p className="text-sm">今月の概算：{data.estimated_minutes}参加者分・分 / 上限 {settings.monthly_minutes_limit}</p><p className="text-xs text-slate-300">参加券の準備から退出までの概算です。実際の無料枠と通信量はLiveKit Cloudのダッシュボードでも確認してください。</p><p className="text-xs text-slate-300">定期確認：{settings.last_tick_at ? new Date(settings.last_tick_at).toLocaleString("ja-JP", { timeZone: "Asia/Tokyo" }) : "未実行（参加できません）"}</p>
        <label className="flex items-center gap-2 text-sm"><input type="checkbox" checked={settings.restrict_to_posts} onChange={(e) => setSettings({ ...settings, restrict_to_posts: e.target.checked })} />検証用の実況ルームだけに限定</label>
        {settings.restrict_to_posts ? <label className="block text-sm">許可する投稿ID（1行1件）<textarea className="mt-1 w-full rounded bg-slate-800 p-2" value={ids} onChange={(e) => setIds(e.target.value)} rows={3} /></label> : null}
        <label className="block text-sm">月間の概算上限（参加者分・分）<input className="ml-2 w-28 rounded bg-slate-800 p-2" type="number" min={1} max={100000} value={settings.monthly_minutes_limit} onChange={(e) => setSettings({ ...settings, monthly_minutes_limit: Number(e.target.value) })} /></label>
        <div className="flex flex-wrap gap-2"><Button disabled={busy} onClick={() => void save(false)}>設定を保存・提供を再開</Button><Button variant="danger" disabled={busy} onClick={() => void save(true)}>新規参加を停止・全員退出</Button><Button variant="secondary" onClick={() => void load()}>更新</Button></div>
      </div>
      <h3 className="font-semibold">接続・参加予約（{data.connections.length}件）</h3>{data.connections.length === 0 ? <p className="text-sm text-slate-300">接続中の参加者はいません。</p> : data.connections.map((person) => <div className="space-y-2 rounded-xl border border-white/20 p-3" key={person.id}><p className="text-sm font-semibold">{person.public_name} · {person.state === "pending" ? "接続準備中" : "参加中"}</p><Link className="text-sm text-sky-300 underline" href={`/posts/${person.post_id}`}>{person.title}</Link><VoiceModerationActions connectionId={person.id} token={token} onDone={() => void load()} /></div>)}
      <h3 className="font-semibold">参加停止</h3>{data.bans.length === 0 ? <p className="text-sm text-slate-300">参加停止はありません。</p> : data.bans.map((ban) => <div key={`${ban.user_id}:${ban.post_id}`} className="flex flex-wrap items-center justify-between gap-2 rounded-xl border border-white/20 p-3"><p className="text-sm">{ban.nickname ?? "匿名"} · {ban.reason}</p><Button variant="secondary" onClick={() => void unban(ban.user_id, ban.post_id)}>解除</Button></div>)}
    </> : <p className="text-sm text-slate-300">読み込み中…</p>}
  </section>;
}

export function VoiceReportReview({ connectionId, reportId, token, initialStatus }: { connectionId: string; reportId: string; token: string | null; initialStatus: string }) {
  const [closed, setClosed] = useState(initialStatus !== "open");
  const [error, setError] = useState("");
  async function close() {
    try { await apiFetch(`/v1/admin/reports/${reportId}/close`, { method: "POST", token }); setClosed(true); }
    catch (e) { setError(apiErrorText(e, "対応状況を保存できませんでした")); }
  }
  return <div className="space-y-2"><VoiceModerationActions connectionId={connectionId} token={token} /><Button variant="secondary" disabled={closed} onClick={() => void close()}>{closed ? "対応済み" : "通報の対応を終える"}</Button>{error ? <p role="alert" className="text-sm text-red-300">{error}</p> : null}</div>;
}
