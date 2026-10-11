"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { useForm } from "react-hook-form";
import { postSchema } from "@/lib/validators";
import type { z } from "zod";
import { TitleField } from "./composer-page";
import { Textarea } from "./ui/textarea";
import { Button } from "./ui/button";
import type { Post } from "@/lib/types";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { useAuth } from "./auth-provider";
import { useRouter } from "next/navigation";
import { useState } from "react";

type Values = z.infer<typeof postSchema>;

const maxPhotoBytes = 10 * 1024 * 1024;

type Signature = {
  cloud_name: string;
  api_key: string;
  timestamp: number;
  public_id: string;
  signature: string;
};

export function PostForm({ initial }: { initial?: Post }) {
  const { token, session } = useAuth();
  const router = useRouter();
  const [error, setError] = useState("");
  const [status, setStatus] = useState("");
  const [photo, setPhoto] = useState<File | null>(null);
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
    setError("");
    setStatus("");
    const payload: { title: string; body: string; image_public_id?: string } = {
      title: values.title,
      body: values.body,
    };
    try {
      if (!initial && photo) {
        const looksLikeImage = photo.type.startsWith("image/") || /\.(jpe?g|png|webp|gif|heic|heif)$/i.test(photo.name);
        if (!looksLikeImage) {
          setError("写真ファイルを選んでください");
          return;
        }
        if (photo.size > maxPhotoBytes) {
          setError("10MB以内の写真にしてください");
          return;
        }
        setStatus("アップロードの準備をしています…");
        const signed = await apiFetch<Signature>("/v1/uploads/image", { method: "POST", token });
        setStatus("写真を送っています…");
        const body = new FormData();
        body.append("file", photo);
        body.append("api_key", signed.api_key);
        body.append("timestamp", String(signed.timestamp));
        body.append("public_id", signed.public_id);
        body.append("signature", signed.signature);
        const uploaded = await fetch(`https://api.cloudinary.com/v1_1/${signed.cloud_name}/image/upload`, {
          method: "POST",
          body,
        });
        const result = (await uploaded.json()) as { public_id?: string; error?: { message?: string } };
        if (!uploaded.ok || result.public_id !== signed.public_id) {
          throw new Error(result.error?.message || "写真のアップロードに失敗しました");
        }
        payload.image_public_id = result.public_id;
        setStatus("投稿を保存しています…");
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
      router.push(`/posts/${post.id}`);
    } catch (e) {
      if (e instanceof ApiRequestError) {
        const field = e.fields ? Object.values(e.fields)[0] : "";
        setError(field || e.message);
      } else {
        setError(e instanceof Error ? e.message : "保存に失敗しました");
      }
      setStatus("");
    }
  }

  return (
    <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-4">
      <TitleField {...form.register("title")} />
      {form.formState.errors.title ? (
        <p className="text-sm text-red-600">{form.formState.errors.title.message}</p>
      ) : null}
      <label className="block text-sm font-medium">
        本文
        <Textarea rows={10} className="mt-1" {...form.register("body")} />
      </label>
      {form.formState.errors.body ? (
        <p className="text-sm text-red-600">{form.formState.errors.body.message}</p>
      ) : null}
      {!initial ? (
        <label className="block text-sm font-medium">
          写真
          <input
            className="mt-1 block w-full text-base"
            type="file"
            accept="image/jpeg,image/png,image/webp,image/gif,image/heic,image/heif"
            onChange={(e) => setPhoto(e.target.files?.[0] ?? null)}
          />
        </label>
      ) : initial.thumbnail_url ? (
        <img src={initial.thumbnail_url} alt="" className="max-h-80 w-full rounded-md bg-zinc-100 object-contain" />
      ) : null}
      {status ? <p className="text-sm text-zinc-600">{status}</p> : null}
      {error ? <p className="text-sm text-red-600">{error}</p> : null}
      <Button type="submit" className="w-full md:w-auto" disabled={form.formState.isSubmitting}>
        {initial ? "更新する" : "投稿する"}
      </Button>
    </form>
  );
}
