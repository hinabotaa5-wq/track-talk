"use client";

import { useAuth } from "@/components/auth-provider";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { nicknameSchema } from "@/lib/validators";
import { zodResolver } from "@hookform/resolvers/zod";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { useForm, useWatch } from "react-hook-form";
import type { z } from "zod";

export default function ProfilePage() {
  const { session, token, profile, loading, refreshProfile } = useAuth();
  const router = useRouter();
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const form = useForm<z.infer<typeof nicknameSchema>>({
    resolver: zodResolver(nicknameSchema),
    defaultValues: { nickname: "", nickname_public: false, post_master_badge_public: true },
  });
  const nickname = useWatch({ control: form.control, name: "nickname" });
  const overLimit = Array.from(nickname).length > 10;

  useEffect(() => {
    if (!loading && !session) router.replace("/login?next=/me/profile");
  }, [loading, session, router]);

  useEffect(() => {
    if (!profile) return;
    form.reset({
      nickname: profile.nickname ?? "",
      nickname_public: profile.nickname_public ?? false,
      post_master_badge_public: profile.post_master_badge_public ?? true,
    });
  }, [profile, form]);

  async function onSubmit(values: z.infer<typeof nicknameSchema>) {
    if (!token) return;
    setMessage("");
    setError("");
    try {
      await apiFetch("/v1/me", {
        method: "PATCH",
        token,
        body: JSON.stringify(values),
      });
      await refreshProfile();
      setMessage("保存しました。");
    } catch (e) {
      setError(e instanceof ApiRequestError ? e.message : "更新に失敗しました。");
    }
  }

  if (loading || !session) {
    return <p className="text-sm text-slate-200">読み込み中...</p>;
  }

  return (
    <div className="mx-auto max-w-md space-y-4">
      <h1 className="text-2xl font-bold">ニックネームとバッジ</h1>
      <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-3">
        <Input placeholder="ニックネーム" {...form.register("nickname")} />
        {overLimit ? (
          <p className="text-sm text-red-400">10文字以内で入力してください</p>
        ) : form.formState.errors.nickname ? (
          <p className="text-sm text-red-400">{form.formState.errors.nickname.message}</p>
        ) : null}
        <label className="flex items-center gap-2 text-sm text-slate-200">
          <input type="checkbox" className="size-4 accent-blue-500" {...form.register("nickname_public")} />
          投稿にニックネームを公開する
        </label>
        <div className="space-y-1">
          <label className="flex items-center gap-2 text-sm text-slate-200">
            <input type="checkbox" className="size-4 accent-blue-500" {...form.register("post_master_badge_public")} />
            「投稿マスター」のバッジを表示する
          </label>
          <p className="pl-6 text-xs text-slate-400">
            いいねがついた投稿が30件以上になると、投稿の詳細でニックネームの横に出ます。ニックネームを公開していないときは出ません。
          </p>
        </div>
        {error ? <p className="text-sm text-red-400">{error}</p> : null}
        {message ? <p className="text-sm text-emerald-400">{message}</p> : null}
        <Button type="submit" className="w-full" disabled={form.formState.isSubmitting}>
          保存
        </Button>
      </form>
    </div>
  );
}
