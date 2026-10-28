"use client";

import { callbackErrorMessages } from "@/lib/auth-callback";
import { authErrorMessage } from "@/lib/auth-errors";
import { createClient } from "@/lib/supabase";
import { safeNextPath } from "@/lib/safe-next";
import { useActionLock } from "@/lib/use-action-lock";
import { loginSchema } from "@/lib/validators";
import { zodResolver } from "@hookform/resolvers/zod";
import { useForm } from "react-hook-form";
import { GoogleSignInButton } from "@/components/google-sign-in-button";
import { useCaptcha } from "@/components/auth-captcha";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { CoachApplyLink } from "@/components/coach-apply-link";
import Link from "next/link";
import { useAuth } from "@/components/auth-provider";
import { useEffect, useState, useSyncExternalStore } from "react";
import type { z } from "zod";

function subscribeToNothing() {
  return () => {};
}

function callbackErrorCode() {
  return new URLSearchParams(window.location.search).get("error");
}

function nextParam() {
  return new URLSearchParams(window.location.search).get("next");
}

// ログイン後の画面へは、履歴のログイン画面を置き換えて進む。
// push で進むと、戻るボタンでまたログイン画面に戻ってしまう。
function leaveLogin() {
  window.location.replace(safeNextPath(nextParam()));
}

export default function LoginPage() {
  const [submitError, setError] = useState("");
  const { pending, acquire, release } = useActionLock();
  const captcha = useCaptcha();
  const code = useSyncExternalStore(subscribeToNothing, callbackErrorCode, () => null);
  const next = useSyncExternalStore(subscribeToNothing, nextParam, () => null);
  const error = submitError || (code ? (callbackErrorMessages[code] ?? "") : "");
  const { session, sessionLoading } = useAuth();

  // Google でログインして戻ったあとなど、履歴に残ったログイン画面へ戻ってきたときは、
  // ログイン済みならフォームを見せずに戻り先へ進む。
  useEffect(() => {
    if (!sessionLoading && session) leaveLogin();
  }, [session, sessionLoading]);
  useEffect(() => {
    function onPageShow(event: PageTransitionEvent) {
      if (!event.persisted) return;
      // 戻る/進むのキャッシュから復元された画面は、ログイン前の状態のままなので読み直して確かめる。
      void createClient()?.auth.getSession().then(({ data }) => {
        if (data.session) leaveLogin();
      });
    }
    window.addEventListener("pageshow", onPageShow);
    return () => window.removeEventListener("pageshow", onPageShow);
  }, []);

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
    if (!captcha.ready) {
      setError("「人間であることの確認」が終わるまでお待ちください。");
      return;
    }
    if (!acquire()) return;
    const { error: err } = await supabase.auth.signInWithPassword({ ...values, options: { captchaToken: captcha.captchaToken } });
    if (err) {
      setError(authErrorMessage(err));
      // 確認のトークンは1回しか使えないので、取り直してからもう一度試せるようにする。
      captcha.reset();
      release();
      return;
    }
    // ログイン前に先読みした「/login への転送」を捨てるため、ページを読み直す。
    leaveLogin();
  }

  return (
    <div className="mx-auto max-w-md space-y-4">
      <h1 className="text-2xl font-bold">ログイン</h1>
      <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-3">
        <Input type="email" placeholder="メール" {...form.register("email")} />
        <Input type="password" placeholder="パスワード" {...form.register("password")} />
        {captcha.widget}
        {error ? <p className="text-sm text-red-400">{error}</p> : null}
        <Button type="submit" className="w-full" disabled={pending || !captcha.ready}>
          ログイン
        </Button>
      </form>
      {/* Google ログインも、パスワードのログインと同じ戻り先（next）に戻す。 */}
      <GoogleSignInButton nextPath={safeNextPath(next)} />
      {safeNextPath(next) === "/coach" ? (
        <CoachApplyLink />
      ) : null}
      <p className="text-sm">
        アカウントがない場合は <Link href="/register" className="inline-block text-blue-400 transition active:scale-[0.98]">登録</Link>
      </p>
      <p className="text-sm">
        <Link href="/reset-password" className="inline-block text-blue-400 transition active:scale-[0.98]">
          パスワードを忘れた
        </Link>
      </p>
      <p className="text-sm"><Link href="/contact" className="text-blue-300 underline">ログインできないときのお問い合わせ</Link></p>
    </div>
  );
}
