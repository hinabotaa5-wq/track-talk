"use client";

import { authErrorMessage } from "@/lib/auth-errors";
import { t, L } from "@/lib/i18n";
import { useLocale } from "@/components/locale-provider";
import { rememberNextPath } from "@/lib/safe-next";
import { createClient } from "@/lib/supabase";
import { isPwnedPassword, pwnedPasswordMessage } from "@/lib/pwned-password";
import { useActionLock } from "@/lib/use-action-lock";
import { loginSchema } from "@/lib/validators";
import { zodResolver } from "@hookform/resolvers/zod";
import { useForm, useWatch } from "react-hook-form";
import { GoogleSignInButton } from "@/components/google-sign-in-button";
import { ServiceConsentText } from "@/components/service-consent-text";
import { rememberOverseasConsent, signupConsentMetadata } from "@/lib/overseas-consent";
import { rememberSignupSentEmail } from "@/lib/signup-sent";
import { useCaptcha } from "@/components/auth-captcha";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import type { z } from "zod";

// 利用規約などを読みに行って戻ってきても、入れたメールアドレス・パスワード・同意のチェックが消えないよう、
// 画面を離れる間だけメモリに置いておく(パスワードを含むので、ブラウザの保存領域には書かない。再読み込みで消える)。
type Draft = { email: string; password: string; agreed: boolean };
const emptyDraft: Draft = { email: "", password: "", agreed: false };
let draft = emptyDraft;

function saveDraft(next: Draft) {
  draft = next;
}

export default function RegisterPage() {
  const router = useRouter();
  const [error, setError] = useState("");
  // 表示言語はブラウザ言語の自動判定（運営者決定 2026-11-13）。登録画面の言語選択は廃止。
  const shown = useLocale();
  const [agreed, setAgreed] = useState(draft.agreed);
  const { pending, acquire, release } = useActionLock();
  const captcha = useCaptcha();
  const form = useForm<z.infer<typeof loginSchema>>({
    resolver: zodResolver(loginSchema),
    defaultValues: { email: draft.email, password: draft.password },
  });

  const email = useWatch({ control: form.control, name: "email" });
  const password = useWatch({ control: form.control, name: "password" });

  useEffect(() => {
    saveDraft({ email: email ?? "", password: password ?? "", agreed });
  }, [email, password, agreed]);

  async function onSubmit(values: z.infer<typeof loginSchema>) {
    if (!agreed) {
      setError(L("登録には利用規約・ガイドライン・プライバシーポリシーと、海外の事業者に預けることへの同意が必要です。"));
      return;
    }
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
    setError("");
    // 漏洩済みパスワードは Supabase に送らずここで止める（本体 #429）。HIBP に届かなくても通す。
    if (await isPwnedPassword(values.password)) {
      captcha.reset();
      release();
      setError(L(pwnedPasswordMessage));
      return;
    }
    rememberNextPath("/me/profile?onboarding=1", 86400);
    rememberOverseasConsent();
    const { data, error: err } = await supabase.auth
      .signUp({
        email: values.email,
        password: values.password,
        options: {
          emailRedirectTo: `${window.location.origin}/auth/callback`,
          captchaToken: captcha.captchaToken,
          // 確認メールを別のタブで開いても、ここでの同意をもう一度聞かずに記録できるよう、同意した版を残す。
          data: signupConsentMetadata(),
        },
      })
      .finally(() => {
        // 確認のトークンは1回しか使えないので、成否にかかわらず取り直す。
        captcha.reset();
        release();
      });
    if (err) {
      setError(authErrorMessage(err));
      return;
    }
    saveDraft(emptyDraft);
    if (data.session) {
      router.replace("/me/profile?onboarding=1");
      return;
    }
    // 登録済みのアドレスでも、Supabase はアドレスの有無を隠すためにエラーを返さず、メールも送らない。
    // そのときは identities が空で返ってくるので、待たせずにログインへ案内する。
    if (data.user && data.user.identities?.length === 0) {
      setError(L("このメールアドレスはすでに登録されています。ログインするか、パスワードを再設定してください。"));
      return;
    }
    // 送ったアドレスと届かないときの案内は、送信済みの画面で出す(アドレスは URL に載せない)。
    rememberSignupSentEmail(values.email);
    router.push("/register/sent");
  }

  return (
    <div className="mx-auto max-w-md space-y-4">
      <h1 className="text-2xl font-bold">{t(shown, "register.title")}</h1>
      <form method="post" onSubmit={form.handleSubmit(onSubmit)} className="space-y-3">
        <Input type="email" placeholder={t(shown, "register.email")} {...form.register("email")} />
        <Input type="password" placeholder={t(shown, "register.password")} {...form.register("password")} />
        {error ? <p className="text-sm text-red-400">{error}</p> : null}
        <label className="flex cursor-pointer items-start gap-2 text-sm">
          <input
            type="checkbox"
            className="mt-1 size-4 shrink-0 accent-blue-600"
            checked={agreed}
            onChange={(e) => setAgreed(e.target.checked)}
          />
          <ServiceConsentText />
        </label>
        {captcha.widget}
        <Button type="submit" className="w-full" disabled={pending || !agreed || !captcha.ready}>
          {t(shown, "register.submit")}
        </Button>
      </form>
      <GoogleSignInButton nextPath="/me/profile?onboarding=1" label={t(shown, "register.google")} disabled={!agreed} onStart={rememberOverseasConsent} />
      <p className="text-sm">
        {t(shown, "register.hasAccount")} <Link href="/login" className="inline-block text-blue-400 transition active:scale-[0.98]">{t(shown, "register.login")}</Link>
      </p>
    </div>
  );
}
