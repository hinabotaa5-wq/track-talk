"use client";
import { L } from "@/lib/i18n";

import { authErrorMessage } from "@/lib/auth-errors";
import { signupSentEmail } from "@/lib/signup-sent";
import { createClient } from "@/lib/supabase";
import { useActionLock } from "@/lib/use-action-lock";
import { useCaptcha } from "@/components/auth-captcha";
import { Button } from "@/components/ui/button";
import { MailCheck } from "lucide-react";
import Link from "next/link";
import { useState, useSyncExternalStore } from "react";

// 送ったアドレスは画面を開いている間に変わらないので、変化の通知は受けない。
function subscribeNothing() {
  return () => {};
}

// メールで登録したあとに移る画面。どのアドレスに送ったかと、届かないときの案内を出す。
// 同意した版は登録のときにアカウントへ残してあるので、再送しても確認後の同意の記録はそのまま使える。
export default function RegisterSentPage() {
  // 保存領域はブラウザにしか無いので、サーバーで描くときはアドレス無しにしておく。
  const email = useSyncExternalStore(subscribeNothing, signupSentEmail, () => "");
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const { pending, acquire, release } = useActionLock();
  const captcha = useCaptcha();

  async function resend() {
    const supabase = createClient();
    if (!supabase || !email || !captcha.ready || !acquire()) return;
    setError("");
    setMessage("");
    const { error: err } = await supabase.auth
      .resend({
        type: "signup",
        email,
        options: {
          emailRedirectTo: `${window.location.origin}/auth/callback`,
          captchaToken: captcha.captchaToken,
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
    setMessage(L("確認メールをもう一度送信しました。"));
  }

  return (
    <div className="mx-auto max-w-md space-y-4">
      <section className="space-y-3 rounded-2xl border border-white/20 glass bg-white/10 p-5 text-center">
        <MailCheck className="mx-auto size-10 text-blue-400" aria-hidden />
        <h1 className="text-2xl font-bold">{L("確認メールを送信しました")}</h1>
        {email ? (
          <p className="break-all text-base font-semibold" data-testid="sent-email">
            {email}
          </p>
        ) : null}
        <p className="text-sm text-slate-300">{L("{v0}にメールを送りました。メール内のリンクを開くと登録が終わります。", { v0: email ? L("このアドレス") : L("入力したメールアドレス") })}</p>
      </section>

      <section className="space-y-2 rounded-2xl border border-white/20 glass bg-white/10 p-4 text-sm">
        <h2 className="font-semibold">{L("メールが届かないとき")}</h2>
        <ul className="list-disc space-y-1 pl-5 text-slate-300">
          <li>{L("数分ほど待ってから、もう一度受信箱を見てください。")}</li>
          <li>{L("迷惑メールのフォルダに入っていないか見てください。")}</li>
          <li>{L("アドレスに間違いがあるときは、登録画面からやり直してください。")}</li>
        </ul>
        {email ? (
          <div className="space-y-3 pt-2">
            {captcha.widget}
            {error ? <p className="text-red-400">{error}</p> : null}
            {message ? <p className="text-emerald-400">{message}</p> : null}
            <Button type="button" variant="secondary" className="w-full" onClick={resend} disabled={pending || !captcha.ready}>{L("確認メールを再送する")}</Button>
          </div>
        ) : null}
      </section>

      <p className="text-center text-sm">
        <Link href="/register" className="inline-block text-blue-400 transition active:scale-[0.98]">{L("登録画面に戻る")}</Link>
        <span className="mx-2 text-slate-400">{L("・")}</span>
        <Link href="/login" className="inline-block text-blue-400 transition active:scale-[0.98]">{L("ログイン")}</Link>
      </p>
    </div>
  );
}
