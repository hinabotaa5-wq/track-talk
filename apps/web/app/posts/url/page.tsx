"use client";

import { useState, type FormEvent } from "react";
import { loginPathFrom } from "@/lib/safe-next";
import { useRouter } from "next/navigation";
import { useAuth } from "@/components/auth-provider";
import { apiFetch, apiErrorText } from "@/lib/api";
import { ComposerPage } from "@/components/composer-page";
import { Button } from "@/components/ui/button";
import { SubmittingProgress } from "@/components/submitting-progress";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { detectVideoUrl } from "@/lib/video-url";
import type { Post } from "@/lib/types";
import { clearHomeFeeds } from "@/lib/home-feed-cache";
import { useActionLock } from "@/lib/use-action-lock";
import { shrinkImage } from "@/lib/shrink-image";
import { uploadToCloudinary, type CloudinarySignature } from "@/lib/cloudinary-upload";

const maxCoverBytes = 10 * 1024 * 1024;

const urlError = "YouTubeの動画、Instagramの投稿・リール、Xの投稿のURLを貼ってください";
const kindLabel = { youtube: "YouTube", instagram: "Instagram", x: "X" } as const;

export default function VideoUrlPostPage() {
  const { session, token } = useAuth();
  const router = useRouter();
  const [url, setUrl] = useState("");
  const [body, setBody] = useState("");
  const [error, setError] = useState("");
  // Instagram の表紙は外から確実に取れないので、投稿者が任意で画像を付けられる。付けるとフィードのカードに出る。
  const [cover, setCover] = useState<File | null>(null);
  const [status, setStatus] = useState("");
  const { pending: submitting, acquire, release } = useActionLock();
  const kind = detectVideoUrl(url);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    if (!session) {
      router.push(loginPathFrom(window.location));
      return;
    }
    if (!kind) {
      setError(urlError);
      return;
    }
    if (!acquire()) return;
    setError("");
    try {
      let imagePublicId: string | undefined;
      if ((kind === "instagram" || kind === "x") && cover) {
        setStatus("送信しています…");
        const sending = await shrinkImage(cover);
        if (sending.size > maxCoverBytes) throw new Error("10MB以内の画像にしてください");
        const signed = await apiFetch<CloudinarySignature>("/v1/uploads/image", { method: "POST", token });
        const result = await uploadToCloudinary("image", sending, signed, () => {});
        if (result.public_id !== signed.public_id) throw new Error("サムネの画像を送れませんでした");
        imagePublicId = result.public_id;
        setStatus("");
      }
      const res = await apiFetch<{ post: Post; existing: boolean }>(`/v1/posts/${kind}`, {
        method: "POST",
        token,
        body: JSON.stringify({ url: url.trim(), body, ...(imagePublicId ? { image_public_id: imagePublicId } : {}) }),
      });
      // 同じ動画のスレッドが既にあるときだけ、そのスレッドの詳細を開く。新しく立てたときはホームに戻る。
      if (res.existing) {
        router.push(`/posts/${res.post.id}`);
        return;
      }
      clearHomeFeeds();
      router.push("/?sort=new");
    } catch (err) {
      setError(apiErrorText(err, err instanceof Error ? err.message : "保存に失敗しました"));
      setStatus("");
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
            placeholder="YouTube・Instagram・X の URL"
            value={url}
            onChange={(e) => setUrl(e.target.value)}
          />
        </label>
        {url.trim() === "" ? null : kind ? (
          <p className="text-sm text-slate-200">{kindLabel[kind]}のURLとして引用します</p>
        ) : (
          <p className="text-sm text-red-400">{urlError}</p>
        )}
        {kind === "instagram" || kind === "x" ? (
          <label className="block text-sm font-medium">
            サムネの画像（任意）
            <span className="mt-0.5 block text-xs font-normal text-slate-300">
              {kindLabel[kind]}のサムネはこちらで読み込めないことがあります。スクリーンショットなどを付けると、一覧のカードに出ます。
            </span>
            <input
              className="mt-1 block w-full text-sm text-slate-300 file:mr-3 file:rounded-full file:border-0 file:bg-white/15 file:px-3 file:py-2 file:text-sm file:font-semibold file:text-slate-100"
              type="file"
              accept="image/jpeg,image/png,image/webp,image/gif,image/heic,image/heif"
              onChange={(e) => setCover(e.target.files?.[0] ?? null)}
              disabled={submitting}
            />
          </label>
        ) : null}
        <label className="block text-sm font-medium">
          詳細
          <Textarea rows={3} className="mt-1" value={body} onChange={(e) => setBody(e.target.value)} />
        </label>
        {submitting ? <SubmittingProgress label="投稿を送信しています" /> : null}
        {status ? <p className="text-sm text-slate-200">{status}</p> : null}
        {error ? <p className="text-sm text-red-400">{error}</p> : null}
        <Button type="submit" className="w-full" disabled={submitting || !kind}>
          {submitting ? "送っています" : "投稿する"}
        </Button>
      </form>
    </ComposerPage>
  );
}
