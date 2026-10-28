"use client";

import { PageLoading } from "@/components/page-loading";

import { authErrorMessage } from "@/lib/auth-errors";
import { createClient } from "@/lib/supabase";
import { useActionLock } from "@/lib/use-action-lock";
import { isRecentRecoverySession } from "@/lib/password-recovery";
import { newPasswordSchema } from "@/lib/validators";
import { zodResolver } from "@hookform/resolvers/zod";
import { useForm } from "react-hook-form";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import Link from "next/link";
import { useEffect, useState } from "react";
import type { z } from "zod";

// リセット用メールのリンクから開く画面。リンクの code でログインした状態になるので、新しいパスワードを設定する。
// ふつうのログインのまま開いても設定させない(いまのパスワードの確認なしに変えられてしまうため)。ログイン中の変更はマイページで行う。
export default function UpdatePasswordPage() {
  // Supabase の設定がない環境では、リンクを確かめられない。
  const [ready, setReady] = useState<"checking" | "ok" | "expired">(() =>
    process.env.NEXT_PUBLIC_SUPABASE_URL && process.env.NEXT_PUBLIC_SUPABASE_ANON_KEY ? "checking" : "expired",
  );
  const [message, setMessage] = useState("");
  const [done, setDone] = useState(false);
  const { pending, acquire, release } = useActionLock();
  const form = useForm<z.infer<typeof newPasswordSchema>>({
    resolver: zodResolver(newPasswordSchema),
    defaultValues: { password: "", confirm: "" },
  });

  useEffect(() => {
    const supabase = createClient();
    if (!supabase) return;
    let alive = true;
    // URL の code はクライアントが読み込み時にセッションへ交換する。再設定リンクなら PASSWORD_RECOVERY が届き、
    // トークンの amr にも再設定リンクでのログインが残る。どちらもないふつうのログインは期限切れと同じ扱いにする。
    const { data } = supabase.auth.onAuthStateChange((event, session) => {
      if (!alive || !session) return;
      if (event === "PASSWORD_RECOVERY" || isRecentRecoverySession(session.access_token)) setReady("ok");
    });
    void supabase.auth.getSession().then(({ data: { session } }) => {
      if (alive) setReady((current) => (current === "ok" || isRecentRecoverySession(session?.access_token) ? "ok" : "expired"));
    });
    return () => {
      alive = false;
      data.subscription.unsubscribe();
    };
  }, []);

  async function onSubmit(values: z.infer<typeof newPasswordSchema>) {
    const supabase = createClient();
    if (!supabase || !acquire()) return;
    const { error } = await supabase.auth.updateUser({ password: values.password }).finally(release);
    if (error) {
      setMessage(authErrorMessage(error));
      return;
    }
    setDone(true);
  }

  return (
    <div className="mx-auto max-w-md space-y-4">
      <h1 className="text-2xl font-bold">新しいパスワード</h1>
      {done ? (
        <p className="text-sm">
          パスワードを変更しました。<Link href="/" className="underline">ホームへ</Link>
        </p>
      ) : ready === "checking" ? (
        <PageLoading label="確認しています…" />
      ) : ready === "expired" ? (
        <p className="text-sm">
          リンクの有効期限が切れたか、リセットを申し込んだのとは別のブラウザで開いています。
          <Link href="/reset-password" className="underline">もう一度メールを送ってください</Link>。
          ログイン中に変えるときは、<Link href="/me/password" className="underline">マイページのパスワードの変更</Link>から変えられます。
        </p>
      ) : (
        <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-3">
          <Input type="password" autoComplete="new-password" placeholder="新しいパスワード(8文字以上)" {...form.register("password")} />
          {form.formState.errors.password ? <p className="text-sm text-red-400">{form.formState.errors.password.message}</p> : null}
          <Input type="password" autoComplete="new-password" placeholder="もう一度" {...form.register("confirm")} />
          {form.formState.errors.confirm ? <p className="text-sm text-red-400">{form.formState.errors.confirm.message}</p> : null}
          {message ? <p className="text-sm text-red-400">{message}</p> : null}
          <Button type="submit" className="w-full" disabled={pending}>
            変更する
          </Button>
        </form>
      )}
    </div>
  );
}
