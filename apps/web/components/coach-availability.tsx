"use client";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { apiFetch, apiErrorText } from "@/lib/api";
import { availabilityDateLabel, availabilityDays, availabilityLabels, tokyoDate, type AvailabilityDay, type AvailabilityStatus, type CoachAvailability } from "@/lib/coach-availability";
import { useActionLock } from "@/lib/use-action-lock";
import { useEffect, useState } from "react";

const dateColor = (weekday: number) => weekday === 0 ? "text-red-400" : weekday === 6 ? "text-blue-400" : "text-slate-200";
const guidance = "日程はメッセージで相談して決めます。選択だけでは確定しません。";

function useTokyoToday() {
  const [today, setToday] = useState(tokyoDate);
  useEffect(() => {
    let timer: ReturnType<typeof setTimeout>;
    const schedule = () => {
      const remaining = 86400000 - ((Date.now() + 9 * 3600000) % 86400000);
      timer = setTimeout(() => { setToday(tokyoDate()); schedule(); }, remaining + 20);
    };
    const refresh = () => setToday(tokyoDate());
    schedule();
    document.addEventListener("visibilitychange", refresh);
    return () => { clearTimeout(timer); document.removeEventListener("visibilitychange", refresh); };
  }, []);
  return today;
}

export function AvailabilityGrid({ today, days, selected = [], onSelect, editing = false, disabled = false }: CoachAvailability & {
  selected?: string[]; onSelect?: (date: string) => void; editing?: boolean; disabled?: boolean;
}) {
  const startWeekday = new Date(`${today}T00:00:00Z`).getUTCDay();
  return <div className="space-y-2">
    <p className="text-xs text-slate-300">{availabilityDateLabel(today)}から3週間・日本時間</p>
    <div className="grid grid-cols-7 gap-1" aria-label="3週間の空き状況">
      {Array.from({ length: 7 }, (_, i) => (startWeekday + i) % 7).map((weekday) => <div key={weekday} className={`text-center text-xs ${dateColor(weekday)}`}>{"日月火水木金土"[weekday]}</div>)}
      {availabilityDays(today, days).map((day) => {
        const date = new Date(`${day.date}T00:00:00Z`);
        const picked = selected.includes(day.date);
        const selectable = editing || day.status === "available" || day.status === "consult";
        const content = <>
          <span className={`block text-[11px] ${dateColor(date.getUTCDay())}`}>{date.getUTCMonth() + 1}/{date.getUTCDate()}</span>
          <span className="block text-xl leading-6">{availabilityLabels[day.status].split(" ")[0]}</span>
          {day.note ? <span className="mt-0.5 block break-all text-[10px] leading-tight text-slate-200">{day.note}</span> : null}
        </>;
        const style = `min-h-16 min-w-0 rounded-lg border px-0.5 py-1 text-center ${picked ? "border-blue-400 bg-blue-500/30 text-white" : "border-white/15 bg-white/5 text-slate-100"}`;
        return onSelect ? <button key={day.date} type="button" aria-label={`${availabilityDateLabel(day.date)} ${availabilityLabels[day.status]}${day.note ? ` ${day.note}` : ""}`} aria-pressed={picked}
          disabled={disabled || !selectable || (!editing && !picked && selected.length >= 3)} onClick={() => onSelect(day.date)} className={`${style} disabled:opacity-50`}>{content}</button>
          : <div key={day.date} className={style}>{content}</div>;
      })}
    </div>
    <p className="text-xs text-slate-300">○ 空きあり　△ 要相談　× 不可　− 未設定</p>
  </div>;
}

// コーチ共通のカレンダー。プラン詳細では希望日を選ぶ欄にも使う。
export function PublicAvailability({ coachID, selected = [], onChange }: { coachID: string; selected?: string[]; onChange?: (dates: string[]) => void }) {
  const [data, setData] = useState<CoachAvailability | null>(null);
  const [error, setError] = useState("");
  useEffect(() => {
    let cancelled = false;
    apiFetch<CoachAvailability>(`/v1/coaches/${coachID}/availability`).then((value) => { if (!cancelled) { setData(value); setError(""); } })
      .catch(() => { if (!cancelled) setError("空き状況を読み込めませんでした"); });
    return () => { cancelled = true; };
  }, [coachID]);
  // ページを開いたまま日付をまたいでも、過去の日は出さない。
  const currentDay = useTokyoToday();
  const today = data && data.today > currentDay ? data.today : currentDay;
  useEffect(() => {
    if (onChange && selected.some((date) => date < today)) onChange(selected.filter((date) => date >= today));
  }, [today, selected, onChange]);
  const toggle = onChange ? (date: string) => onChange(selected.includes(date) ? selected.filter((d) => d !== date) : selected.length < 3 ? [...selected, date] : selected) : undefined;
  return <section className="space-y-3 rounded-xl border border-white/20 bg-white/5 p-3" aria-label={onChange ? "希望日の選択" : "コーチの空き状況"}>
    <h2 className="text-sm font-semibold">{onChange ? "希望日（任意・3つまで）" : "コーチの空き状況"}</h2>
    {error ? <p role="alert" className="text-sm text-red-400">{error}</p> : data ? <AvailabilityGrid today={today} days={data.days} selected={selected} onSelect={toggle} /> : <p className="text-sm text-slate-300">読み込み中...</p>}
    {onChange ? <p className="text-xs text-slate-200">○か△の日を選べます。{selected.length}/3日選択中です。</p> : null}
    <p className="text-xs text-slate-300">{guidance}</p>
  </section>;
}

export function CoachAvailabilityEditor({ token }: { token: string | null }) {
  const [data, setData] = useState<CoachAvailability | null>(null);
  const [selected, setSelected] = useState("");
  const [error, setError] = useState("");
  const [saved, setSaved] = useState(false);
  const { pending, acquire, release } = useActionLock();
  useEffect(() => {
    let cancelled = false;
    apiFetch<CoachAvailability>("/v1/coach/me/availability", { token }).then((value) => { if (!cancelled) { setData(value); setSelected(value.today); setError(""); } })
      .catch((err) => { if (!cancelled) setError(apiErrorText(err, "空き状況を読み込めませんでした")); });
    return () => { cancelled = true; };
  }, [token]);
  const currentDay = useTokyoToday();
  const today = data && data.today > currentDay ? data.today : currentDay;
  const days = data ? availabilityDays(today, data.days) : [];
  const day = days.find((d) => d.date === selected) ?? days[0];
  function update(change: Partial<AvailabilityDay>) {
    if (!data || !day) return;
    setSaved(false);
    setData({ ...data, days: days.map((d) => d.date === day.date ? { ...d, ...change } : d) });
  }
  async function save() {
    if (!data || !acquire()) return;
    setError(""); setSaved(false);
    try {
      const result = await apiFetch<CoachAvailability>("/v1/coach/me/availability", { method: "PUT", token, body: JSON.stringify({ days }) });
      setData(result); setSaved(true);
    } catch (err) { setError(apiErrorText(err, "保存できませんでした")); }
    finally { release(); }
  }
  return <section className="space-y-3 rounded-2xl border border-white/20 bg-white/10 p-4" aria-label="空き状況の入力">
    <h2 className="font-bold">空き日カレンダー</h2>
    <p className="text-xs text-slate-300">日を選んで空き状況を入力してください。すべての対面指導・大会サポートのプランで共通です。</p>
    {data && day ? <>
      <AvailabilityGrid today={today} days={days} selected={[day.date]} onSelect={(date) => { setSelected(date); setSaved(false); }} editing disabled={pending} />
      <fieldset disabled={pending} className="space-y-3 rounded-xl border border-white/15 p-3">
        <legend className="text-sm font-semibold">{availabilityDateLabel(day.date)}</legend>
        <div className="grid grid-cols-2 gap-2">
          {(Object.keys(availabilityLabels) as AvailabilityStatus[]).map((status) => <button key={status} type="button" aria-pressed={day.status === status} onClick={() => update({ status, ...(status === "" ? { note: "" } : {}) })}
            className={`min-h-11 rounded-lg border px-2 text-sm ${day.status === status ? "border-blue-400 bg-blue-500/30" : "border-white/20 bg-white/5"}`}>{availabilityLabels[status]}</button>)}
        </div>
        <label className="block space-y-1 text-sm">メモ（任意・30文字まで）
          <Input aria-label="空き状況のメモ" maxLength={30} disabled={!day.status} value={day.note} onChange={(e) => update({ note: e.target.value })} placeholder="例：午後のみ" />
        </label>
      </fieldset>
      <Button type="button" className="w-full" disabled={pending} onClick={() => void save()}>空き状況を保存する</Button>
    </> : !error ? <p className="text-sm text-slate-300">読み込み中...</p> : null}
    {saved ? <p role="status" className="text-sm text-slate-200">保存しました</p> : null}
    {error ? <p role="alert" className="text-sm text-red-400">{error}</p> : null}
  </section>;
}
