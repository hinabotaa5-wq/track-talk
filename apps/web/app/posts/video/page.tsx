"use client";

import { useState, type FormEvent } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/components/auth-provider";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { ComposerPage, TitleField } from "@/components/composer-page";
import { UploadProgress } from "@/components/upload-progress";
import { Button } from "@/components/ui/button";
import { uploadToCloudinary, type CloudinarySignature, type CloudinaryUploadProgress } from "@/lib/cloudinary-upload";
import type { Post } from "@/lib/types";

const maxBytes = 100 * 1024 * 1024;

export default function OwnVideoPage() {
  const { session, token } = useAuth();
  const router = useRouter();
  const [title, setTitle] = useState("");
  const [file, setFile] = useState<File | null>(null);
  const [error, setError] = useState("");
  const [status, setStatus] = useState("");
  const [upload, setUpload] = useState<CloudinaryUploadProgress | null>(null);
  const [uploadPhase, setUploadPhase] = useState<"upload" | "saving">("upload");
  const [submitting, setSubmitting] = useState(false);

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
    setUpload(null);
    setSubmitting(true);
    try {
      setStatus("アップロードの準備をしています…");
      const signed = await apiFetch<CloudinarySignature>("/v1/uploads/video", { method: "POST", token });
      setStatus("");
      setUploadPhase("upload");
      setUpload({ loaded: 0, total: file.size, percent: 0 });
      const payload = await uploadToCloudinary("video", file, signed, setUpload);
      if (payload.public_id !== signed.public_id) {
        throw new Error("アップロードに失敗しました");
      }
      setUpload((current) => (current ? { ...current, loaded: current.total, percent: 100 } : current));
      setUploadPhase("saving");
      setStatus("");
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
      setUpload(null);
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <ComposerPage title="自分の動画">
      <form onSubmit={onSubmit} className="space-y-4">
        <TitleField value={title} onChange={(e) => setTitle(e.target.value)} />
        <label className="block text-sm font-medium">
          動画ファイル
          <input
            className="mt-1 block w-full text-base"
            type="file"
            accept="video/mp4,video/quicktime,video/webm"
            onChange={(e) => setFile(e.target.files?.[0] ?? null)}
            disabled={submitting}
            required
          />
        </label>
        {upload ? (
          <UploadProgress noun="動画" phase={uploadPhase} progress={upload} fileName={file?.name} fileSize={file?.size} />
        ) : null}
        {status ? <p className="text-sm text-zinc-600">{status}</p> : null}
        {error ? <p className="text-sm text-red-600">{error}</p> : null}
        <Button type="submit" className="w-full" disabled={submitting}>
          {submitting ? "送っています" : "アップロードして投稿"}
        </Button>
      </form>
    </ComposerPage>
  );
}
