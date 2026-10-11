"use client";

import { apiFetch, ApiRequestError } from "@/lib/api";
import { formatDateTime } from "@/lib/format";
import type { Comment, Meta, Post } from "@/lib/types";
import { useAuth } from "@/components/auth-provider";
import { feedTitle } from "@/components/feed-card";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { commentSchema } from "@/lib/validators";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import Link from "next/link";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";

export function PostDetail({ id }: { id: string }) {
  const { session, profile, token } = useAuth();
  const router = useRouter();
  const [post, setPost] = useState<Post | null>(null);
  const [comments, setComments] = useState<Comment[]>([]);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const form = useForm({ resolver: zodResolver(commentSchema), defaultValues: { body: "" } });

  async function load() {
    try {
      const [p, c] = await Promise.all([
        apiFetch<Post>(`/v1/posts/${id}`, { token }),
        apiFetch<{ comments: Comment[]; meta: Meta }>(`/v1/posts/${id}/comments?per_page=50`),
      ]);
      setPost(p);
      setComments(c.comments ?? []);
      setError("");
    } catch (e) {
      if (e instanceof ApiRequestError && e.status === 404) setError("投稿が見つかりません");
      else setError("読み込みに失敗しました");
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    const handle = window.setTimeout(() => {
      void load();
    }, 0);
    return () => window.clearTimeout(handle);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id, token]);

  if (loading) return <p>読み込み中…</p>;
  if (error || !post) return <p className="text-red-700">{error || "エラー"}</p>;

  const current = post;
  const mine = profile?.id === current.author.id;

  async function toggleLike() {
    if (!session) {
      router.push("/login");
      return;
    }
    if (current.liked_by_me) {
      await apiFetch(`/v1/posts/${current.id}/likes`, { method: "DELETE", token });
    } else {
      await apiFetch(`/v1/posts/${current.id}/likes`, { method: "POST", token });
    }
    load();
  }

  async function remove() {
    if (!confirm("この投稿を削除しますか？")) return;
    await apiFetch(`/v1/posts/${current.id}`, { method: "DELETE", token });
    router.push("/posts");
  }

  async function report(targetType: "post" | "comment", targetId: string) {
    if (!session) {
      router.push("/login");
      return;
    }
    await apiFetch("/v1/reports", {
      method: "POST",
      token,
      body: JSON.stringify({
        target_type: targetType,
        target_id: targetId,
        reason: "other",
        detail: "",
      }),
    });
    alert("通報を受け付けました");
  }

  async function onComment(values: { body: string }) {
    if (!session) {
      router.push("/login");
      return;
    }
    await apiFetch(`/v1/posts/${current.id}/comments`, {
      method: "POST",
      token,
      body: JSON.stringify(values),
    });
    form.reset();
    load();
  }

  async function deleteComment(cid: string) {
    await apiFetch(`/v1/comments/${cid}`, { method: "DELETE", token });
    load();
  }

  return (
    <article className="space-y-6 pb-36 md:pb-0">
      <p className="text-sm text-zinc-500">
        <Link href="/posts">投稿</Link>
        {post.competition ? (
          <>
            {" · "}
            <Link href={`/competitions/${post.competition.id}`}>{post.competition.name}</Link>
          </>
        ) : null}
      </p>
      {post.thumbnail_url ? (
        <img src={post.thumbnail_url} alt="" className="aspect-video w-full rounded-xl object-cover" />
      ) : null}
      <h1 className="text-3xl font-bold">{feedTitle(post.title)}</h1>
      <p className="text-sm text-zinc-500">
        {post.category.name} · {post.author.nickname} · {formatDateTime(post.created_at)}
      </p>
      {post.body ? (
        <div className="whitespace-pre-wrap rounded-xl border bg-white p-4 leading-7">{post.body}</div>
      ) : null}
      <div className="flex flex-wrap gap-2">
        <Button type="button" variant={post.liked_by_me ? "primary" : "secondary"} onClick={toggleLike}>
          いいね {post.like_count}
        </Button>
        {mine ? (
          <>
            <Link href={`/posts/${post.id}/edit`} className="rounded-md border px-4 py-2 text-sm">
              編集
            </Link>
            <Button type="button" variant="danger" onClick={remove}>
              削除
            </Button>
          </>
        ) : (
          <Button type="button" variant="ghost" onClick={() => report("post", post.id)}>
            通報
          </Button>
        )}
      </div>
      <section className="space-y-3">
        <h2 className="text-xl font-semibold">コメント</h2>
        {comments.length === 0 ? <p className="text-sm text-zinc-500">まだコメントはありません。</p> : null}
        {comments.map((c) => (
          <div key={c.id} className="rounded-lg border bg-white p-3">
            <p className="whitespace-pre-wrap text-sm">{c.body}</p>
            <p className="mt-2 text-xs text-zinc-500">
              {c.author.nickname} · {formatDateTime(c.created_at)}
            </p>
            {profile?.id === c.author.id ? (
              <div className="mt-1 flex gap-3">
                <button
                  type="button"
                  className="text-xs text-orange-700"
                  onClick={async () => {
                    const body = window.prompt("コメントを編集", c.body);
                    if (!body) return;
                    await apiFetch(`/v1/comments/${c.id}`, {
                      method: "PATCH",
                      token,
                      body: JSON.stringify({ body }),
                    });
                    load();
                  }}
                >
                  編集
                </button>
                <button
                  type="button"
                  className="text-xs text-red-600"
                  onClick={() => deleteComment(c.id)}
                >
                  削除
                </button>
              </div>
            ) : (
              <button type="button" className="mt-1 text-xs text-zinc-500" onClick={() => report("comment", c.id)}>
                通報
              </button>
            )}
          </div>
        ))}
        <form
          onSubmit={form.handleSubmit(onComment)}
          className="fixed inset-x-0 bottom-0 z-20 border-t border-zinc-200 bg-white p-3 pb-[max(0.75rem,env(safe-area-inset-bottom))] md:static md:border-0 md:bg-transparent md:p-0"
        >
          <div className="mx-auto flex w-full max-w-5xl items-end gap-2 md:flex-col md:items-stretch">
            <Textarea rows={2} className="min-h-11 flex-1" placeholder="コメントを書く" {...form.register("body")} />
            <Button type="submit" className="shrink-0 md:w-auto">
              コメントする
            </Button>
          </div>
        </form>
      </section>
    </article>
  );
}
