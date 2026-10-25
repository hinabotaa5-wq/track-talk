"use client";

import { useRef, useState, type FormEvent } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/components/auth-provider";
import { apiFetch, ApiRequestError, apiErrorText } from "@/lib/api";
import { ComposerPage } from "@/components/composer-page";
import { SubmittingProgress } from "@/components/submitting-progress";
import { UploadProgress } from "@/components/upload-progress";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { uploadToCloudinary, type CloudinarySignature, type CloudinaryUploadProgress } from "@/lib/cloudinary-upload";
import type { Post } from "@/lib/types";
import { clearHomeFeeds } from "@/lib/home-feed-cache";
import { useActionLock } from "@/lib/use-action-lock";

const maxBytes = 100 * 1024 * 1024;

export default function OwnVideoPage() {
  const { session, token } = useAuth();
  const router = useRouter();
  const [body, setBody] = useState("");
  const [file, setFile] = useState<File | null>(null);
  const uploaded = useRef<{ file: File; id: string } | null>(null);
  const [error, setError] = useState("");
  const [status, setStatus] = useState("");
  const [upload, setUpload] = useState<CloudinaryUploadProgress | null>(null);
  const [uploadPhase, setUploadPhase] = useState<"upload" | "saving">("upload");
  const { pending: submitting, acquire, release } = useActionLock();

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
    if (!acquire()) return;
    setError("");
    setUpload(null);
    try {
      let publicId: string;
      if (uploaded.current?.file === file) {
        // 詳細などの直しで送り直すときは、上げ済みの動画を使い回す（同じ動画を上げ直して孤児を増やさない）。
        publicId = uploaded.current.id;
      } else {
        setStatus("アップロードの準備をしています…");
        const signed = await apiFetch<CloudinarySignature>("/v1/uploads/video", { method: "POST", token });
        setStatus("");
        setUploadPhase("upload");
        setUpload({ loaded: 0, total: file.size, percent: 0 });
        const payload = await uploadToCloudinary("video", file, signed, setUpload);
        if (payload.public_id !== signed.public_id) {
          throw new Error("アップロードに失敗しました");
        }
        publicId = payload.public_id;
        uploaded.current = { file, id: publicId };
        setUpload((current) => (current ? { ...current, loaded: current.total, percent: 100 } : current));
      }
      setUploadPhase("saving");
      setStatus("");
      await apiFetch<Post>("/v1/posts", {
        method: "POST",
        token,
        body: JSON.stringify({
          body,
          kind: "video",
          video_public_id: publicId,
        }),
      });
      clearHomeFeeds();
      router.push("/?sort=new");
    } catch (err) {
      // 動画そのものを断られたとき（項目の指定がないエラー）は、上げ直しが必要なので使い回さない。
      if (!(err instanceof ApiRequestError && err.fields)) uploaded.current = null;
      setError(apiErrorText(err, err instanceof Error ? err.message : "保存に失敗しました"));
      setStatus("");
      setUpload(null);
      release();
    }
  }

  return (
    <ComposerPage title="自分の動画">
      <form onSubmit={onSubmit} className="space-y-4">
        <label className="block text-sm font-medium">
          動画ファイル
          <input
            className="mt-1 block w-full text-sm text-slate-300 file:mr-3 file:rounded-full file:border-0 file:bg-white/15 file:px-3 file:py-2 file:text-sm file:font-semibold file:text-slate-100"
            type="file"
            accept="video/mp4,video/quicktime,video/webm"
            onChange={(e) => setFile(e.target.files?.[0] ?? null)}
            disabled={submitting}
            required
          />
        </label>
        <label className="block text-sm font-medium">
          詳細
          <Textarea rows={3} className="mt-1" value={body} onChange={(e) => setBody(e.target.value)} />
        </label>
        {upload ? (
          <UploadProgress noun="動画" phase={uploadPhase} progress={upload} fileName={file?.name} fileSize={file?.size} />
        ) : submitting ? <SubmittingProgress /> : null}
        {status ? <p className="text-sm text-slate-200">{status}</p> : null}
        {error ? <p className="text-sm text-red-400">{error}</p> : null}
        <Button type="submit" className="w-full" disabled={submitting || !file}>
          {submitting ? "送っています" : "アップロードして投稿"}
        </Button>
      </form>
    </ComposerPage>
  );
}
