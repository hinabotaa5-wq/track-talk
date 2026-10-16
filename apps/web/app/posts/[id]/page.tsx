import type { Metadata } from "next";
import { cache } from "react";
import { ApiRequestError, apiFetch } from "@/lib/api";
import { JsonLd } from "@/components/json-ld";
import { siteName, siteUrl, snippet } from "@/lib/site";
import type { Comment, Meta, Post } from "@/lib/types";
import { PostDetail } from "./post-detail";

type Props = { params: Promise<{ id: string }>; searchParams: Promise<{ from?: string }> };

type Loaded = { kind: "ok"; post: Post; comments: Comment[] } | { kind: "hidden" } | { kind: "error" };

// ログインなしで読める投稿を、サーバーで1回だけ読む。metadata と本文の両方で使う。
// 本文をサーバーで描いておくと、検索エンジンが中身を読める。
const loadPost = cache(async (id: string): Promise<Loaded> => {
  try {
    const [post, comments] = await Promise.all([
      apiFetch<Post>(`/v1/posts/${encodeURIComponent(id)}`, { signal: AbortSignal.timeout(3000) }),
      apiFetch<{ comments: Comment[]; meta: Meta }>(`/v1/posts/${encodeURIComponent(id)}/comments?per_page=50`, {
        signal: AbortSignal.timeout(3000),
      }),
    ]);
    return { kind: "ok", post, comments: comments.comments ?? [] };
  } catch (err) {
    // API が一時的に落ちているだけのときに noindex を返すと、載っていたページが検索結果から消えてしまう。
    const hidden = err instanceof ApiRequestError && [401, 403, 404].includes(err.status);
    return { kind: hidden ? "hidden" : "error" };
  }
});

// ログインなしで読めない投稿(ケガの投稿など)や消された投稿は、検索結果に出さない。
export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { id } = await params;
  const loaded = await loadPost(id);
  if (loaded.kind !== "ok") {
    return { title: "投稿", ...(loaded.kind === "hidden" ? { robots: { index: false, follow: true } } : {}) };
  }
  const { post } = loaded;
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

// 掲示板の投稿だと検索エンジンに伝える。ニックネームを公開していない人は「匿名」にする。
function postJsonLd(post: Post, comments: Comment[]) {
  const url = `${siteUrl}/posts/${post.id}`;
  return {
    "@context": "https://schema.org",
    "@type": "DiscussionForumPosting",
    "@id": url,
    url,
    headline: post.title,
    text: snippet(post.body, 500) || post.title,
    datePublished: post.created_at,
    dateModified: post.updated_at,
    author: { "@type": "Person", name: post.author_name || "匿名" },
    ...(post.thumbnail_url ? { image: post.thumbnail_url } : {}),
    interactionStatistic: [
      { "@type": "InteractionCounter", interactionType: "https://schema.org/LikeAction", userInteractionCount: post.like_count },
      { "@type": "InteractionCounter", interactionType: "https://schema.org/CommentAction", userInteractionCount: post.comment_count },
    ],
    comment: comments.slice(0, 20).map((c) => ({
      "@type": "Comment",
      text: snippet(c.body, 300),
      datePublished: c.created_at,
      author: { "@type": "Person", name: c.author_name || "匿名" },
    })),
  };
}

export default async function PostPage({ params, searchParams }: Props) {
  const { id } = await params;
  const { from } = await searchParams;
  const loaded = await loadPost(id);
  const initial = loaded.kind === "ok" ? { post: loaded.post, comments: loaded.comments } : undefined;
  return (
    <>
      {initial && initial.post.kind !== "injury" ? <JsonLd data={postJsonLd(initial.post, initial.comments)} /> : null}
      <PostDetail id={id} returnTo={from === "me" ? "me" : undefined} initial={initial} />
    </>
  );
}
