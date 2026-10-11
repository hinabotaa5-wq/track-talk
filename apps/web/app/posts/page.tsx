import { apiFetch } from "@/lib/api";
import { PostCard } from "@/components/post-card";
import { EmptyState, ErrorState } from "@/components/empty-state";
import Link from "next/link";
import type { Meta, Post } from "@/lib/types";

export const dynamic = "force-dynamic";

export default async function PostsPage({
  searchParams,
}: {
  searchParams: Promise<{ sort?: string; page?: string }>;
}) {
  const sp = await searchParams;
  const sort = sp.sort === "popular" ? "popular" : "new";
  const page = sp.page ?? "1";
  const qs = new URLSearchParams({ sort, page, per_page: "20" });

  let posts: Post[] = [];
  let meta: Meta = { page: 1, per_page: 20, total: 0 };
  let error = "";
  try {
    const list = await apiFetch<{ posts: Post[]; meta: Meta }>(`/v1/posts?${qs}`);
    posts = list.posts ?? [];
    meta = list.meta;
  } catch {
    error = "投稿一覧を取得できませんでした。";
  }

  const hasNext = meta.page * meta.per_page < meta.total;

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h1 className="text-2xl font-bold">投稿</h1>
          <p className="text-sm text-zinc-600">新着または人気順でレースの議論を探せます。</p>
        </div>
        <Link href="/posts/new" className="rounded-md bg-blue-600 px-4 py-2 text-sm text-white">
          投稿する
        </Link>
      </div>
      <div className="flex flex-wrap gap-2 text-sm">
        <Link className={sort === "new" ? "font-semibold text-blue-700" : ""} href="/posts?sort=new">
          新着
        </Link>
        <Link
          className={sort === "popular" ? "font-semibold text-blue-700" : ""}
          href="/posts?sort=popular"
        >
          人気
        </Link>
      </div>
      {error ? <ErrorState message={error} /> : null}
      {posts.length === 0 && !error ? (
        <EmptyState title="投稿がありません" body="条件を変えるか、最初の投稿を作成してください。" />
      ) : (
        <div className="space-y-3">
          {posts.map((p) => (
            <PostCard key={p.id} post={p} />
          ))}
        </div>
      )}
      {hasNext ? (
        <Link
          href={`/posts?sort=${sort}&page=${meta.page + 1}`}
          className="block w-full rounded-md border bg-white py-3 text-center text-sm font-medium"
        >
          次の投稿
        </Link>
      ) : null}
    </div>
  );
}
