"use client";

import { useState, type FormEvent } from "react";
import { useRouter } from "next/navigation";
import { injuryBodyPartLabel, injuryBodyParts, injuryGroupPath, isInjuryBodyPart } from "@/lib/injury";
import { Input } from "./ui/input";
import { Textarea } from "./ui/textarea";
import { Button } from "./ui/button";
import type { Post } from "@/lib/types";
import { apiFetch, apiErrorText } from "@/lib/api";
import { useActionLock } from "@/lib/use-action-lock";
import { clearHomeFeeds } from "@/lib/home-feed-cache";
import { useAuth } from "./auth-provider";

const selectClass =
  "mt-1 w-full rounded-md border border-white/25 glass bg-white/10 px-3 py-2 text-base text-slate-100 outline-none focus:border-blue-400";

// groupPart を渡すと、そのケガのグループへ、いまの段階として投稿する(ケガは固定)。
export function InjuryForm({ initial, returnTo, groupPart }: { initial?: Post; returnTo?: "me"; groupPart?: string }) {
  const { token, session } = useAuth();
  const router = useRouter();
  const [part, setPart] = useState(groupPart ?? initial?.injury?.body_part ?? "");
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
    if (!isInjuryBodyPart(part)) {
      setError("ケガを選んでください");
      return;
    }
    if (body.trim() === "") {
      setError("いまの気持ちや状況を入力してください");
      return;
    }
    if (!acquire()) return;
    try {
      const payload = { kind: "injury", title: title.trim(), body: body.trim(), body_part: part };
      const post = initial
        ? await apiFetch<Post>(`/v1/posts/${initial.id}`, { method: "PATCH", token, body: JSON.stringify(payload) })
        : await apiFetch<Post>(groupPart ? `/v1/injury/groups/${groupPart}/posts` : "/v1/posts", { method: "POST", token, body: JSON.stringify(payload) });
      clearHomeFeeds();
      // 編集のあとは編集画面を履歴から外し、ヘッダーの戻るで編集画面に戻らないようにする。
      (initial ? router.replace : router.push)(
        initial ? (returnTo === "me" ? `/posts/${post.id}?from=me` : `/posts/${post.id}`) : groupPart ? injuryGroupPath(groupPart) : "/injury",
      );
    } catch (e) {
      setError(apiErrorText(e, "保存に失敗しました"));
      release();
    }
  }

  return (
    <form onSubmit={onSubmit} className="space-y-4">
      {groupPart ? (
        <p className="text-sm font-medium">ケガ: {injuryBodyPartLabel(groupPart)}</p>
      ) : (
        <label className="block text-sm font-medium">
          ケガ
          <select className={selectClass} value={part} onChange={(e) => setPart(e.target.value)}>
            <option value="">選んでください</option>
            {injuryBodyParts.map((item) => (
              <option key={item.value} value={item.value}>
                {item.label}
              </option>
            ))}
          </select>
        </label>
      )}
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
          placeholder="いまの気持ちをそのまま書いてOKです"
          value={body}
          onChange={(e) => setBody(e.target.value)}
        />
      </label>
      {error ? <p className="text-sm text-red-400">{error}</p> : null}
      <Button type="submit" className="w-full" disabled={pending}>
        {pending ? (initial ? "更新しています" : "送っています") : initial ? "更新する" : "投稿する"}
      </Button>
    </form>
  );
}
