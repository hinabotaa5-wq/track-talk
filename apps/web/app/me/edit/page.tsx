"use client";

import { useAuth } from "@/components/auth-provider";
import { apiFetch } from "@/lib/api";
import { profileSchema } from "@/lib/validators";
import { zodResolver } from "@hookform/resolvers/zod";
import { useForm } from "react-hook-form";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Button } from "@/components/ui/button";
import { useRouter } from "next/navigation";
import { useEffect } from "react";
import type { z } from "zod";

export default function EditProfilePage() {
  const { session, profile, token, loading, refreshProfile } = useAuth();
  const router = useRouter();
  const form = useForm<z.infer<typeof profileSchema>>({
    resolver: zodResolver(profileSchema),
    defaultValues: { nickname: "", bio: "" },
  });

  useEffect(() => {
    if (!loading && !session) router.replace("/login");
  }, [loading, session, router]);

  useEffect(() => {
    if (profile) form.reset({ nickname: profile.nickname, bio: profile.bio });
  }, [profile, form]);

  async function onSubmit(values: z.infer<typeof profileSchema>) {
    await apiFetch("/v1/me/profile", {
      method: "PUT",
      token,
      body: JSON.stringify(values),
    });
    await refreshProfile();
    router.push("/me");
  }

  return (
    <div className="mx-auto max-w-md space-y-4">
      <h1 className="text-2xl font-bold">プロフィール編集</h1>
      <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-3">
        <Input placeholder="ニックネーム" {...form.register("nickname")} />
        <Textarea rows={5} placeholder="自己紹介" {...form.register("bio")} />
        <Button type="submit">保存</Button>
      </form>
    </div>
  );
}
