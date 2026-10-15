import type { Metadata } from "next";
import { ApiRequestError, apiFetch } from "@/lib/api";
import { siteName, snippet } from "@/lib/site";
import type { Post } from "@/lib/types";
import { PostDetail } from "./post-detail";

// ログインなしで読めない投稿(ケガの投稿など)や消された投稿は、検索結果に出さない。
export async function generateMetadata({ params }: { params: Promise<{ id: string }> }): Promise<Metadata> {
  const { id } = await params;
  let post: Post;
  try {
    post = await apiFetch<Post>(`/v1/posts/${encodeURIComponent(id)}`, { signal: AbortSignal.timeout(3000) });
  } catch (err) {
    // API が一時的に落ちているだけのときに noindex を返すと、載っていたページが検索結果から消えてしまう。
    const hidden = err instanceof ApiRequestError && [401, 403, 404].includes(err.status);
    return { title: "投稿", ...(hidden ? { robots: { index: false, follow: true } } : {}) };
  }
  const description = snippet(post.body) || undefined;
  return {
    title: post.title,
    description,
    alternates: { canonical: `/posts/${post.id}` },
    robots: post.kind === "injury" ? { index: false, follow: true } : undefined,
    openGraph: {
      type: "article",
      siteName,
      locale: "ja_JP",
      title: post.title,
      description,
      publishedTime: post.created_at,
      modifiedTime: post.updated_at,
      ...(post.thumbnail_url ? { images: [post.thumbnail_url] } : {}),
    },
  };
}

export default async function PostPage({
  params,
  searchParams,
}: {
  params: Promise<{ id: string }>;
  searchParams: Promise<{ from?: string }>;
}) {
  const { id } = await params;
  const { from } = await searchParams;
  return <PostDetail id={id} returnTo={from === "me" ? "me" : undefined} />;
}
