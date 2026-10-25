"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import type { Room } from "livekit-client";
import Link from "next/link";
import { useAuth } from "./auth-provider";
import { Button } from "./ui/button";
import { apiFetch, apiErrorText, ApiRequestError } from "@/lib/api";
import type { MeetVoiceJoin, MeetVoiceStatus } from "@/lib/types";
import { connectMeetVoice } from "@/lib/meet-voice";

type Person = { id: string; name: string; local: boolean; muted: boolean; speaking: boolean };
type Connection = { room: Room; disconnect: () => Promise<void>; id: string; token: string; controller: AbortController; unlisten: () => void };

export function MeetVoicePanel({ postId }: { postId: string }) {
  const { token, session } = useAuth();
  const [status, setStatus] = useState<MeetVoiceStatus | null>(null);
  const [phase, setPhase] = useState<"idle" | "joining" | "connected" | "reconnecting">("idle");
  const [people, setPeople] = useState<Person[]>([]);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [audioBlocked, setAudioBlocked] = useState(false);
  const [confirmNeeded, setConfirmNeeded] = useState(false);
  const [volume, setVolume] = useState(80);
  const [reported, setReported] = useState<string[]>([]);
  const [reportTarget, setReportTarget] = useState<Person | null>(null);
  const [reason, setReason] = useState("harassment");
  const [detail, setDetail] = useState("");
  const [reportBusy, setReportBusy] = useState(false);
  const host = useRef<HTMLDivElement>(null);
  const connection = useRef<Connection | null>(null);
  const pending = useRef<AbortController | null>(null);
  const busy = useRef(false);
  const micBusy = useRef(false);
  const volumeRef = useRef(volume);
  const latestToken = useRef(token);
  useEffect(() => { latestToken.current = token; }, [token]);

  const release = useCallback((id: string, authToken: string) => apiFetch(`/v1/posts/${postId}/voice/leave`, {
    token: authToken, method: "POST", body: JSON.stringify({ connection_id: id }), keepalive: true,
  }).catch(() => {}), [postId]);
  const stop = useCallback(async () => {
    pending.current?.abort();
    const current = connection.current;
    connection.current = null;
    if (!current) return;
    current.unlisten();
    current.controller.abort();
    // Stop local media synchronously via disconnect before waiting for the API.
    await Promise.allSettled([current.disconnect(), release(current.id, latestToken.current ?? current.token)]);
  }, [release]);

  useEffect(() => {
    let active = true;
    async function load() {
      if (!latestToken.current) return;
      try {
        const next = await apiFetch<MeetVoiceStatus>(`/v1/posts/${postId}/voice`, { token: latestToken.current });
        if (active) setStatus(next);
      } catch (e) {
        if (active && e instanceof ApiRequestError && e.status === 404) setStatus(null);
      }
    }
    const first = window.setTimeout(() => { setPhase("idle"); setPeople([]); setReportTarget(null); setConfirmNeeded(false); void load(); }, 0);
    const poll = window.setInterval(() => void load(), 15000);
    const pagehide = () => { void stop(); setPhase("idle"); setPeople([]); };
    window.addEventListener("pagehide", pagehide);
    return () => { active = false; clearTimeout(first); clearInterval(poll); window.removeEventListener("pagehide", pagehide); void stop(); };
  }, [postId, session?.user.id, stop]);

  const refreshPeople = (room: Room) => {
    const participants = [room.localParticipant, ...room.remoteParticipants.values()];
    setPeople(participants.map((p) => ({ id: p.identity, name: p.name || "匿名", local: p === room.localParticipant, muted: !p.isMicrophoneEnabled, speaking: p.isSpeaking })));
    for (const participant of room.remoteParticipants.values()) participant.setVolume(volumeRef.current / 100);
    setAudioBlocked(!room.canPlaybackAudio);
  };

  async function leave(message = "退出しました") {
    await stop();
    busy.current = false;
    setPhase("idle"); setPeople([]); setConfirmNeeded(false); setAudioBlocked(false); setNotice(message);
  }

  async function heartbeat(confirm = false) {
    const current = connection.current;
    if (!current || current.controller.signal.aborted) return;
    try {
      const data = await apiFetch<{ confirmation_required: boolean }>(`/v1/posts/${postId}/voice/heartbeat`, {
        method: "POST", token: latestToken.current ?? current.token,
        body: JSON.stringify({ connection_id: current.id, confirm }),
      });
      if (connection.current === current) { setConfirmNeeded(data.confirmation_required); setNotice(""); }
    } catch (e) {
      if (connection.current !== current) return;
      if (e instanceof ApiRequestError && [401, 403, 404, 410].includes(e.status)) await leave(e.message);
      else setNotice("接続状況を確認できません。通信が戻らない場合は自動で退出します。");
    }
  }
  useEffect(() => {
    if (phase !== "connected" && phase !== "reconnecting") return;
    const timer = window.setInterval(() => void heartbeat(), 30000);
    return () => clearInterval(timer);
    // The connection and current access token are read from refs.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [phase]);

  async function join() {
    if (!token || busy.current || connection.current || !host.current) return;
    busy.current = true;
    const controller = new AbortController(); pending.current = controller;
    setError(""); setNotice(""); setPhase("joining");
    let credentials: MeetVoiceJoin | null = null;
    try {
      // Let a pending response finish so cancellation can release its connection ID.
      credentials = await apiFetch<MeetVoiceJoin>(`/v1/posts/${postId}/voice/join`, { method: "POST", token });
      controller.signal.throwIfAborted();
      const { RoomEvent } = await import("livekit-client");
      const opened = await connectMeetVoice(credentials, host.current, controller.signal);
      controller.signal.throwIfAborted();
      const room = opened.room;
      const sync = () => refreshPeople(room);
      const reconnecting = () => { setPhase("reconnecting"); };
      const reconnected = () => { setPhase("connected"); sync(); };
      const disconnected = () => { void leave("通話が終了しました。必要に応じて再参加してください。"); };
      const events = [RoomEvent.ParticipantConnected, RoomEvent.ParticipantDisconnected, RoomEvent.TrackMuted, RoomEvent.TrackUnmuted, RoomEvent.ActiveSpeakersChanged, RoomEvent.LocalTrackPublished, RoomEvent.LocalTrackUnpublished, RoomEvent.TrackSubscribed, RoomEvent.AudioPlaybackStatusChanged];
      for (const event of events) room.on(event, sync);
      room.on(RoomEvent.Reconnecting, reconnecting); room.on(RoomEvent.SignalReconnecting, reconnecting); room.on(RoomEvent.Reconnected, reconnected); room.on(RoomEvent.Disconnected, disconnected);
      const unlisten = () => {
        for (const event of events) room.off(event, sync);
        room.off(RoomEvent.Reconnecting, reconnecting); room.off(RoomEvent.SignalReconnecting, reconnecting); room.off(RoomEvent.Reconnected, reconnected); room.off(RoomEvent.Disconnected, disconnected);
      };
      connection.current = { ...opened, id: credentials.connection_id, token, controller, unlisten };
      refreshPeople(room); setPhase("connected");
    } catch (e) {
      controller.abort();
      if (credentials) await release(credentials.connection_id, token);
      if (!controller.signal.reason || !(e instanceof DOMException && e.name === "AbortError")) {
        setError(apiErrorText(e, "通話に接続できませんでした。満員・通信状況・サービスの利用上限をご確認ください。"));
      }
      setPhase("idle");
    } finally { busy.current = false; }
  }

  async function toggleMic() {
    const current = connection.current;
    if (!current || micBusy.current) return;
    micBusy.current = true; setError("");
    try {
      const publication = await current.room.localParticipant.setMicrophoneEnabled(!current.room.localParticipant.isMicrophoneEnabled);
      if (current.controller.signal.aborted) { publication?.track?.stop(); await current.disconnect(); return; }
      refreshPeople(current.room);
    } catch { setError("マイクを使用できません。ブラウザーの許可をご確認ください。聞くだけの参加は続けられます。"); }
    finally { micBusy.current = false; }
  }
  async function report() {
    if (!reportTarget || reportBusy) return;
    setReportBusy(true);
    try {
      await apiFetch("/v1/reports", { method: "POST", token, body: JSON.stringify({ target_type: "voice_participant", target_id: reportTarget.id, reason, detail }) });
      setReported((ids) => [...ids, reportTarget.id]); setReportTarget(null); setDetail(""); setNotice("通報を受け付けました。");
    } catch (e) { setError(apiErrorText(e, "通報できませんでした")); }
    finally { setReportBusy(false); }
  }

  const active = phase === "connected" || phase === "reconnecting";
  const local = people.find((p) => p.local);
  if (token && status && !status.enabled && !active && phase !== "joining") return null;
  return <section aria-label="実況ルームの音声通話" className="space-y-3 rounded-2xl border border-sky-300/30 bg-sky-950/35 p-4">
    <div className="flex flex-wrap items-center justify-between gap-2"><h2 className="font-bold">音声通話 <span className="text-sm font-normal text-slate-300">{active ? people.length : status?.participants ?? 0}/4人</span></h2><span className="text-xs text-slate-300">{phase === "reconnecting" ? "再接続中…" : active ? "参加中" : "聞くだけでも参加できます"}</span></div>
    <div ref={host} className="hidden" aria-hidden="true" />
    {!token ? <p className="text-sm"><Link href="/login" className="text-sky-300 underline">ログインして参加</Link></p> : !active ? <Button onClick={() => void join()} disabled={phase === "joining" || !status?.enabled || status.full}>{phase === "joining" ? "接続中…" : status?.full ? "満員です" : "通話に参加"}</Button> : <>
      <ul className="grid grid-cols-2 gap-2" aria-label="通話の参加者">{Array.from({ length: 4 }, (_, i) => {
        const person = people[i];
        return <li key={person?.id ?? i} className={`min-w-0 rounded-xl border p-3 ${person?.speaking && !person.muted ? "border-emerald-300 bg-emerald-500/15" : "border-white/15 bg-white/5"}`}>
          {person ? <><p className="break-words text-sm font-semibold">{person.name}{person.local ? "（自分）" : ""}</p><p className="mt-1 text-xs text-slate-300">{person.muted ? "マイクOFF" : person.speaking ? "発言中" : "マイクON"}</p>{!person.local ? <button type="button" className="mt-2 text-xs text-slate-300 underline" onClick={() => { setReportTarget(person); setError(""); }} disabled={reported.includes(person.id)}>{reported.includes(person.id) ? "通報済み" : "通報"}</button> : null}</> : <p className="text-xs text-slate-400">空いています</p>}
        </li>;
      })}</ul>
      <div className="flex flex-wrap gap-2"><Button onClick={() => void toggleMic()} disabled={phase === "reconnecting"} variant={local?.muted ? "secondary" : "primary"}>{local?.muted ? "マイクをON" : "マイクをOFF"}</Button><Button variant="danger" onClick={() => void leave()}>退出</Button></div>
      <label className="flex items-center gap-3 text-sm">通話の音量<input className="min-w-0 flex-1" type="range" min="0" max="100" value={volume} aria-label="通話の音量" onChange={(e) => { const value = Number(e.target.value); setVolume(value); volumeRef.current = value; for (const p of connection.current?.room.remoteParticipants.values() ?? []) p.setVolume(value / 100); }} /><span className="w-9 text-right text-xs">{volume}%</span></label>
    </>}
    {audioBlocked && active ? <Button variant="secondary" onClick={() => { void connection.current?.room.startAudio().then(() => setAudioBlocked(false)).catch(() => setError("音声を再生できませんでした。もう一度お試しください。")); }}>音声を再生する</Button> : null}
    {confirmNeeded && active ? <div role="alert" className="rounded-xl bg-amber-500/15 p-3 text-sm"><p>1人での接続が続いています。続ける場合は1分以内に確認してください。</p><Button onClick={() => void heartbeat(true)}>通話を続ける</Button></div> : null}
    {reportTarget ? <form className="space-y-2 rounded-xl border border-white/20 p-3" onSubmit={(e) => { e.preventDefault(); void report(); }}><p className="text-sm font-semibold">{reportTarget.name}さんを通報</p><p className="text-xs text-slate-300">音声は保存していないため、後から再生できません。</p><label className="block text-sm">理由<select className="ml-2 rounded bg-slate-800 p-2" value={reason} onChange={(e) => setReason(e.target.value)}><option value="harassment">迷惑行為・嫌がらせ</option><option value="spam">スパム</option><option value="misinfo">誤った情報</option><option value="other">その他</option></select></label><label className="block text-sm">状況（任意）<textarea value={detail} onChange={(e) => setDetail(e.target.value)} maxLength={500} className="mt-1 w-full rounded bg-slate-800 p-2" /></label><div className="flex gap-2"><Button disabled={reportBusy} type="submit">通報する</Button><Button variant="secondary" type="button" onClick={() => setReportTarget(null)}>閉じる</Button></div></form> : null}
    {error ? <p role="alert" className="text-sm text-red-300">{error}</p> : null}{notice ? <p role="status" className="text-sm text-slate-300">{notice}</p> : null}
    <p className="text-xs leading-relaxed text-slate-300">入室時はマイクOFFです。イヤホンの利用をおすすめします。配信の視聴位置は同期しません。画面を離れたら退出し、画面ロック・別アプリでは切断される場合があります。</p>
  </section>;
}
