"use client";

import { useState, type FormEvent } from "react";
import { useRouter } from "next/navigation";
import { injuryBodyParts, injuryMoods, injuryNotice, isInjuryBodyPart, isInjuryMood } from "@/lib/injury";
import { Input } from "./ui/input";
import { Textarea } from "./ui/textarea";
import { Button } from "./ui/button";
import type { Post } from "@/lib/types";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { useActionLock } from "@/lib/use-action-lock";
import { clearHomeFeeds } from "@/lib/home-feed-cache";
import { useAuth } from "./auth-provider";

const selectClass =
  "mt-1 w-full rounded-md border border-white/25 glass bg-white/10 px-3 py-2 text-base text-slate-100 outline-none focus:border-blue-400";

export function InjuryForm({ initial, returnTo }: { initial?: Post; returnTo?: "me" }) {
  const { token, session } = useAuth();
  const router = useRouter();
  const [mood, setMood] = useState<string>(initial?.injury?.mood ?? "");
  const [part, setPart] = useState(initial?.injury?.body_part ?? "");
  const [title, setTitle] = useState(initial?.title ?? "");
  const [body, setBody] = useState(initial?.body ?? "");
  const [error, setError] = useState("");
  const { pending, acquire, release } = useActionLock();

  async function onSubmit(event: FormEvent) {
    event.preventDefault();
    if (!session) {
      router.push("/login");
      return;
    }
    setError("");
    if (!isInjuryMood(mood)) {
      setError("投稿の種類を選んでください");
      return;
    }
    if (!isInjuryBodyPart(part)) {
      setError("部位を選んでください");
      return;
    }
    if (body.trim() === "") {
      setError("いまの気持ちや状況を入力してください");
      return;
    }
    if (!acquire()) return;
    try {
      const payload = { kind: "injury", title: title.trim(), body: body.trim(), mood, body_part: part };
      const post = initial
        ? await apiFetch<Post>(`/v1/posts/${initial.id}`, { method: "PATCH", token, body: JSON.stringify(payload) })
        : await apiFetch<Post>("/v1/posts", { method: "POST", token, body: JSON.stringify(payload) });
      clearHomeFeeds();
      router.push(
        initial ? (returnTo === "me" ? `/posts/${post.id}?from=me` : `/posts/${post.id}`) : "/injury",
      );
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
      <fieldset>
        <legend className="text-sm font-medium">投稿の種類</legend>
        <div className="mt-1 grid grid-cols-2 gap-2">
          {injuryMoods.map((item) => (
            <label
              key={item.value}
              className={`cursor-pointer rounded-xl border px-3 py-2.5 text-sm transition active:scale-95 ${
                mood === item.value
                  ? "border-blue-400 bg-blue-950/60 text-white"
                  : "border-white/20 glass bg-white/10 text-slate-200 hover:bg-white/16"
              }`}
            >
              <input
                type="radio"
                name="mood"
                value={item.value}
                checked={mood === item.value}
                onChange={() => setMood(item.value)}
                className="sr-only"
              />
              <span className="block font-semibold">{item.label}</span>
              <span className="mt-0.5 block text-xs text-slate-300">{item.hint}</span>
            </label>
          ))}
        </div>
      </fieldset>
      <label className="block text-sm font-medium">
        部位
        <select className={selectClass} value={part} onChange={(e) => setPart(e.target.value)}>
          <option value="">選んでください</option>
          {injuryBodyParts.map((item) => (
            <option key={item.value} value={item.value}>
              {item.label}
            </option>
          ))}
        </select>
      </label>
      <label className="block text-sm font-medium">
        タイトル(任意)
        <Input
          className="mt-1"
          maxLength={120}
          placeholder="空欄なら本文の冒頭が表示されます"
          value={title}
          onChange={(e) => setTitle(e.target.value)}
        />
      </label>
      <label className="block text-sm font-medium">
        本文
        <Textarea
          rows={8}
          className="mt-1"
          maxLength={5000}
          placeholder={
            mood === "rehab" ? "いまやっているトレーニングや基礎練を書いてみましょう" : "いまの気持ちをそのまま書いてOKです"
          }
          value={body}
          onChange={(e) => setBody(e.target.value)}
        />
      </label>
      <p className="text-xs text-slate-300">{injuryNotice}</p>
      {error ? <p className="text-sm text-red-400">{error}</p> : null}
      <Button type="submit" className="w-full" disabled={pending}>
        {pending ? (initial ? "更新しています" : "送っています") : initial ? "更新する" : "投稿する"}
      </Button>
    </form>
  );
}
