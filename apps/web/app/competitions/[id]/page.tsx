import { apiFetch, ApiRequestError } from "@/lib/api";
import { formatDate, scopeLabel } from "@/lib/format";
import { PostCard } from "@/components/post-card";
import { EmptyState, ErrorState } from "@/components/empty-state";
import type { Competition, Meta, Post } from "@/lib/types";
import { notFound } from "next/navigation";

export const dynamic = "force-dynamic";

export default async function CompetitionDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  let comp: Competition | null = null;
  let related: Post[] = [];
  let failed = false;
  try {
    const [c, posts] = await Promise.all([
      apiFetch<Competition>(`/v1/competitions/${id}`),
      apiFetch<{ posts: Post[]; meta: Meta }>(`/v1/competitions/${id}/posts?per_page=20`),
    ]);
    comp = c;
    related = posts.posts ?? [];
  } catch (e) {
    if (e instanceof ApiRequestError && e.status === 404) notFound();
    failed = true;
  }
  if (failed || !comp) {
    return <ErrorState message="大会情報を取得できませんでした。" />;
  }
  return (
    <div className="space-y-6">
      <p className="text-xs text-zinc-500">{scopeLabel(comp.scope)}</p>
      <h1 className="text-3xl font-bold">{comp.name}</h1>
      <p className="text-sm text-zinc-600">
        {formatDate(comp.starts_on)}
        {comp.ends_on ? ` 〜 ${formatDate(comp.ends_on)}` : ""} · {comp.location}
      </p>
      <p className="whitespace-pre-wrap text-zinc-700">{comp.summary}</p>
      <section className="space-y-3">
        <h2 className="text-xl font-semibold">関連投稿</h2>
        {related.length === 0 ? (
          <EmptyState title="関連投稿はまだありません" body="この大会についての感想を投稿できます。" />
        ) : (
          related.map((p) => <PostCard key={p.id} post={p} />)
        )}
      </section>
    </div>
  );
}
