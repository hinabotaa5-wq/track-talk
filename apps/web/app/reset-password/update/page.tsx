"use client";

import { authErrorMessage } from "@/lib/auth-errors";
import { createClient } from "@/lib/supabase";
import { useActionLock } from "@/lib/use-action-lock";
import { newPasswordSchema } from "@/lib/validators";
import { zodResolver } from "@hookform/resolvers/zod";
import { useForm } from "react-hook-form";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import Link from "next/link";
import { useEffect, useState } from "react";
import type { z } from "zod";

// リセット用メールのリンクから開く画面。リンクの code でログインした状態になるので、新しいパスワードを設定する。
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
    // URL の code はクライアントが読み込み時にセッションへ交換する。交換が済むと PASSWORD_RECOVERY か SIGNED_IN が届く。
    const { data } = supabase.auth.onAuthStateChange((event, session) => {
      if (alive && session && (event === "PASSWORD_RECOVERY" || event === "SIGNED_IN" || event === "INITIAL_SESSION")) setReady("ok");
    });
    void supabase.auth.getSession().then(({ data: { session } }) => {
      if (alive) setReady(session ? "ok" : "expired");
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
        <p className="text-sm">確認しています…</p>
      ) : ready === "expired" ? (
        <p className="text-sm">
          リンクの有効期限が切れたか、リセットを申し込んだのとは別のブラウザで開いています。
          <Link href="/reset-password" className="underline">もう一度メールを送ってください</Link>
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
