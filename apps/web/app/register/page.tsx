"use client";

import { authErrorMessage } from "@/lib/auth-errors";
import { rememberNextPath } from "@/lib/safe-next";
import { createClient } from "@/lib/supabase";
import { useActionLock } from "@/lib/use-action-lock";
import { loginSchema } from "@/lib/validators";
import { zodResolver } from "@hookform/resolvers/zod";
import { useForm, useWatch } from "react-hook-form";
import { GoogleSignInButton } from "@/components/google-sign-in-button";
import { OverseasConsentNotice } from "@/components/overseas-consent-notice";
import { ServiceConsentText } from "@/components/service-consent-text";
import { rememberOverseasConsent } from "@/lib/overseas-consent";
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
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
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
      setError("登録には利用規約・ガイドライン・プライバシーポリシーと、海外の事業者に預けることへの同意が必要です。");
      return;
    }
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
    setError("");
    rememberNextPath("/me/profile?onboarding=1", 86400);
    rememberOverseasConsent();
    const { data, error: err } = await supabase.auth
      .signUp({
        email: values.email,
        password: values.password,
        options: { emailRedirectTo: `${window.location.origin}/auth/callback`, captchaToken: captcha.captchaToken },
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
      router.push("/me/profile?onboarding=1");
      return;
    }
    // 登録済みのアドレスでも、Supabase はアドレスの有無を隠すためにエラーを返さず、メールも送らない。
    // そのときは identities が空で返ってくるので、待たせずにログインへ案内する。
    if (data.user && data.user.identities?.length === 0) {
      setError("このメールアドレスはすでに登録されています。ログインするか、パスワードを再設定してください。");
      return;
    }
    setMessage("確認メールを送信しました。メール内のリンクを開いてください。数分たっても届かないときは、迷惑メールのフォルダも見てください。");
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
          <ServiceConsentText />
        </label>
        <details className="rounded-lg border border-slate-700 px-3 py-2">
          <summary className="cursor-pointer text-sm text-slate-200">海外の事業者に預けることについて</summary>
          <div className="pt-2">
            <OverseasConsentNotice />
          </div>
        </details>
        {captcha.widget}
        <Button type="submit" className="w-full" disabled={pending || !agreed || !captcha.ready}>
          登録
        </Button>
      </form>
      <GoogleSignInButton nextPath="/me/profile?onboarding=1" label="Googleで登録" disabled={!agreed} onStart={rememberOverseasConsent} />
      <p className="text-sm">
        アカウントがある場合は <Link href="/login" className="inline-block text-blue-400 transition active:scale-[0.98]">ログイン</Link>
      </p>
    </div>
  );
}
