"use client";

import { PostForm } from "@/components/post-form";
import { useAuth } from "@/components/auth-provider";
import { useEffect } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";

export default function NewPostPage() {
  const { session, loading } = useAuth();
  const router = useRouter();

  useEffect(() => {
    if (!loading && !session) router.replace("/login");
  }, [loading, session, router]);

  return (
    <div className="mx-auto max-w-2xl space-y-4">
      <h1 className="text-2xl font-bold">投稿を作成</h1>
      <p className="text-sm text-zinc-600">
        公式記録の転載ではなく、自分の視点での議論を書いてください。
      </p>
      <PostForm />
      <Link href="/upload" className="text-sm text-zinc-500">
        投稿の入口に戻る
      </Link>
    </div>
  );
}
