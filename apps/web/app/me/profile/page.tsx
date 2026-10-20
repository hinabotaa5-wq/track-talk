"use client";

import { useAuth } from "@/components/auth-provider";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { appEvents } from "@/lib/practice-events";
import { profileIncomplete } from "@/lib/profile-gate";
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
    defaultValues: { nickname: "", nickname_public: false, event: "" as never },
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
      event: (profile.event ?? "") as never,
    });
  }, [profile, form]);

  async function onSubmit(values: z.infer<typeof nicknameSchema>) {
    if (!token) return;
    setMessage("");
    setError("");
    // ニックネームか種目がまだの人が決め終えたら、ほかの画面へ進めるのでホームへ送る。
    const firstTime = profileIncomplete(profile);
    try {
      await apiFetch("/v1/me", {
        method: "PATCH",
        token,
        body: JSON.stringify(values),
      });
      await refreshProfile();
      if (firstTime) {
        router.replace("/");
        return;
      }
      setMessage("プロフィールを更新しました。");
    } catch (e) {
      setError(e instanceof ApiRequestError ? e.message : "更新に失敗しました。");
    }
  }

  const incomplete = profileIncomplete(profile);

  if (loading || !session) {
    return <p className="text-sm text-slate-200">読み込み中...</p>;
  }

  return (
    <div className="mx-auto max-w-md space-y-4">
      <h1 className="text-2xl font-bold">{incomplete ? "ニックネームと種目を決めよう" : "ニックネームと種目の変更"}</h1>
      {incomplete ? <p className="text-sm text-slate-200">どちらも、使い始めるのに必要です。あとから変えられます。</p> : null}
      <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-3">
        <Input placeholder="ニックネーム" {...form.register("nickname")} />
        {overLimit ? (
          <p className="text-sm text-red-400">10文字以内で入力してください</p>
        ) : form.formState.errors.nickname ? (
          <p className="text-sm text-red-400">{form.formState.errors.nickname.message}</p>
        ) : null}
        <label className="block text-sm font-medium">
          種目
          <select
            className="mt-1 w-full rounded-md border border-white/25 glass bg-white/10 px-3 py-2 text-base text-slate-100 outline-none focus:border-blue-400"
            {...form.register("event")}
          >
            <option value="" className="text-slate-900">
              選んでください
            </option>
            {appEvents.map((item) => (
              <option key={item.value} value={item.value} className="text-slate-900">
                {item.label}
              </option>
            ))}
          </select>
        </label>
        {form.formState.errors.event ? <p className="text-sm text-red-400">{form.formState.errors.event.message}</p> : null}
        <label className="flex items-center gap-2 text-sm text-slate-200">
          <input type="checkbox" className="size-4 accent-blue-500" {...form.register("nickname_public")} />
          投稿にニックネームを公開する
        </label>
        {error ? <p className="text-sm text-red-400">{error}</p> : null}
        {message ? <p className="text-sm text-emerald-400">{message}</p> : null}
        <Button type="submit" className="w-full" disabled={form.formState.isSubmitting}>
          保存
        </Button>
      </form>
    </div>
  );
}
