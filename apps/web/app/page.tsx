import Link from "next/link";
import { apiFetch } from "@/lib/api";
import { PostCard } from "@/components/post-card";
import { EmptyState, ErrorState } from "@/components/empty-state";
import { formatDate, scopeLabel } from "@/lib/format";
import type { Competition, Meta, Post } from "@/lib/types";

export const dynamic = "force-dynamic";

export default async function HomePage() {
  let latest: Post[] = [];
  let popular: Post[] = [];
  let competitions: Competition[] = [];
  let error = "";
  try {
    const [a, b, c] = await Promise.all([
      apiFetch<{ posts: Post[]; meta: Meta }>("/v1/posts?sort=new&per_page=5"),
      apiFetch<{ posts: Post[]; meta: Meta }>("/v1/posts?sort=popular&per_page=5"),
      apiFetch<{ competitions: Competition[] }>("/v1/competitions?per_page=4"),
    ]);
    latest = a.posts ?? [];
    popular = b.posts ?? [];
    competitions = c.competitions ?? [];
  } catch {
    error = "投稿の取得に失敗しました。APIの起動を確認してください。";
  }

  return (
    <div className="space-y-10">
      <section>
        <p className="text-sm font-medium text-orange-700">陸上競技コミュニティ</p>
        <h1 className="mt-1 text-3xl font-bold">TRACK TALK</h1>
        <p className="mt-3 max-w-2xl text-zinc-600">
          記録サイトではなく、レース展開や記録について語り合う場所です。観戦ファンも現役選手も歓迎します。
        </p>
        <div className="mt-4 flex gap-3">
          <Link
            href="/posts"
            className="rounded-md bg-orange-600 px-4 py-2 text-sm font-medium text-white"
          >
            投稿を見る
          </Link>
          <Link href="/posts/new" className="rounded-md border px-4 py-2 text-sm">
            議論を始める
          </Link>
        </div>
      </section>
      {error ? <ErrorState message={error} /> : null}
      <section className="grid gap-8 md:grid-cols-2">
        <div className="space-y-3">
          <h2 className="text-xl font-semibold">新着</h2>
          {latest.length === 0 ? (
            <EmptyState title="まだ投稿がありません" body="最初のレース感想を書いてみましょう。" />
          ) : (
            latest.map((p) => <PostCard key={p.id} post={p} />)
          )}
        </div>
        <div className="space-y-3">
          <h2 className="text-xl font-semibold">人気（直近30日）</h2>
          {popular.length === 0 ? (
            <EmptyState title="まだ人気投稿はありません" body="いいねで盛り上がりましょう。" />
          ) : (
            popular.map((p) => <PostCard key={p.id} post={p} />)
          )}
        </div>
      </section>
      <section>
        <div className="mb-3 flex items-center justify-between">
          <h2 className="text-xl font-semibold">大会</h2>
          <Link href="/competitions" className="text-sm text-orange-700">
            一覧へ
          </Link>
        </div>
        <div className="grid gap-3 sm:grid-cols-2">
          {competitions.map((c) => (
            <Link
              key={c.id}
              href={`/competitions/${c.id}`}
              className="rounded-xl border bg-white p-4 hover:border-orange-300"
            >
              <p className="text-xs text-zinc-500">{scopeLabel(c.scope)}</p>
              <h3 className="font-semibold">{c.name}</h3>
              <p className="text-sm text-zinc-600">{formatDate(c.starts_on)}</p>
            </Link>
          ))}
        </div>
      </section>
    </div>
  );
}
