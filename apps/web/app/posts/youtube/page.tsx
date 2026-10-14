"use client";

import { useState, type FormEvent } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/components/auth-provider";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { ComposerPage, TitleField } from "@/components/composer-page";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { isYouTubeUrl } from "@/lib/validators";
import type { Post } from "@/lib/types";
import { clearHomeFeeds } from "@/lib/home-feed-cache";
import { useActionLock } from "@/lib/use-action-lock";

export default function YouTubePostPage() {
  const { session, token } = useAuth();
  const router = useRouter();
  const [url, setUrl] = useState("");
  const [title, setTitle] = useState("");
  const [body, setBody] = useState("");
  const [error, setError] = useState("");
  const { pending: submitting, acquire, release } = useActionLock();

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    if (!session) {
      router.push("/login");
      return;
    }
    if (!isYouTubeUrl(url)) {
      setError("YouTubeのURLを貼ってください");
      return;
    }
    if (!acquire()) return;
    setError("");
    try {
      const data = await apiFetch<{ post: Post; existing: boolean }>("/v1/posts/youtube", {
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
    <ComposerPage title="YouTubeのURL">
      <form onSubmit={onSubmit} className="space-y-4">
        <label className="block text-sm font-medium">
          URL
          <Input
            className="mt-1"
            type="url"
            inputMode="url"
            placeholder="https://www.youtube.com/watch?v="
            value={url}
            onChange={(e) => setUrl(e.target.value)}
          />
        </label>
        {url.trim() !== "" && !isYouTubeUrl(url) ? (
          <p className="text-sm text-red-400">YouTubeのURLを貼ってください</p>
        ) : null}
        <TitleField value={title} onChange={(e) => setTitle(e.target.value)} />
        <label className="block text-sm font-medium">
          詳細
          <Textarea rows={3} className="mt-1" value={body} onChange={(e) => setBody(e.target.value)} />
        </label>
        {error ? <p className="text-sm text-red-400">{error}</p> : null}
        <Button type="submit" className="w-full" disabled={submitting || !isYouTubeUrl(url)}>
          {submitting ? "開いています" : "この動画のスレッドを開く"}
        </Button>
      </form>
    </ComposerPage>
  );
}
