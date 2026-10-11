"use client";

import Link from "next/link";
import { useEffect, useState, type FormEvent } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/components/auth-provider";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import type { Post } from "@/lib/types";

export default function YouTubePostPage() {
  const { session, loading, token } = useAuth();
  const router = useRouter();
  const [url, setUrl] = useState("");
  const [title, setTitle] = useState("");
  const [error, setError] = useState("");
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    if (!loading && !session) router.replace("/login");
  }, [loading, session, router]);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    if (!session) {
      router.push("/login");
      return;
    }
    setError("");
    setSubmitting(true);
    try {
      const data = await apiFetch<{ post: Post; existing: boolean }>("/v1/posts/youtube", {
        method: "POST",
        token,
        body: JSON.stringify({ url, title }),
      });
      router.push(`/posts/${data.post.id}`);
    } catch (err) {
      if (err instanceof ApiRequestError) {
        const field = err.fields ? Object.values(err.fields)[0] : "";
        setError(field || err.message);
      } else {
        setError("保存に失敗しました");
      }
    } finally {
      setSubmitting(false);
    }
  }

  if (loading || !session) return <p>読み込み中…</p>;

  return (
    <div className="mx-auto max-w-2xl space-y-4">
      <h1 className="text-2xl font-bold">YouTubeのURL</h1>
      <p className="text-sm text-zinc-600">
        同じ動画のスレがあれば、それを開きます。タイトルを空にすると動画のタイトルを使います。新しい動画は、サムネイルが取れてからホームに出ます。
      </p>
      <form onSubmit={onSubmit} className="space-y-4">
        <label className="block text-sm font-medium">
          タイトル（空なら動画のタイトル）
          <Input
            className="mt-1"
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            maxLength={120}
          />
        </label>
        <label className="block text-sm font-medium">
          URL
          <Input
            className="mt-1"
            type="url"
            inputMode="url"
            placeholder="https://www.youtube.com/watch?v="
            value={url}
            onChange={(e) => setUrl(e.target.value)}
            required
          />
        </label>
        {error ? <p className="text-sm text-red-600">{error}</p> : null}
        <Button type="submit" className="w-full md:w-auto" disabled={submitting}>
          この動画のスレを開く
        </Button>
      </form>
      <Link href="/upload" className="text-sm text-zinc-500">
        投稿の入口に戻る
      </Link>
    </div>
  );
}
