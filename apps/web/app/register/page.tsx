"use client";

import { authErrorMessage } from "@/lib/auth-errors";
import { createClient } from "@/lib/supabase";
import { useActionLock } from "@/lib/use-action-lock";
import { registerSchema } from "@/lib/validators";
import { zodResolver } from "@hookform/resolvers/zod";
import { useForm } from "react-hook-form";
import { GoogleSignInButton } from "@/components/google-sign-in-button";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { useState } from "react";
import type { z } from "zod";

export default function RegisterPage() {
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const { pending, acquire, release } = useActionLock();
  const form = useForm<z.infer<typeof registerSchema>>({
    resolver: zodResolver(registerSchema),
    defaultValues: { email: "", password: "", nickname: "", nickname_public: false },
  });

  async function onSubmit(values: z.infer<typeof registerSchema>) {
    const supabase = createClient();
    if (!supabase) {
      setError("Supabase の公開設定がありません。");
      return;
    }
    if (!acquire()) return;
    setError("");
    const { data, error: err } = await supabase.auth
      .signUp({
        email: values.email,
        password: values.password,
        options: {
          data: {
            nickname: values.nickname.trim(),
            nickname_public: values.nickname_public,
          },
        },
      })
      .finally(release);
    if (err) {
      setError(authErrorMessage(err));
      return;
    }
    if (data.session) {
      setMessage("登録しました。トップへ移動できます。");
      return;
    }
    setMessage("確認メールを送信しました。メール内のリンクを開いてください。");
  }

  return (
    <div className="mx-auto max-w-md space-y-4">
      <h1 className="text-2xl font-bold">新規登録</h1>
      <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-3">
        <Input type="email" placeholder="メール" {...form.register("email")} />
        <Input type="password" placeholder="パスワード（8文字以上）" {...form.register("password")} />
        <Input maxLength={30} placeholder="ニックネーム" {...form.register("nickname")} />
        <label className="flex items-center gap-2 text-sm text-slate-200">
          <input type="checkbox" className="size-4 accent-blue-500" {...form.register("nickname_public")} />
          投稿詳細でニックネームを公開する
        </label>
        {form.formState.errors.nickname ? (
          <p className="text-sm text-red-400">{form.formState.errors.nickname.message}</p>
        ) : null}
        {error ? <p className="text-sm text-red-400">{error}</p> : null}
        {message ? <p className="text-sm text-emerald-400">{message}</p> : null}
        <Button type="submit" className="w-full" disabled={pending}>
          登録
        </Button>
      </form>
      <GoogleSignInButton nextPath="/me/profile?onboarding=1" label="Googleで登録" />
    </div>
  );
}
