"use client";

import { useState, type FormEvent } from "react";
import { useRouter } from "next/navigation";
import { practiceSchema } from "@/lib/validators";
import { practiceEvents } from "@/lib/practice-events";
import { Input } from "./ui/input";
import { Textarea } from "./ui/textarea";
import { Button } from "./ui/button";
import type { Post } from "@/lib/types";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { useActionLock } from "@/lib/use-action-lock";
import { clearHomeFeeds } from "@/lib/home-feed-cache";
import { useAuth } from "./auth-provider";

const fieldClass = "mt-1";
const selectClass =
  "mt-1 w-full rounded-md border border-white/25 glass bg-white/10 px-3 py-2 text-base text-slate-100 outline-none focus:border-blue-400";

function toLocalInput(iso: string) {
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return "";
  const pad = (value: number) => String(value).padStart(2, "0");
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

export function PracticeForm({ initial, returnTo }: { initial?: Post; returnTo?: "me" }) {
  const { token, session } = useAuth();
  const router = useRouter();
  const practice = initial?.practice;
  const [venue, setVenue] = useState(practice?.venue ?? "");
  const [startsAt, setStartsAt] = useState(practice ? toLocalInput(practice.starts_at) : "");
  const [event, setEvent] = useState(practice?.event ?? "");
  const [eventOther, setEventOther] = useState(practice?.event_other ?? "");
  const [capacity, setCapacity] = useState(practice ? String(practice.capacity) : "");
  const [body, setBody] = useState(initial?.body ?? "");
  const [error, setError] = useState("");
  const { pending: submitting, acquire, release } = useActionLock();

  async function onSubmit(formEvent: FormEvent) {
    formEvent.preventDefault();
    if (!session) {
      router.push("/login");
      return;
    }
    setError("");
    const parsed = practiceSchema.safeParse({
      venue,
      starts_at: startsAt,
      event,
      event_other: event === "other" ? eventOther : "",
      capacity: Number(capacity),
      body,
    });
    if (!parsed.success) {
      setError(parsed.error.issues[0]?.message ?? "入力を確認してください");
      return;
    }
    if (parsed.data.event === "other" && parsed.data.event_other === "") {
      setError("種目名を入力してください");
      return;
    }
    const when = new Date(parsed.data.starts_at);
    if (Number.isNaN(when.getTime())) {
      setError("練習日時を入力してください");
      return;
    }
    if (!initial && when.getTime() <= Date.now()) {
      setError("練習日時は現在より後にしてください");
      return;
    }
    if (!acquire()) return;
    try {
      const payload = {
        kind: "practice",
        body: parsed.data.body,
        venue: parsed.data.venue,
        starts_at: when.toISOString(),
        event: parsed.data.event,
        event_other: parsed.data.event === "other" ? parsed.data.event_other : "",
        capacity: parsed.data.capacity,
      };
      const post = initial
        ? await apiFetch<Post>(`/v1/posts/${initial.id}`, {
            method: "PATCH",
            token,
            body: JSON.stringify(payload),
          })
        : await apiFetch<Post>("/v1/posts", {
            method: "POST",
            token,
            body: JSON.stringify(payload),
          });
      clearHomeFeeds();
      router.push(initial ? (returnTo === "me" ? `/posts/${post.id}?from=me` : `/posts/${post.id}`) : "/");
    } catch (e) {
      if (e instanceof ApiRequestError) {
        const field = e.fields ? Object.values(e.fields)[0] : "";
        setError(field || e.message);
      } else {
        setError("保存に失敗しました");
      }
      release();
    }
  }

  return (
    <form onSubmit={onSubmit} className="space-y-4">
      <label className="block text-sm font-medium">
        練習場所
        <Input
          className={fieldClass}
          maxLength={80}
          placeholder="〇〇競技場"
          value={venue}
          onChange={(e) => setVenue(e.target.value)}
        />
      </label>
      <label className="block min-w-0 text-sm font-medium">
        練習日時
        <span className="mt-1 grid grid-cols-[minmax(0,1fr)]">
          <Input
            className="block w-full min-w-0 max-w-full"
            type="datetime-local"
            value={startsAt}
            min={initial ? undefined : toLocalInput(new Date().toISOString())}
            onChange={(e) => setStartsAt(e.target.value)}
          />
        </span>
      </label>
      <label className="block text-sm font-medium">
        種目
        <select className={selectClass} value={event} onChange={(e) => setEvent(e.target.value)}>
          <option value="">選んでください</option>
          {practiceEvents.map((item) => (
            <option key={item.value} value={item.value}>
              {item.label}
            </option>
          ))}
        </select>
      </label>
      {event === "other" ? (
        <label className="block text-sm font-medium">
          種目名
          <Input
            className={fieldClass}
            maxLength={40}
            placeholder="3000m"
            value={eventOther}
            onChange={(e) => setEventOther(e.target.value)}
          />
        </label>
      ) : null}
      <label className="block text-sm font-medium">
        募集人数
        <Input
          className={fieldClass}
          type="number"
          inputMode="numeric"
          min={1}
          max={99}
          placeholder="3"
          value={capacity}
          onChange={(e) => setCapacity(e.target.value)}
        />
      </label>
      <label className="block text-sm font-medium">
        詳細
        <Textarea rows={8} className="mt-1" maxLength={2000} value={body} onChange={(e) => setBody(e.target.value)} />
      </label>
      {error ? <p className="text-sm text-red-400">{error}</p> : null}
      <Button type="submit" className="w-full" disabled={submitting}>
        {submitting ? (initial ? "更新しています" : "送っています") : initial ? "更新する" : "投稿する"}
      </Button>
    </form>
  );
}
