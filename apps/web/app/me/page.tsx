"use client";

import { useAuth } from "@/components/auth-provider";
import { apiFetch } from "@/lib/api";
import { FeedCard } from "@/components/feed-card";
import type { Meta, Post } from "@/lib/types";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { ChevronRight, FileText, LogOut, Mail, ScrollText, Shield, UserX } from "lucide-react";
import { createClient } from "@/lib/supabase";
import { useActionLock } from "@/lib/use-action-lock";

const menuLinks = [
  { href: "/me/contact", label: "お問い合わせ", icon: Mail },
  { href: "/guidelines", label: "ガイドライン", icon: ScrollText },
  { href: "/terms", label: "利用規約", icon: FileText },
  { href: "/privacy", label: "プライバシーポリシー", icon: Shield },
] as const;

function menuRowClass(danger = false) {
  return `flex min-h-12 w-full items-center gap-3 px-4 text-left text-sm font-medium transition hover:bg-white/12 active:scale-95 active:bg-white/16 disabled:opacity-50 ${
    danger ? "text-red-300" : "text-slate-100"
  }`;
}

export default function MePage() {
  const { session, token, loading } = useAuth();
  const router = useRouter();
  const [posts, setPosts] = useState<Post[]>([]);
  const [postsError, setPostsError] = useState("");

  const logoutLock = useActionLock();

  async function logout() {
    if (!logoutLock.acquire()) return;
    const supabase = createClient();
    await supabase?.auth.signOut();
    router.push("/");
    router.refresh();
  }

  useEffect(() => {
    if (!loading && !session) router.replace("/login");
  }, [loading, session, router]);

  useEffect(() => {
    if (!token) return;
    apiFetch<{ posts: Post[]; meta: Meta }>("/v1/me/posts", { token })
      .then((d) => {
        setPosts(d.posts ?? []);
        setPostsError("");
      })
      .catch(() => setPostsError("投稿の取得に失敗しました"));
  }, [token]);

  if (loading || !session) {
    return <p className="text-sm text-slate-200">読み込み中...</p>;
  }

  return (
    <div className="space-y-4 md:grid md:grid-cols-[18rem_minmax(0,1fr)] md:items-start md:gap-8 md:space-y-0">
      <nav aria-label="アカウント" className="divide-y divide-white/15 overflow-hidden rounded-2xl border border-white/20 glass bg-white/10">
        <button type="button" onClick={logout} disabled={logoutLock.pending} className={menuRowClass()}>
          <LogOut className="size-4 shrink-0 text-slate-300" aria-hidden />
          <span className="flex-1">ログアウト</span>
        </button>
        {menuLinks.map((item) => {
          const Icon = item.icon;
          return (
            <Link key={item.href} href={item.href} className={menuRowClass()}>
              <Icon className="size-4 shrink-0 text-slate-300" aria-hidden />
              <span className="flex-1">{item.label}</span>
              <ChevronRight className="size-4 shrink-0 text-slate-400" aria-hidden />
            </Link>
          );
        })}
        <Link href="/me/delete" className={menuRowClass(true)}>
          <UserX className="size-4 shrink-0" aria-hidden />
          <span className="flex-1">退会する</span>
          <ChevronRight className="size-4 shrink-0 text-red-300/70" aria-hidden />
        </Link>
      </nav>
      <div className="space-y-4">
        <h2 className="text-xl font-semibold">自分の投稿</h2>
        {postsError ? (
          <p className="text-sm text-red-400">{postsError}</p>
        ) : posts.length === 0 ? (
          <p className="text-sm text-slate-200">まだ投稿がありません</p>
        ) : (
          <div className="grid grid-cols-2 gap-3 lg:grid-cols-3">
            {posts.map((p) => (
              <FeedCard key={p.id} post={p} returnTo="me" />
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
