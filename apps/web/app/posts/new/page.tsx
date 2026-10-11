"use client";

import { PostForm } from "@/components/post-form";
import { useAuth } from "@/components/auth-provider";
import { apiFetch } from "@/lib/api";
import type { Category, Competition } from "@/lib/types";
import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";

export default function NewPostPage() {
  const { session, loading } = useAuth();
  const router = useRouter();
  const [categories, setCategories] = useState<Category[]>([]);
  const [competitions, setCompetitions] = useState<Competition[]>([]);

  useEffect(() => {
    if (!loading && !session) router.replace("/login");
  }, [loading, session, router]);

  useEffect(() => {
    apiFetch<{ categories: Category[] }>("/v1/categories").then((d) =>
      setCategories(d.categories ?? []),
    );
    apiFetch<{ competitions: Competition[] }>("/v1/competitions?per_page=50").then((d) =>
      setCompetitions(d.competitions ?? []),
    );
  }, []);

  return (
    <div className="mx-auto max-w-2xl space-y-4">
      <h1 className="text-2xl font-bold">投稿を作成</h1>
      <p className="text-sm text-zinc-600">
        公式記録の転載ではなく、自分の視点での議論を書いてください。
      </p>
      <PostForm categories={categories} competitions={competitions} />
      <Link href="/posts" className="text-sm text-zinc-500">
        一覧に戻る
      </Link>
    </div>
  );
}
