import { apiFetch } from "@/lib/api";
import { PostCard } from "@/components/post-card";
import { EmptyState, ErrorState } from "@/components/empty-state";
import type { Meta, Post, Profile } from "@/lib/types";

export const dynamic = "force-dynamic";

export default async function UserPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  let user: Profile | null = null;
  let posts: Post[] = [];
  let failed = false;
  try {
    const [u, list] = await Promise.all([
      apiFetch<Profile>(`/v1/users/${id}`),
      apiFetch<{ posts: Post[]; meta: Meta }>(`/v1/users/${id}/posts`),
    ]);
    user = u;
    posts = list.posts ?? [];
  } catch {
    failed = true;
  }
  if (failed || !user) {
    return <ErrorState message="ユーザーを取得できませんでした。" />;
  }
  return (
    <div className="space-y-4">
      <h1 className="text-2xl font-bold">{user.nickname}</h1>
      <p className="whitespace-pre-wrap text-zinc-700">{user.bio || "自己紹介は未設定です。"}</p>
      <h2 className="text-xl font-semibold">投稿</h2>
      {posts.length === 0 ? (
        <EmptyState title="投稿はまだありません" body="" />
      ) : (
        posts.map((p) => <PostCard key={p.id} post={p} />)
      )}
    </div>
  );
}
