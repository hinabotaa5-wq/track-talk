"use client";

import Link from "next/link";
import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/components/auth-provider";
import { EmptyState } from "@/components/empty-state";

export default function OwnVideoPage() {
  const { session, loading } = useAuth();
  const router = useRouter();

  useEffect(() => {
    if (!loading && !session) router.replace("/login");
  }, [loading, session, router]);

  if (loading || !session) return <p>読み込み中…</p>;

  return (
    <div className="mx-auto max-w-2xl space-y-4">
      <h1 className="text-2xl font-bold">自分の動画</h1>
      <EmptyState title="準備中" body="自分の動画の投稿は、もうしばらくお待ちください。" />
      <Link href="/upload" className="text-sm text-zinc-500">
        投稿の入口に戻る
      </Link>
    </div>
  );
}
