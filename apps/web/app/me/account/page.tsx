"use client";

import { useAuth } from "@/components/auth-provider";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { authErrorMessage } from "@/lib/auth-errors";
import { rememberNextPath } from "@/lib/safe-next";
import { createClient } from "@/lib/supabase";
import { useActionLock } from "@/lib/use-action-lock";
import { changeEmailSchema, changePasswordSchema } from "@/lib/validators";
import { zodResolver } from "@hookform/resolvers/zod";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { useForm } from "react-hook-form";
import type { z } from "zod";

// 変える前に、いまのパスワードで本人かどうかを確かめる。
async function confirmCurrentPassword(email: string, password: string) {
  const supabase = createClient();
  if (!supabase) return "Supabase の公開設定がありません。";
  const { error } = await supabase.auth.signInWithPassword({ email, password });
  return error ? authErrorMessage(error) : "";
}

function EmailSection({ email }: { email: string }) {
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const { pending, acquire, release } = useActionLock();
  const form = useForm<z.infer<typeof changeEmailSchema>>({
    resolver: zodResolver(changeEmailSchema),
    defaultValues: { email: "", current: "" },
  });

  async function onSubmit(values: z.infer<typeof changeEmailSchema>) {
    const supabase = createClient();
    if (!supabase || !acquire()) return;
    setError("");
    setMessage("");
    try {
      if (values.email.toLowerCase() === email.toLowerCase()) {
        setError("いまのメールアドレスと同じです。");
        return;
      }
      const wrong = await confirmCurrentPassword(email, values.current);
      if (wrong) {
        setError(wrong);
        return;
      }
      rememberNextPath("/me/account", 86400);
      const { error: err } = await supabase.auth.updateUser({ email: values.email }, { emailRedirectTo: `${window.location.origin}/auth/callback` });
      if (err) {
        setError(authErrorMessage(err));
        return;
      }
      form.reset();
      setMessage("確認メールを送りました。メール内のリンクを開くと、メールアドレスが変わります。届かないときは迷惑メールを確認してください。");
    } finally {
      release();
    }
  }

  return (
    <section className="space-y-3 rounded-2xl border border-white/20 glass bg-white/10 p-4">
      <h2 className="text-base font-bold">メールアドレスの変更</h2>
      <p className="text-sm text-slate-300">いま: {email}</p>
      <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-3">
        <Input type="email" autoComplete="email" placeholder="新しいメールアドレス" {...form.register("email")} />
        {form.formState.errors.email ? <p className="text-sm text-red-400">{form.formState.errors.email.message}</p> : null}
        <Input type="password" autoComplete="current-password" placeholder="いまのパスワード" {...form.register("current")} />
        {form.formState.errors.current ? <p className="text-sm text-red-400">{form.formState.errors.current.message}</p> : null}
        {error ? <p className="text-sm text-red-400">{error}</p> : null}
        {message ? <p className="text-sm text-emerald-300">{message}</p> : null}
        <Button type="submit" className="w-full" disabled={pending}>
          変更する
        </Button>
      </form>
    </section>
  );
}

function PasswordSection({ email }: { email: string }) {
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const { pending, acquire, release } = useActionLock();
  const form = useForm<z.infer<typeof changePasswordSchema>>({
    resolver: zodResolver(changePasswordSchema),
    defaultValues: { current: "", password: "", confirm: "" },
  });

  async function onSubmit(values: z.infer<typeof changePasswordSchema>) {
    const supabase = createClient();
    if (!supabase || !acquire()) return;
    setError("");
    setMessage("");
    try {
      const wrong = await confirmCurrentPassword(email, values.current);
      if (wrong) {
        setError(wrong);
        return;
      }
      const { error: err } = await supabase.auth.updateUser({ password: values.password });
      if (err) {
        setError(authErrorMessage(err));
        return;
      }
      form.reset();
      setMessage("パスワードを変更しました。");
    } finally {
      release();
    }
  }

  return (
    <section className="space-y-3 rounded-2xl border border-white/20 glass bg-white/10 p-4">
      <h2 className="text-base font-bold">パスワードの変更</h2>
      <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-3">
        <Input type="password" autoComplete="current-password" placeholder="いまのパスワード" {...form.register("current")} />
        {form.formState.errors.current ? <p className="text-sm text-red-400">{form.formState.errors.current.message}</p> : null}
        <Input type="password" autoComplete="new-password" placeholder="新しいパスワード(8文字以上)" {...form.register("password")} />
        {form.formState.errors.password ? <p className="text-sm text-red-400">{form.formState.errors.password.message}</p> : null}
        <Input type="password" autoComplete="new-password" placeholder="もう一度" {...form.register("confirm")} />
        {form.formState.errors.confirm ? <p className="text-sm text-red-400">{form.formState.errors.confirm.message}</p> : null}
        {error ? <p className="text-sm text-red-400">{error}</p> : null}
        {message ? <p className="text-sm text-emerald-300">{message}</p> : null}
        <Button type="submit" className="w-full" disabled={pending}>
          変更する
        </Button>
      </form>
    </section>
  );
}

export default function AccountPage() {
  const { session, loading } = useAuth();
  const router = useRouter();

  useEffect(() => {
    if (!loading && !session) router.replace("/login?next=/me/account");
  }, [loading, session, router]);

  if (loading || !session) return <p className="text-sm text-slate-200">読み込み中...</p>;

  const email = session.user.email ?? "";
  // Google でログインしている人は、メールアドレスもパスワードも Google 側で管理する。
  const providers = session.user.app_metadata?.providers as string[] | undefined;
  const hasPassword = providers ? providers.includes("email") : session.user.app_metadata?.provider === "email";

  return (
    <div className="mx-auto max-w-md space-y-4">
      <h1 className="text-2xl font-bold">メールアドレスとパスワード</h1>
      {hasPassword && email ? (
        <>
          <EmailSection email={email} />
          <PasswordSection email={email} />
        </>
      ) : (
        <p className="text-sm text-slate-300">Google でログインしているので、メールアドレスとパスワードは Google のアカウントで変更してください。</p>
      )}
    </div>
  );
}
