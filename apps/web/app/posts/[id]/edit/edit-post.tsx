"use client";

import { InjuryForm } from "@/components/injury-form";
import { PostForm } from "@/components/post-form";
import { PracticeForm } from "@/components/practice-form";
import { RecruitmentForm } from "@/components/recruitment-form";
import { LeaveGuardArea } from "@/components/leave-guard";
import { useAuth } from "@/components/auth-provider";
import { ApiRequestError, apiFetch } from "@/lib/api";
import type { Post } from "@/lib/types";
import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";

export function EditPost({ id, returnTo }: { id: string; returnTo?: "me" }) {
  const { session, token, loading } = useAuth();
  const router = useRouter();
  const [post, setPost] = useState<Post | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    if (!loading && !session) router.replace("/login");
  }, [loading, session, router]);

  useEffect(() => {
    if (!token) return;
    // id や token が変わったら、前の読み込みの結果は使わない。
    let alive = true;
    apiFetch<Post>(`/v1/posts/${id}`, { token })
      .then((p) => {
        if (!alive) return;
        setPost(p);
        setError("");
      })
      .catch((e) => {
        if (!alive) return;
        setError(e instanceof ApiRequestError && e.status === 404 ? "投稿が見つかりません" : "読み込みに失敗しました");
      });
    return () => {
      alive = false;
    };
  }, [id, token]);

  if (error && !post) return <p className="text-red-400">{error}</p>;
  if (loading || !post) return <p>読み込み中…</p>;
  if (!post.mine) {
    return <p>この投稿を編集する権限がありません。</p>;
  }

  const recruitment = post.kind === "recruitment";
  const practice = post.kind === "practice";
  const injury = post.kind === "injury";
  const heading = practice ? "練習相手募集を編集" : recruitment ? "募集を編集" : injury ? "ケガの投稿を編集" : "投稿を編集";

  return (
    <LeaveGuardArea className="mx-auto max-w-2xl space-y-4">
      <h1 className="text-2xl font-bold">{heading}</h1>
      {practice ? (
        <PracticeForm initial={post} returnTo={returnTo} />
      ) : injury ? (
        <InjuryForm initial={post} returnTo={returnTo} />
      ) : recruitment ? (
        <RecruitmentForm initial={post} returnTo={returnTo} />
      ) : (
        <PostForm initial={post} returnTo={returnTo} />
      )}
    </LeaveGuardArea>
  );
}
