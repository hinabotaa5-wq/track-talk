"use client";

import { authErrorMessage } from "@/lib/auth-errors";
import { createClient } from "@/lib/supabase";
import { useActionLock } from "@/lib/use-action-lock";
import { resetSchema } from "@/lib/validators";
import { zodResolver } from "@hookform/resolvers/zod";
import { useForm } from "react-hook-form";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { useState } from "react";
import type { z } from "zod";

export default function ResetPasswordPage() {
  const [message, setMessage] = useState("");
  const { pending, acquire, release } = useActionLock();
  const form = useForm<z.infer<typeof resetSchema>>({
    resolver: zodResolver(resetSchema),
    defaultValues: { email: "" },
  });

  async function onSubmit(values: z.infer<typeof resetSchema>) {
    const supabase = createClient();
    if (!supabase || !acquire()) return;
    const redirectTo = `${process.env.NEXT_PUBLIC_SITE_URL ?? "http://localhost:3000"}/login`;
    const { error } = await supabase.auth.resetPasswordForEmail(values.email, { redirectTo }).finally(release);
    setMessage(error ? authErrorMessage(error) : "リセット用メールを送信しました。");
  }

  return (
    <div className="mx-auto max-w-md space-y-4">
      <h1 className="text-2xl font-bold">パスワードリセット</h1>
      <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-3">
        <Input type="email" placeholder="メール" {...form.register("email")} />
        {message ? <p className="text-sm">{message}</p> : null}
        <Button type="submit" className="w-full" disabled={pending}>
          送信
        </Button>
      </form>
    </div>
  );
}
