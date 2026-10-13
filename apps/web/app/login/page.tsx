"use client";

import { authErrorMessage } from "@/lib/auth-errors";
import { createClient } from "@/lib/supabase";
import { safeNextPath } from "@/lib/safe-next";
import { useActionLock } from "@/lib/use-action-lock";
import { loginSchema } from "@/lib/validators";
import { zodResolver } from "@hookform/resolvers/zod";
import { useForm } from "react-hook-form";
import { GoogleSignInButton } from "@/components/google-sign-in-button";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import Link from "next/link";
import { useState, useSyncExternalStore } from "react";
import type { z } from "zod";

const callbackErrors: Record<string, string> = {
  config: "Supabase の公開設定がありません。",
  google: "Googleログインに失敗しました。",
};

function subscribeToNothing() {
  return () => {};
}

function callbackErrorCode() {
  return new URLSearchParams(window.location.search).get("error");
}

export default function LoginPage() {
  const [submitError, setError] = useState("");
  const { pending, acquire, release } = useActionLock();
  const code = useSyncExternalStore(subscribeToNothing, callbackErrorCode, () => null);
  const error = submitError || (code ? (callbackErrors[code] ?? "") : "");

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
    const { error: err } = await supabase.auth.signInWithPassword(values);
    if (err) {
      setError(authErrorMessage(err));
      release();
      return;
    }
    // ログイン前に先読みした「/login への転送」を捨てるため、ページを読み直す。
    window.location.assign(safeNextPath(new URLSearchParams(window.location.search).get("next")));
  }

  return (
    <div className="mx-auto max-w-md space-y-4">
      <h1 className="text-2xl font-bold">ログイン</h1>
      <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-3">
        <Input type="email" placeholder="メール" {...form.register("email")} />
        <Input type="password" placeholder="パスワード" {...form.register("password")} />
        {error ? <p className="text-sm text-red-400">{error}</p> : null}
        <Button type="submit" className="w-full" disabled={pending}>
          ログイン
        </Button>
      </form>
      <GoogleSignInButton />
      <p className="text-sm">
        アカウントがない場合は <Link href="/register" className="text-blue-400">登録</Link>
      </p>
      <p className="text-sm">
        <Link href="/reset-password" className="text-blue-400">
          パスワードを忘れた
        </Link>
      </p>
    </div>
  );
}
