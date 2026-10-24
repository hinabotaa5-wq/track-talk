"use client";

import { useState } from "react";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { Button } from "@/components/ui/button";
import type { MeetResources } from "@/lib/meet-resources";

export function MeetResourceEditor({ meet, token, onSaved }: {
  meet: Pick<MeetResources, "meet_key"> & Partial<MeetResources>;
  token: string | null;
  onSaved: () => Promise<void>;
}) {
  const [official, setOfficial] = useState(meet.official_url ?? "");
  const [timetable, setTimetable] = useState(meet.timetable_url ?? "");
  const [youtube, setYoutube] = useState(meet.youtube_video_id ? `https://www.youtube.com/watch?v=${meet.youtube_video_id}` : "");
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  return <form className="space-y-3 rounded-lg border border-white/15 p-3" onSubmit={async (event) => {
    event.preventDefault();
    if (saving) return;
    setSaving(true); setMessage(""); setError("");
    try {
      await apiFetch("/v1/admin/meet-resources", { method: "PUT", token, body: JSON.stringify({ meet_key: meet.meet_key, official_url: official, timetable_url: timetable, youtube_url: youtube }) });
      await onSaved();
      setMessage("リンクと生配信を保存しました");
    } catch (err) {
      setError(err instanceof ApiRequestError && err.fields ? Object.values(err.fields).join(" / ") : "保存できませんでした");
    } finally { setSaving(false); }
  }}>
    <h3 className="font-semibold">大会リンク・生配信</h3>
    {[
      { label: "大会の公式サイトURL", value: official, change: setOfficial, placeholder: "https://…" },
      { label: "タイムテーブルURL", value: timetable, change: setTimetable, placeholder: "https://…（PDFへのリンクも使えます）" },
      { label: "YouTube生配信URL", value: youtube, change: setYoutube, placeholder: "https://www.youtube.com/live/…" },
    ].map((field) => <label className="block" key={field.label}>
      <span className="text-xs text-slate-300">{field.label}</span>
      <input type="url" value={field.value} disabled={saving} onChange={(event) => field.change(event.target.value)} placeholder={field.placeholder} className="mt-1 w-full rounded-lg border border-white/25 bg-white/10 px-2 py-2 text-white" />
    </label>)}
    <p className="text-xs text-slate-300">空欄でリンクや生配信を外せます。公式サイトが空欄なら大会情報のURLを使います。</p>
    <Button type="submit" disabled={saving}>{saving ? "保存中…" : "リンクを保存する"}</Button>
    {message ? <p role="status" className="text-emerald-300">{message}</p> : null}
    {error ? <p role="alert" className="text-red-300">{error}</p> : null}
  </form>;
}
