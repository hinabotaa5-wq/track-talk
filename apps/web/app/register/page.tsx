"use client";

import { createClient } from "@/lib/supabase";
import { registerSchema } from "@/lib/validators";
import { zodResolver } from "@hookform/resolvers/zod";
import { useForm } from "react-hook-form";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { apiFetch } from "@/lib/api";
import { useState } from "react";
import type { z } from "zod";

export default function RegisterPage() {
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const form = useForm<z.infer<typeof registerSchema>>({
    resolver: zodResolver(registerSchema),
    defaultValues: { email: "", password: "", nickname: "" },
  });

  async function onSubmit(values: z.infer<typeof registerSchema>) {
    const supabase = createClient();
    if (!supabase) {
      setError("Supabase の公開設定がありません。");
      return;
    }
    setError("");
    const { data, error: err } = await supabase.auth.signUp({
      email: values.email,
      password: values.password,
    });
    if (err) {
      setError(err.message);
      return;
    }
    const token = data.session?.access_token;
    if (token) {
      try {
        await apiFetch("/v1/me/profile", {
          method: "PUT",
          token,
          body: JSON.stringify({ nickname: values.nickname, bio: "" }),
        });
      } catch (e) {
        setError(e instanceof Error ? e.message : "プロフィール作成に失敗しました");
        return;
      }
      setMessage("登録しました。トップへ移動できます。");
    } else {
      setMessage("確認メールを送信しました。メール内のリンクを開いてください。");
    }
  }

  return (
    <div className="mx-auto max-w-md space-y-4">
      <h1 className="text-2xl font-bold">新規登録</h1>
      <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-3">
        <Input placeholder="ニックネーム" {...form.register("nickname")} />
        <Input type="email" placeholder="メール" {...form.register("email")} />
        <Input type="password" placeholder="パスワード（8文字以上）" {...form.register("password")} />
        {error ? <p className="text-sm text-red-600">{error}</p> : null}
        {message ? <p className="text-sm text-green-700">{message}</p> : null}
        <Button type="submit" className="w-full md:w-auto">登録</Button>
      </form>
    </div>
  );
}
