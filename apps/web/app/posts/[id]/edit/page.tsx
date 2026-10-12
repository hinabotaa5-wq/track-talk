"use client";

import { PostForm } from "@/components/post-form";
import { RecruitmentForm } from "@/components/recruitment-form";
import { useAuth } from "@/components/auth-provider";
import { apiFetch } from "@/lib/api";
import type { Post } from "@/lib/types";
import { useEffect, useState } from "react";
import { useParams, useRouter } from "next/navigation";

export default function EditPostPage() {
  const { id } = useParams<{ id: string }>();
  const { session, token, loading } = useAuth();
  const router = useRouter();
  const [post, setPost] = useState<Post | null>(null);

  useEffect(() => {
    if (!loading && !session) router.replace("/login");
  }, [loading, session, router]);

  useEffect(() => {
    if (!token) return;
    apiFetch<Post>(`/v1/posts/${id}`, { token }).then(setPost);
  }, [id, token]);

  if (loading || !post) return <p>読み込み中…</p>;
  if (!post.mine) {
    return <p>この投稿を編集する権限がありません。</p>;
  }

  const recruitment = post.kind === "recruitment";

  return (
    <div className="mx-auto max-w-2xl space-y-4">
      <h1 className="text-2xl font-bold">{recruitment ? "募集を編集" : "投稿を編集"}</h1>
      {recruitment ? <RecruitmentForm initial={post} /> : <PostForm initial={post} />}
    </div>
  );
}
