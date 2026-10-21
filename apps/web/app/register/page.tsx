"use client";

import { authErrorMessage } from "@/lib/auth-errors";
import { rememberNextPath } from "@/lib/safe-next";
import { createClient } from "@/lib/supabase";
import { useActionLock } from "@/lib/use-action-lock";
import { loginSchema } from "@/lib/validators";
import { zodResolver } from "@hookform/resolvers/zod";
import { useForm } from "react-hook-form";
import { GoogleSignInButton } from "@/components/google-sign-in-button";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState } from "react";
import type { z } from "zod";

export default function RegisterPage() {
  const router = useRouter();
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const [agreed, setAgreed] = useState(false);
  const { pending, acquire, release } = useActionLock();
  const form = useForm<z.infer<typeof loginSchema>>({
    resolver: zodResolver(loginSchema),
    defaultValues: { email: "", password: "" },
  });

  async function onSubmit(values: z.infer<typeof loginSchema>) {
    if (!agreed) {
      setError("登録には利用規約・ガイドライン・プライバシーポリシーへの同意が必要です。");
      return;
    }
    const supabase = createClient();
    if (!supabase) {
      setError("Supabase の公開設定がありません。");
      return;
    }
    if (!acquire()) return;
    setError("");
    rememberNextPath("/me/profile?onboarding=1", 86400);
    const { data, error: err } = await supabase.auth
      .signUp({
        email: values.email,
        password: values.password,
        options: { emailRedirectTo: `${window.location.origin}/auth/callback` },
      })
      .finally(release);
    if (err) {
      setError(authErrorMessage(err));
      return;
    }
    if (data.session) {
      router.push("/me/profile?onboarding=1");
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
        {error ? <p className="text-sm text-red-400">{error}</p> : null}
        {message ? <p className="text-sm text-emerald-400">{message}</p> : null}
        <label className="flex cursor-pointer items-start gap-2 text-sm">
          <input
            type="checkbox"
            className="mt-1 size-4 shrink-0 accent-blue-600"
            checked={agreed}
            onChange={(e) => setAgreed(e.target.checked)}
          />
          <span>
            <Link href="/terms" className="inline-block text-blue-400 transition active:scale-95">
              利用規約
            </Link>
            ・
            <Link href="/guidelines" className="inline-block text-blue-400 transition active:scale-95">
              コミュニティガイドライン
            </Link>
            ・
            <Link href="/privacy" className="inline-block text-blue-400 transition active:scale-95">
              プライバシーポリシー
            </Link>
            に同意する
          </span>
        </label>
        <Button type="submit" className="w-full" disabled={pending || !agreed}>
          登録
        </Button>
      </form>
      <GoogleSignInButton nextPath="/me/profile?onboarding=1" label="Googleで登録" disabled={!agreed} />
    </div>
  );
}
