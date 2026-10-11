"use client";

import Link from "next/link";
import { useEffect, type InputHTMLAttributes, type ReactNode } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/components/auth-provider";
import { Input } from "@/components/ui/input";

export function ComposerPage({
  title,
  lead,
  children,
}: {
  title: string;
  lead: string;
  children: ReactNode;
}) {
  const { session, loading } = useAuth();
  const router = useRouter();

  useEffect(() => {
    if (!loading && !session) router.replace("/login");
  }, [loading, session, router]);

  if (loading || !session) return <p>読み込み中…</p>;

  return (
    <div className="mx-auto max-w-2xl space-y-4">
      <h1 className="text-2xl font-bold">{title}</h1>
      <p className="text-sm text-zinc-600">{lead}</p>
      {children}
      <Link href="/upload" className="text-sm text-zinc-500">
        投稿の入口に戻る
      </Link>
    </div>
  );
}

export function TitleField(props: InputHTMLAttributes<HTMLInputElement>) {
  return (
    <label className="block text-sm font-medium">
      タイトル
      <Input className="mt-1" maxLength={120} {...props} />
    </label>
  );
}
