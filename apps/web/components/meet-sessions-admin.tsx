"use client";

import { useCallback, useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import { ApiRequestError, apiFetch } from "@/lib/api";

type AdminMeet = { meet_key: string; name: string; date_label: string; session_count: number };

const placeholder = "開始日時,種目,ラウンド,性別\n2026-10-12 10:30,100m,予選,女子\n2026-10-12 11:05,走幅跳,決勝,男子";

// 管理者画面の「タイムテーブル」タブ。主要大会の種目ごとの開始時刻を貼り付けて、「いま実施中」の元にする。
export function MeetSessionsAdmin({ token }: { token: string | null }) {
  const [meets, setMeets] = useState<AdminMeet[] | null>(null);
  const [meetKey, setMeetKey] = useState("");
  const [csv, setCsv] = useState("");
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");

  const load = useCallback(async () => {
    try {
      const data = await apiFetch<{ meets: AdminMeet[] }>("/v1/admin/meet-sessions", { token });
      setMeets(data.meets);
      setMeetKey((current) => current || data.meets[0]?.meet_key || "");
      setError("");
    } catch {
      setError("読み込めませんでした");
    }
  }, [token]);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load();
  }, [load]);

  async function save() {
    setSaving(true);
    setMessage("");
    setError("");
    try {
      const res = await apiFetch<{ session_count: number }>("/v1/admin/meet-sessions", {
        method: "PUT",
        token,
        body: JSON.stringify({ meet_key: meetKey, csv }),
      });
      setMessage(`${res.session_count}件を保存しました`);
      await load();
    } catch (err) {
      if (err instanceof ApiRequestError && err.fields) {
        setError(
          Object.entries(err.fields)
            .map(([key, text]) => (key.startsWith("line_") ? `${key.slice(5)}行目: ${text}` : text))
            .join(" / "),
        );
      } else {
        setError("保存できませんでした");
      }
    } finally {
      setSaving(false);
    }
  }

  if (!meets) return error ? <p className="text-sm text-red-400">{error}</p> : <p className="text-sm text-slate-200">読み込み中...</p>;
  if (meets.length === 0) return <p className="text-sm text-slate-200">タイムテーブルを入れられる大会(これから開催か開催中の主要大会)はありません。</p>;

  return (
    <div className="space-y-3 rounded-xl border border-white/20 glass bg-white/10 p-3 text-sm text-slate-100">
      <p className="text-xs text-slate-300">
        貼り付けた内容で、その大会のタイムテーブルを置き換えます(空にすると消えます)。開始日時は日本時間で、列は
        「開始日時,種目,ラウンド,性別」です。ラウンドと性別は空でかまいません。表計算からの貼り付け(タブ区切り)もできます。
      </p>
      <label className="block">
        <span className="text-xs text-slate-300">大会</span>
        <select
          value={meetKey}
          onChange={(e) => {
            setMeetKey(e.target.value);
            setMessage("");
          }}
          className="mt-1 w-full rounded-lg border border-white/25 bg-white/10 px-2 py-2 text-white"
        >
          {meets.map((m) => (
            <option key={m.meet_key} value={m.meet_key} className="text-slate-900">
              {m.name}({m.date_label}・{m.session_count}件)
            </option>
          ))}
        </select>
      </label>
      <textarea
        value={csv}
        onChange={(e) => setCsv(e.target.value)}
        rows={10}
        placeholder={placeholder}
        className="w-full rounded-lg border border-white/25 bg-white/10 p-2 font-mono text-xs text-white placeholder:text-slate-400"
      />
      <Button type="button" onClick={save} disabled={saving || !meetKey}>
        {saving ? "保存中..." : "保存する"}
      </Button>
      {message ? <p className="text-emerald-300">{message}</p> : null}
      {error ? <p className="text-red-400">{error}</p> : null}
    </div>
  );
}
