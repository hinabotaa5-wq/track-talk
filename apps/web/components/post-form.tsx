"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { useForm } from "react-hook-form";
import { postSchema } from "@/lib/validators";
import type { z } from "zod";
import { TitleField } from "./composer-page";
import { Textarea } from "./ui/textarea";
import { Button } from "./ui/button";
import { UploadProgress } from "./upload-progress";
import type { Post } from "@/lib/types";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { uploadToCloudinary, type CloudinarySignature, type CloudinaryUploadProgress } from "@/lib/cloudinary-upload";
import { useActionLock } from "@/lib/use-action-lock";
import { useAuth } from "./auth-provider";
import { useRouter } from "next/navigation";
import { useState } from "react";

type Values = z.infer<typeof postSchema>;

const maxPhotoBytes = 10 * 1024 * 1024;

export function PostForm({ initial, returnTo }: { initial?: Post; returnTo?: "me" }) {
  const { token, session } = useAuth();
  const router = useRouter();
  const [error, setError] = useState("");
  const [status, setStatus] = useState("");
  const [upload, setUpload] = useState<CloudinaryUploadProgress | null>(null);
  const [uploadPhase, setUploadPhase] = useState<"upload" | "saving">("upload");
  const [photo, setPhoto] = useState<File | null>(null);
  const { pending, acquire, release } = useActionLock();
  const form = useForm<Values>({
    resolver: zodResolver(postSchema),
    defaultValues: {
      title: initial?.title ?? "",
      body: initial?.body ?? "",
    },
  });

  async function onSubmit(values: Values) {
    if (!session) {
      router.push("/login");
      return;
    }
    if (!acquire()) return;
    setError("");
    setStatus("");
    setUpload(null);
    const payload: { title: string; body: string; image_public_id?: string } = {
      title: values.title,
      body: values.body,
    };
    try {
      if (!initial && photo) {
        const looksLikeImage = photo.type.startsWith("image/") || /\.(jpe?g|png|webp|gif|heic|heif)$/i.test(photo.name);
        if (!looksLikeImage) {
          setError("写真ファイルを選んでください");
          release();
          return;
        }
        if (photo.size > maxPhotoBytes) {
          setError("10MB以内の写真にしてください");
          release();
          return;
        }
        setStatus("アップロードの準備をしています…");
        const signed = await apiFetch<CloudinarySignature>("/v1/uploads/image", { method: "POST", token });
        setStatus("");
        setUploadPhase("upload");
        setUpload({ loaded: 0, total: photo.size, percent: 0 });
        const result = await uploadToCloudinary("image", photo, signed, setUpload);
        if (result.public_id !== signed.public_id) {
          throw new Error("写真のアップロードに失敗しました");
        }
        payload.image_public_id = result.public_id;
        setUpload((current) => (current ? { ...current, loaded: current.total, percent: 100 } : current));
        setUploadPhase("saving");
        setStatus("");
      }
      const post = initial
        ? await apiFetch<Post>(`/v1/posts/${initial.id}`, {
            method: "PATCH",
            token,
            body: JSON.stringify(payload),
          })
        : await apiFetch<Post>("/v1/posts", {
            method: "POST",
            token,
            body: JSON.stringify(payload),
          });
      router.push(returnTo === "me" ? `/posts/${post.id}?from=me` : `/posts/${post.id}`);
    } catch (e) {
      if (e instanceof ApiRequestError) {
        const field = e.fields ? Object.values(e.fields)[0] : "";
        setError(field || e.message);
      } else {
        setError(e instanceof Error ? e.message : "保存に失敗しました");
      }
      setStatus("");
      setUpload(null);
      release();
    }
  }

  return (
    <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-4">
      <TitleField {...form.register("title")} />
      {form.formState.errors.title ? (
        <p className="text-sm text-red-400">{form.formState.errors.title.message}</p>
      ) : null}
      <label className="block text-sm font-medium">
        本文
        <Textarea rows={10} className="mt-1" {...form.register("body")} />
      </label>
      {form.formState.errors.body ? (
        <p className="text-sm text-red-400">{form.formState.errors.body.message}</p>
      ) : null}
      {!initial ? (
        <label className="block text-sm font-medium">
          写真
          <input
            className="mt-1 block w-full text-sm text-slate-300 file:mr-3 file:rounded-full file:border-0 file:bg-white/15 file:px-3 file:py-2 file:text-sm file:font-semibold file:text-slate-100"
            type="file"
            accept="image/jpeg,image/png,image/webp,image/gif,image/heic,image/heif"
            onChange={(e) => setPhoto(e.target.files?.[0] ?? null)}
            disabled={pending}
          />
        </label>
      ) : initial.thumbnail_url ? (
        <img src={initial.thumbnail_url} alt="" className="max-h-80 w-full rounded-md bg-black/25 object-contain" />
      ) : null}
      {upload ? (
        <UploadProgress noun="写真" phase={uploadPhase} progress={upload} fileName={photo?.name} fileSize={photo?.size} />
      ) : null}
      {status ? <p className="text-sm text-slate-200">{status}</p> : null}
      {error ? <p className="text-sm text-red-400">{error}</p> : null}
      <Button type="submit" className="w-full" disabled={pending}>
        {pending ? (initial ? "更新しています" : "送っています") : initial ? "更新する" : "投稿する"}
      </Button>
    </form>
  );
}
