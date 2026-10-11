import Link from "next/link";
import { Search } from "lucide-react";
import { apiFetch } from "@/lib/api";
import { FeedCard } from "@/components/feed-card";
import { SavedFeed } from "@/components/saved-feed";
import { EmptyState, ErrorState } from "@/components/empty-state";
import type { Meta, Post } from "@/lib/types";

export const dynamic = "force-dynamic";

type HomeSort = "popular" | "new" | "saved";

function homeHref(sort: HomeSort, q: string, page?: number) {
  const params = new URLSearchParams();
  if (sort !== "popular") params.set("sort", sort);
  if (q) params.set("q", q);
  if (page && page > 1) params.set("page", String(page));
  const s = params.toString();
  return s ? `/?${s}` : "/";
}

function chipClass(active: boolean) {
  return active
    ? "inline-flex min-h-11 items-center rounded-full bg-blue-600 px-4 text-sm font-medium text-white"
    : "inline-flex min-h-11 items-center rounded-full bg-zinc-100 px-4 text-sm text-zinc-700";
}

export default async function HomePage({
  searchParams,
}: {
  searchParams: Promise<{ sort?: string; q?: string; page?: string }>;
}) {
  const sp = await searchParams;
  const sort: HomeSort = sp.sort === "new" ? "new" : sp.sort === "saved" ? "saved" : "popular";
  const q = (sp.q ?? "").trim();
  const page = Math.max(1, Number(sp.page) || 1);
  const qs = new URLSearchParams({ sort, page: String(page), per_page: "20" });
  if (q) qs.set("q", q);

  let posts: Post[] = [];
  let meta: Meta = { page: 1, per_page: 20, total: 0 };
  let error = "";
  if (sort !== "saved") {
    try {
      const data = await apiFetch<{ posts: Post[]; meta: Meta }>(`/v1/posts?${qs}`);
      posts = data.posts ?? [];
      meta = data.meta;
    } catch {
      error = "投稿の取得に失敗しました。APIの起動を確認してください。";
    }
  }

  const hasNext = meta.page * meta.per_page < meta.total;

  return (
    <div className="-mx-4 -mt-6">
      <h1 className="sr-only">ホーム</h1>
      <form action="/" className="border-b border-zinc-200 bg-white px-4 py-3">
        {sort !== "popular" ? <input type="hidden" name="sort" value={sort} /> : null}
        <div className="relative">
          <Search
            className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-zinc-400"
            aria-hidden
          />
          <input
            type="search"
            name="q"
            defaultValue={q}
            placeholder="キーワード"
            aria-label="キーワード"
            className="h-11 w-full rounded-full border border-zinc-300 bg-white pr-4 pl-10 text-base outline-none focus:border-blue-500"
          />
        </div>
      </form>
      <div className="flex gap-2 px-4 py-3">
        <Link href={homeHref("popular", q)} className={chipClass(sort === "popular")} aria-current={sort === "popular" ? "page" : undefined}>
          人気
        </Link>
        <Link href={homeHref("new", q)} className={chipClass(sort === "new")} aria-current={sort === "new" ? "page" : undefined}>
          新しい順
        </Link>
        <Link href={homeHref("saved", q)} className={chipClass(sort === "saved")} aria-current={sort === "saved" ? "page" : undefined}>
          保存
        </Link>
      </div>
      <div className="space-y-4 px-4 pb-4">
        {sort === "saved" ? <SavedFeed q={q} page={page} /> : null}
        {sort !== "saved" && error ? <ErrorState message={error} /> : null}
        {sort !== "saved" && !error && posts.length === 0 ? (
          <EmptyState
            title={q ? "一致する投稿がありません" : "まだ投稿がありません"}
            body={q ? "別の言葉で探してください。" : "最初のレース感想を書いてみましょう。"}
          />
        ) : null}
        {sort !== "saved" && posts.length > 0 ? (
          <div className="grid grid-cols-2 gap-3">
            {posts.map((post) => (
              <FeedCard key={post.id} post={post} />
            ))}
          </div>
        ) : null}
        {sort !== "saved" && hasNext ? (
          <Link
            href={homeHref(sort, q, meta.page + 1)}
            className="block w-full rounded-md border bg-white py-3 text-center text-sm font-medium"
          >
            次の投稿
          </Link>
        ) : null}
      </div>
    </div>
  );
}
