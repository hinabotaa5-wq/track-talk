"use client";

import { callbackErrorMessages } from "@/lib/auth-callback";
import { authErrorMessage } from "@/lib/auth-errors";
import { t, L } from "@/lib/i18n";
import { useLocale } from "@/components/locale-provider";
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
  // 表示言語はブラウザ言語の自動判定（運営者決定 2026-11-13）。ログイン画面の言語選択は廃止。
  const shown = useLocale();
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
      setError(L("Supabase の公開設定がありません。"));
      return;
    }
    if (!captcha.ready) {
      setError(L("「人間であることの確認」が終わるまでお待ちください。"));
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
      <h1 className="text-2xl font-bold">{t(shown, "login.title")}</h1>
      <form method="post" onSubmit={form.handleSubmit(onSubmit)} className="space-y-3">
        <Input type="email" placeholder={t(shown, "login.email")} {...form.register("email")} />
        <Input type="password" placeholder={t(shown, "login.password")} {...form.register("password")} />
        {captcha.widget}
        {error ? <p className="text-sm text-red-400">{error}</p> : null}
        <Button type="submit" className="w-full" disabled={pending || !captcha.ready}>
          {t(shown, "login.submit")}
        </Button>
      </form>
      {/* Google ログインも、パスワードのログインと同じ戻り先（next）に戻す。 */}
      <GoogleSignInButton nextPath={safeNextPath(next)} />
      {/* 初めての人向けの登録は、ログインの塊と区切ってボタンで出す。 */}
      <div className="space-y-2 border-t border-white/15 pt-4">
        <p className="text-sm">{t(shown, "login.noAccount")}</p>
        <Link
          href="/register"
          className="inline-flex w-full items-center justify-center gap-2 rounded-full border border-white/25 glass bg-white/12 px-4.5 py-2 text-sm font-semibold text-slate-200 shadow-xs transition-all duration-150 hover:bg-white/16 hover:text-white active:scale-[0.98] active:bg-white/20 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
        >
          {t(shown, "register.title")}
        </Link>
      </div>
      {/* 困った人向けの2導線は、登録とは間を置いてテキストリンクのまま並べる。 */}
      <div className="space-y-1 pt-1">
        <p className="text-sm">
          <Link
            href="/reset-password"
            className="flex min-h-[44px] w-full items-center justify-center px-4 py-2 text-center text-sm font-medium text-blue-400 transition hover:text-blue-300 hover:underline active:scale-[0.98] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
          >
            {t(shown, "login.forgot")}
          </Link>
        </p>
        <p className="text-sm">
          <Link
            href="/contact"
            className="flex min-h-[44px] w-full items-center justify-center px-4 py-2 text-center text-sm font-medium text-blue-400 transition hover:text-blue-300 hover:underline active:scale-[0.98] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
          >
            {t(shown, "login.contact")}
          </Link>
        </p>
      </div>
    </div>
  );
}
