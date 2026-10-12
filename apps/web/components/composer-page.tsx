"use client";

import { useEffect, type InputHTMLAttributes, type ReactNode } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/components/auth-provider";
import { Input } from "@/components/ui/input";

export function ComposerPage({
  title,
  children,
}: {
  title: string;
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
      {children}
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
