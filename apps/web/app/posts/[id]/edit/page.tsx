"use client";

import { PostForm } from "@/components/post-form";
import { useAuth } from "@/components/auth-provider";
import { apiFetch } from "@/lib/api";
import type { Category, Competition, Post } from "@/lib/types";
import { useEffect, useState } from "react";
import { useParams, useRouter } from "next/navigation";

export default function EditPostPage() {
  const { id } = useParams<{ id: string }>();
  const { session, profile, loading } = useAuth();
  const router = useRouter();
  const [post, setPost] = useState<Post | null>(null);
  const [categories, setCategories] = useState<Category[]>([]);
  const [competitions, setCompetitions] = useState<Competition[]>([]);

  useEffect(() => {
    if (!loading && !session) router.replace("/login");
  }, [loading, session, router]);

  useEffect(() => {
    apiFetch<Post>(`/v1/posts/${id}`).then(setPost);
    apiFetch<{ categories: Category[] }>("/v1/categories").then((d) =>
      setCategories(d.categories ?? []),
    );
    apiFetch<{ competitions: Competition[] }>("/v1/competitions?per_page=50").then((d) =>
      setCompetitions(d.competitions ?? []),
    );
  }, [id]);

  if (!post) return <p>読み込み中…</p>;
  if (profile && profile.id !== post.author.id) {
    return <p>この投稿を編集する権限がありません。</p>;
  }

  return (
    <div className="mx-auto max-w-2xl space-y-4">
      <h1 className="text-2xl font-bold">投稿を編集</h1>
      <PostForm categories={categories} competitions={competitions} initial={post} />
    </div>
  );
}
