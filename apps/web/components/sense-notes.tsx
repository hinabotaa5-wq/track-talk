"use client";

import { useState, type FormEvent } from "react";
import { Lock, Pencil, Trash2 } from "lucide-react";
import { Button } from "./ui/button";
import { Textarea } from "./ui/textarea";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { appEvents } from "@/lib/practice-events";
import { useActionLock } from "@/lib/use-action-lock";
import { senseCheer, senseNoteMaxLength, senseNotePlaceholder, type SenseNote, type SenseNoteStats } from "@/lib/sense-notes";

const selectClass =
  "w-full rounded-md border border-white/25 glass bg-white/10 px-3 py-2 text-base text-slate-100 outline-none focus:border-blue-400";

function errorText(e: unknown, fallback: string) {
  if (e instanceof ApiRequestError) {
    const field = e.fields ? Object.values(e.fields)[0] : "";
    return field || e.message;
  }
  return fallback;
}

function SenseNoteFields({
  body,
  event,
  onBody,
  onEvent,
  autoFocus = false,
}: {
  body: string;
  event: string;
  onBody: (v: string) => void;
  onEvent: (v: string) => void;
  autoFocus?: boolean;
}) {
  const length = [...body.trim()].length;
  return (
    <div className="space-y-2">
      <label className="block">
        <span className="sr-only">今日の練習で感じた感覚</span>
        <Textarea
          value={body}
          onChange={(e) => onBody(e.target.value)}
          placeholder={senseNotePlaceholder}
          rows={3}
          maxLength={senseNoteMaxLength + 20}
          autoFocus={autoFocus}
        />
      </label>
      <div className="flex items-center gap-3">
        <label className="min-w-0 flex-1">
          <span className="sr-only">種目(任意)</span>
          <select className={selectClass} value={event} onChange={(e) => onEvent(e.target.value)}>
            <option value="">種目(任意)</option>
            {appEvents.map((item) => (
              <option key={item.value} value={item.value}>
                {item.label}
              </option>
            ))}
          </select>
        </label>
        <span className={`shrink-0 text-xs tabular-nums ${length > senseNoteMaxLength ? "text-red-300" : "text-slate-300"}`}>
          {length}/{senseNoteMaxLength}
        </span>
      </div>
    </div>
  );
}

// 今日の感覚を書く欄。書けたら、ねぎらいの言葉を出す。
export function SenseNoteComposer({
  token,
  onCreated,
}: {
  token: string | null;
  onCreated: (note: SenseNote, stats: SenseNoteStats) => void;
}) {
  const [body, setBody] = useState("");
  const [event, setEvent] = useState("");
  const [error, setError] = useState("");
  const [cheer, setCheer] = useState<{ title: string; body: string } | null>(null);
  const lock = useActionLock();

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    const text = body.trim();
    if (!text) {
      setError("感じたことをひとこと書いてください");
      return;
    }
    if ([...text].length > senseNoteMaxLength) {
      setError(`${senseNoteMaxLength}文字までです`);
      return;
    }
    if (!lock.acquire()) return;
    setError("");
    try {
      const res = await apiFetch<{ note: SenseNote; stats: SenseNoteStats }>("/v1/me/sense-notes", {
        method: "POST",
        token,
        body: JSON.stringify({ body: text, event: event || null }),
      });
      setBody("");
      setEvent("");
      setCheer(senseCheer(res.stats));
      onCreated(res.note, res.stats);
    } catch (err) {
      setError(errorText(err, "保存できませんでした"));
    } finally {
      lock.release();
    }
  }

  if (cheer) {
    return (
      <div role="status" className="sense-cheer space-y-3 rounded-xl border border-sky-300/40 bg-sky-950/40 p-4 text-center">
        <p className="text-lg font-bold text-sky-100">{cheer.title}</p>
        <p className="text-sm text-sky-100/90">{cheer.body}</p>
        <Button variant="secondary" onClick={() => setCheer(null)}>
          もうひとつ書く
        </Button>
      </div>
    );
  }

  return (
    <form onSubmit={onSubmit} className="space-y-2">
      <SenseNoteFields body={body} event={event} onBody={setBody} onEvent={setEvent} />
      {error ? <p className="text-sm text-red-300">{error}</p> : null}
      <div className="flex items-center justify-between gap-3">
        <p className="flex items-center gap-1 text-xs text-slate-300">
          <Lock className="size-3.5" aria-hidden />
          あなただけに見えます
        </p>
        <Button type="submit" disabled={lock.pending}>
          {lock.pending ? "ためています…" : "感覚をためる"}
        </Button>
      </div>
    </form>
  );
}

// 一覧の1件。本人だけが見るので、編集と削除をその場で出す。
export function SenseNoteItem({
  note,
  token,
  onUpdated,
  onDeleted,
}: {
  note: SenseNote;
  token: string | null;
  onUpdated: (note: SenseNote) => void;
  onDeleted: (id: string, stats: SenseNoteStats) => void;
}) {
  const [editing, setEditing] = useState(false);
  const [body, setBody] = useState(note.body);
  const [event, setEvent] = useState(note.event ?? "");
  const [error, setError] = useState("");
  const lock = useActionLock();

  async function save(e: FormEvent) {
    e.preventDefault();
    const text = body.trim();
    if (!text || [...text].length > senseNoteMaxLength) {
      setError(`1〜${senseNoteMaxLength}文字で書いてください`);
      return;
    }
    if (!lock.acquire()) return;
    setError("");
    try {
      const updated = await apiFetch<SenseNote>(`/v1/me/sense-notes/${note.id}`, {
        method: "PATCH",
        token,
        body: JSON.stringify({ body: text, event: event || null }),
      });
      onUpdated(updated);
      setEditing(false);
    } catch (err) {
      setError(errorText(err, "保存できませんでした"));
    } finally {
      lock.release();
    }
  }

  async function remove() {
    if (!confirm("この感覚メモを削除しますか？")) return;
    if (!lock.acquire()) return;
    try {
      const res = await apiFetch<{ stats: SenseNoteStats }>(`/v1/me/sense-notes/${note.id}`, { method: "DELETE", token });
      onDeleted(note.id, res.stats);
    } catch (err) {
      alert(errorText(err, "削除できませんでした"));
      lock.release();
    }
  }

  if (editing) {
    return (
      <form onSubmit={save} className="space-y-2 rounded-xl border border-white/25 glass bg-white/12 p-3">
        <SenseNoteFields body={body} event={event} onBody={setBody} onEvent={setEvent} autoFocus />
        {error ? <p className="text-sm text-red-300">{error}</p> : null}
        <div className="flex justify-end gap-2">
          <Button
            type="button"
            variant="ghost"
            onClick={() => {
              setBody(note.body);
              setEvent(note.event ?? "");
              setError("");
              setEditing(false);
            }}
          >
            やめる
          </Button>
          <Button type="submit" disabled={lock.pending}>
            保存する
          </Button>
        </div>
      </form>
    );
  }

  return (
    <div className="flex items-start gap-3 rounded-xl border border-white/20 glass bg-white/10 p-3">
      <div className="min-w-0 flex-1 space-y-1.5">
        {note.event_label ? (
          <span className="inline-block rounded-full border border-sky-300/40 bg-sky-400/15 px-2.5 py-0.5 text-xs font-semibold text-sky-100">
            {note.event_label}
          </span>
        ) : null}
        <p className="whitespace-pre-wrap break-words text-sm leading-relaxed text-slate-100">{note.body}</p>
      </div>
      <div className="flex shrink-0 gap-1">
        <button
          type="button"
          onClick={() => setEditing(true)}
          disabled={lock.pending}
          aria-label="編集"
          className="inline-flex size-9 items-center justify-center rounded-full text-slate-300 transition hover:bg-white/16 hover:text-white active:scale-95 disabled:opacity-50"
        >
          <Pencil className="size-4" aria-hidden />
        </button>
        <button
          type="button"
          onClick={remove}
          disabled={lock.pending}
          aria-label="削除"
          className="inline-flex size-9 items-center justify-center rounded-full text-red-300 transition hover:bg-red-950/40 active:scale-95 disabled:opacity-50"
        >
          <Trash2 className="size-4" aria-hidden />
        </button>
      </div>
    </div>
  );
}

export function SenseNoteStatsRow({ stats }: { stats: SenseNoteStats }) {
  const items = [
    { label: "感覚", value: stats.total, unit: "個" },
    { label: "連続", value: stats.streak, unit: "日" },
    { label: "書いた日", value: stats.days, unit: "日" },
  ];
  return (
    <dl className="grid grid-cols-3 gap-2">
      {items.map((item) => (
        <div key={item.label} className="rounded-xl border border-white/15 bg-white/8 px-2 py-2 text-center">
          <dt className="text-[11px] text-slate-300">{item.label}</dt>
          <dd className="font-bold text-slate-50">
            <span className="text-xl tabular-nums">{item.value}</span>
            <span className="ml-0.5 text-xs font-medium text-slate-300">{item.unit}</span>
          </dd>
        </div>
      ))}
    </dl>
  );
}
