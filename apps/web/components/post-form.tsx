"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { useForm } from "react-hook-form";
import { postSchema } from "@/lib/validators";
import type { z } from "zod";
import { Input } from "./ui/input";
import { Textarea } from "./ui/textarea";
import { Button } from "./ui/button";
import type { Category, Competition, Post } from "@/lib/types";
import { apiFetch } from "@/lib/api";
import { useAuth } from "./auth-provider";
import { useRouter } from "next/navigation";
import { useState } from "react";

type Values = z.infer<typeof postSchema>;

export function PostForm({
  categories,
  competitions,
  initial,
}: {
  categories: Category[];
  competitions: Competition[];
  initial?: Post;
}) {
  const { token, session } = useAuth();
  const router = useRouter();
  const [error, setError] = useState("");
  const form = useForm<Values>({
    resolver: zodResolver(postSchema),
    defaultValues: {
      title: initial?.title ?? "",
      body: initial?.body ?? "",
      category_id: (initial?.category.id as Values["category_id"]) ?? "race_impression",
      competition_id: initial?.competition?.id ?? "",
    },
  });

  async function onSubmit(values: Values) {
    if (!session) {
      router.push("/login");
      return;
    }
    setError("");
    const payload = {
      title: values.title,
      body: values.body,
      category_id: values.category_id,
      competition_id: values.competition_id || null,
    };
    try {
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
      setError(e instanceof Error ? e.message : "保存に失敗しました");
    }
  }

  return (
    <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-4">
      <label className="block text-sm font-medium">
        タイトル
        <Input className="mt-1" {...form.register("title")} />
      </label>
      {form.formState.errors.title ? (
        <p className="text-sm text-red-600">{form.formState.errors.title.message}</p>
      ) : null}
      <label className="block text-sm font-medium">
        カテゴリ
        <select className="mt-1 w-full rounded-md border bg-white px-3 py-2 text-base" {...form.register("category_id")}>
          {categories.map((c) => (
            <option key={c.id} value={c.id}>
              {c.name}
            </option>
          ))}
        </select>
      </label>
      <label className="block text-sm font-medium">
        関連大会（任意）
        <select
          className="mt-1 w-full rounded-md border bg-white px-3 py-2 text-base"
          {...form.register("competition_id")}
        >
          <option value="">なし</option>
          {competitions.map((c) => (
            <option key={c.id} value={c.id}>
              {c.name}
            </option>
          ))}
        </select>
      </label>
      <label className="block text-sm font-medium">
        本文
        <Textarea rows={10} className="mt-1" {...form.register("body")} />
      </label>
      {form.formState.errors.body ? (
        <p className="text-sm text-red-600">{form.formState.errors.body.message}</p>
      ) : null}
      {error ? <p className="text-sm text-red-600">{error}</p> : null}
      <Button type="submit" className="w-full md:w-auto" disabled={form.formState.isSubmitting}>
        {initial ? "更新する" : "投稿する"}
      </Button>
    </form>
  );
}
