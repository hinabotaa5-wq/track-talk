"use client";

import { useState, type FormEvent } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/components/auth-provider";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { ComposerPage, TitleField } from "@/components/composer-page";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { detectVideoUrl } from "@/lib/video-url";
import type { Post } from "@/lib/types";
import { clearHomeFeeds } from "@/lib/home-feed-cache";
import { useActionLock } from "@/lib/use-action-lock";

const urlError = "YouTubeの動画か、Instagramの投稿・リールのURLを貼ってください";
const kindLabel = { youtube: "YouTube", instagram: "Instagram" } as const;

export default function VideoUrlPostPage() {
  const { session, token } = useAuth();
  const router = useRouter();
  const [url, setUrl] = useState("");
  const [title, setTitle] = useState("");
  const [body, setBody] = useState("");
  const [error, setError] = useState("");
  const { pending: submitting, acquire, release } = useActionLock();
  const kind = detectVideoUrl(url);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    if (!session) {
      router.push("/login");
      return;
    }
    if (!kind) {
      setError(urlError);
      return;
    }
    if (!acquire()) return;
    setError("");
    try {
      await apiFetch<{ post: Post; existing: boolean }>(`/v1/posts/${kind}`, {
        method: "POST",
        token,
        body: JSON.stringify({ url: url.trim(), title, body }),
      });
      clearHomeFeeds();
      router.push("/");
    } catch (err) {
      if (err instanceof ApiRequestError) {
        const field = err.fields ? Object.values(err.fields)[0] : "";
        setError(field || err.message);
      } else {
        setError("保存に失敗しました");
      }
      release();
    }
  }

  return (
    <ComposerPage title="動画URL">
      <form onSubmit={onSubmit} className="space-y-4">
        <label className="block text-sm font-medium">
          URL
          <Input
            className="mt-1"
            type="url"
            inputMode="url"
            placeholder="YouTube か Instagram の URL"
            value={url}
            onChange={(e) => setUrl(e.target.value)}
          />
        </label>
        {url.trim() === "" ? null : kind ? (
          <p className="text-sm text-slate-200">{kindLabel[kind]}のURLとして引用します</p>
        ) : (
          <p className="text-sm text-red-400">{urlError}</p>
        )}
        <TitleField value={title} onChange={(e) => setTitle(e.target.value)} required />
        <label className="block text-sm font-medium">
          詳細
          <Textarea rows={3} className="mt-1" value={body} onChange={(e) => setBody(e.target.value)} />
        </label>
        {error ? <p className="text-sm text-red-400">{error}</p> : null}
        <Button type="submit" className="w-full" disabled={submitting || !kind || !title.trim()}>
          {submitting ? "開いています" : "このスレッドを開く"}
        </Button>
      </form>
    </ComposerPage>
  );
}
