"use client";

import { authErrorMessage } from "@/lib/auth-errors";
import { createClient } from "@/lib/supabase";
import { useActionLock } from "@/lib/use-action-lock";
import { loginSchema } from "@/lib/validators";
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
  const form = useForm<z.infer<typeof loginSchema>>({
    resolver: zodResolver(loginSchema),
    defaultValues: { email: "", password: "" },
  });

  async function onSubmit(values: z.infer<typeof loginSchema>) {
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
        {error ? <p className="text-sm text-danger">{error}</p> : null}
        {message ? <p className="text-sm text-success">{message}</p> : null}
        <Button type="submit" className="w-full" disabled={pending}>
          登録
        </Button>
      </form>
      <GoogleSignInButton />
    </div>
  );
}
