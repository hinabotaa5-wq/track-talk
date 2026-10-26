"use client";

import { authErrorMessage } from "@/lib/auth-errors";
import { createClient } from "@/lib/supabase";
import { useActionLock } from "@/lib/use-action-lock";
import { resetSchema } from "@/lib/validators";
import { zodResolver } from "@hookform/resolvers/zod";
import { useForm } from "react-hook-form";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { useCaptcha } from "@/components/auth-captcha";
import { useState } from "react";
import type { z } from "zod";

export default function ResetPasswordPage() {
  const [message, setMessage] = useState("");
  const { pending, acquire, release } = useActionLock();
  const captcha = useCaptcha();
  const form = useForm<z.infer<typeof resetSchema>>({
    resolver: zodResolver(resetSchema),
    defaultValues: { email: "" },
  });

  async function onSubmit(values: z.infer<typeof resetSchema>) {
    const supabase = createClient();
    if (!supabase || !captcha.ready || !acquire()) return;
    // リンクを開いたら、新しいパスワードを設定する画面に戻す(Supabase の Redirect URLs にこの URL を入れておく)。
    const redirectTo = `${process.env.NEXT_PUBLIC_SITE_URL ?? "http://localhost:3000"}/reset-password/update`;
    const { error } = await supabase.auth
      .resetPasswordForEmail(values.email, { redirectTo, captchaToken: captcha.captchaToken })
      .finally(() => {
        // 確認のトークンは1回しか使えないので、成否にかかわらず取り直す。
        captcha.reset();
        release();
      });
    setMessage(
      error
        ? authErrorMessage(error)
        : "リセット用メールを送信しました。\n届かない場合は迷惑メールに入っているかアカウントを作成していない可能性があります。",
    );
  }

  return (
    <div className="mx-auto max-w-md space-y-4">
      <h1 className="text-2xl font-bold">パスワードリセット</h1>
      <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-3">
        <Input type="email" placeholder="メールアドレス" {...form.register("email")} />
        {captcha.widget}
        {message ? <p className="whitespace-pre-line text-sm">{message}</p> : null}
        <Button type="submit" className="w-full" disabled={pending || !captcha.ready}>
          送信
        </Button>
      </form>
    </div>
  );
}
