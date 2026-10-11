"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/components/auth-provider";
import { apiFetch } from "@/lib/api";
import { FeedCard } from "@/components/feed-card";
import { EmptyState, ErrorState } from "@/components/empty-state";
import type { Meta, Post } from "@/lib/types";

export function SavedFeed({ q, page }: { q: string; page: number }) {
  const { session, loading, token } = useAuth();
  const router = useRouter();
  const [posts, setPosts] = useState<Post[]>([]);
  const [meta, setMeta] = useState<Meta>({ page: 1, per_page: 20, total: 0 });
  const [error, setError] = useState("");
  const [ready, setReady] = useState(false);

  useEffect(() => {
    if (!loading && !session) router.replace("/login");
  }, [loading, session, router]);

  useEffect(() => {
    if (!token) return;
    const qs = new URLSearchParams({ page: String(page), per_page: "20" });
    if (q) qs.set("q", q);
    apiFetch<{ posts: Post[]; meta: Meta }>(`/v1/me/saves?${qs}`, { token })
      .then((data) => {
        setPosts(data.posts ?? []);
        setMeta(data.meta);
        setError("");
      })
      .catch(() => setError("保存した投稿の取得に失敗しました。"))
      .finally(() => setReady(true));
  }, [token, q, page]);

  if (loading || !session || !ready) return <p>読み込み中…</p>;

  const hasNext = meta.page * meta.per_page < meta.total;
  const next = new URLSearchParams();
  next.set("sort", "saved");
  if (q) next.set("q", q);
  next.set("page", String(meta.page + 1));

  return (
    <>
      {error ? <ErrorState message={error} /> : null}
      {!error && posts.length === 0 ? (
        <EmptyState
          title={q ? "一致する投稿がありません" : "まだ保存がありません"}
          body={q ? "別の言葉で探してください。" : "投稿の保存ボタンから残せます。"}
        />
      ) : null}
      {posts.length > 0 ? (
        <div className="grid grid-cols-2 gap-3">
          {posts.map((post) => (
            <FeedCard key={post.id} post={post} />
          ))}
        </div>
      ) : null}
      {hasNext ? (
        <Link href={`/?${next}`} className="block w-full rounded-md border bg-white py-3 text-center text-sm font-medium">
          次の投稿
        </Link>
      ) : null}
    </>
  );
}
