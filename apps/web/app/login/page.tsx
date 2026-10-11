"use client";

import { createClient } from "@/lib/supabase";
import { loginSchema } from "@/lib/validators";
import { zodResolver } from "@hookform/resolvers/zod";
import { useForm } from "react-hook-form";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState } from "react";
import type { z } from "zod";

export default function LoginPage() {
  const router = useRouter();
  const [error, setError] = useState("");
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
    const { error: err } = await supabase.auth.signInWithPassword(values);
    if (err) {
      setError(err.message);
      return;
    }
    router.push("/");
    router.refresh();
  }

  return (
    <div className="mx-auto max-w-md space-y-4">
      <h1 className="text-2xl font-bold">ログイン</h1>
      <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-3">
        <Input type="email" placeholder="メール" {...form.register("email")} />
        <Input type="password" placeholder="パスワード" {...form.register("password")} />
        {error ? <p className="text-sm text-red-600">{error}</p> : null}
        <Button type="submit" className="w-full md:w-auto">ログイン</Button>
      </form>
      <p className="text-sm">
        アカウントがない場合は <Link href="/register" className="text-blue-700">登録</Link>
      </p>
      <p className="text-sm">
        <Link href="/reset-password" className="text-blue-700">
          パスワードを忘れた
        </Link>
      </p>
      <p className="text-sm">
        <Link href="/about" className="text-zinc-500">
          このアプリについて
        </Link>
      </p>
    </div>
  );
}
