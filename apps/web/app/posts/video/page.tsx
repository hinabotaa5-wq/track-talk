"use client";

import Link from "next/link";
import { useEffect, useState, type FormEvent } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/components/auth-provider";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import type { Post } from "@/lib/types";

const maxBytes = 100 * 1024 * 1024;

type Signature = {
  cloud_name: string;
  api_key: string;
  timestamp: number;
  public_id: string;
  signature: string;
};

export default function OwnVideoPage() {
  const { session, loading, token } = useAuth();
  const router = useRouter();
  const [title, setTitle] = useState("");
  const [file, setFile] = useState<File | null>(null);
  const [error, setError] = useState("");
  const [status, setStatus] = useState("");
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
    if (!file) {
      setError("動画ファイルを選んでください");
      return;
    }
    if (file.size > maxBytes) {
      setError("100MB以内の動画にしてください");
      return;
    }
    setError("");
    setSubmitting(true);
    try {
      setStatus("アップロードの準備をしています…");
      const signed = await apiFetch<Signature>("/v1/uploads/video", { method: "POST", token });
      setStatus("Cloudinaryへ送っています…");
      const body = new FormData();
      body.append("file", file);
      body.append("api_key", signed.api_key);
      body.append("timestamp", String(signed.timestamp));
      body.append("public_id", signed.public_id);
      body.append("signature", signed.signature);
      const uploaded = await fetch(`https://api.cloudinary.com/v1_1/${signed.cloud_name}/video/upload`, {
        method: "POST",
        body,
      });
      const payload = (await uploaded.json()) as { public_id?: string; error?: { message?: string } };
      if (!uploaded.ok || payload.public_id !== signed.public_id) {
        throw new Error(payload.error?.message || "アップロードに失敗しました");
      }
      setStatus("投稿を保存しています…");
      const post = await apiFetch<Post>("/v1/posts", {
        method: "POST",
        token,
        body: JSON.stringify({
          title,
          body: "",
          kind: "video",
          video_public_id: payload.public_id,
        }),
      });
      router.push(`/posts/${post.id}`);
    } catch (err) {
      if (err instanceof ApiRequestError) {
        const field = err.fields ? Object.values(err.fields)[0] : "";
        setError(field || err.message);
      } else {
        setError(err instanceof Error ? err.message : "保存に失敗しました");
      }
      setStatus("");
    } finally {
      setSubmitting(false);
    }
  }

  if (loading || !session) return <p>読み込み中…</p>;

  return (
    <div className="mx-auto max-w-2xl space-y-4">
      <h1 className="text-2xl font-bold">自分の動画</h1>
      <p className="text-sm text-zinc-600">
        ファイルはCloudinaryに保存します。保存が終わってからホームに出ます。見るのにログインは要りません。
      </p>
      <form onSubmit={onSubmit} className="space-y-4">
        <label className="block text-sm font-medium">
          動画ファイル
          <input
            className="mt-1 block w-full text-base"
            type="file"
            accept="video/mp4,video/quicktime,video/webm"
            onChange={(e) => setFile(e.target.files?.[0] ?? null)}
            required
          />
        </label>
        <label className="block text-sm font-medium">
          タイトル（空なら（質問なし））
          <Input className="mt-1" value={title} onChange={(e) => setTitle(e.target.value)} maxLength={120} />
        </label>
        {status ? <p className="text-sm text-zinc-600">{status}</p> : null}
        {error ? <p className="text-sm text-red-600">{error}</p> : null}
        <Button type="submit" className="w-full md:w-auto" disabled={submitting}>
          アップロードして投稿
        </Button>
      </form>
      <Link href="/upload" className="text-sm text-zinc-500">
        投稿の入口に戻る
      </Link>
    </div>
  );
}
