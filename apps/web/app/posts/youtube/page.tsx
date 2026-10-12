"use client";

import { useState, type FormEvent } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/components/auth-provider";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { ComposerPage, TitleField } from "@/components/composer-page";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import type { Post } from "@/lib/types";

export default function YouTubePostPage() {
  const { session, token } = useAuth();
  const router = useRouter();
  const [url, setUrl] = useState("");
  const [title, setTitle] = useState("");
  const [error, setError] = useState("");
  const [submitting, setSubmitting] = useState(false);

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

  return (
    <ComposerPage title="YouTubeのURL">
      <form onSubmit={onSubmit} className="space-y-4">
        <TitleField value={title} onChange={(e) => setTitle(e.target.value)} />
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
        <Button type="submit" className="w-full" disabled={submitting}>
          {submitting ? "開いています" : "この動画のスレッドを開く"}
        </Button>
      </form>
    </ComposerPage>
  );
}
