"use client";

import { useAuth } from "@/components/auth-provider";
import { InjuryMeCards } from "@/components/injury-me-cards";
import { SenseNotesCard } from "@/components/sense-notes-card";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect } from "react";
import { Bookmark, ChevronRight, FileText, HeartPulse, LayoutGrid, LogOut, Mail, ScrollText, Shield, UserRoundPen, UserX } from "lucide-react";
import { createClient } from "@/lib/supabase";
import { useActionLock } from "@/lib/use-action-lock";

const menuLinks = [
  { href: "/me/posts", label: "自分の投稿", icon: LayoutGrid },
  { href: "/me/saved", label: "保存した投稿", icon: Bookmark },
  { href: "/me/injury", label: "自分のケガ履歴", icon: HeartPulse },
  { href: "/me/profile", label: "ニックネームの変更", icon: UserRoundPen },
  { href: "/me/contact", label: "お問い合わせ", icon: Mail },
  { href: "/guidelines", label: "ガイドライン", icon: ScrollText },
  { href: "/terms", label: "利用規約", icon: FileText },
  { href: "/privacy", label: "プライバシーポリシー", icon: Shield },
] as const;

const menuRowClass =
  "flex min-h-12 w-full items-center gap-3 px-4 text-left text-sm font-medium text-slate-100 transition hover:bg-white/12 active:scale-95 active:bg-white/16 disabled:opacity-50";

export default function MePage() {
  const { session, token, loading } = useAuth();
  const router = useRouter();
  const logoutLock = useActionLock();

  async function logout() {
    if (!confirm("ログアウトしますか？")) return;
    if (!logoutLock.acquire()) return;
    const supabase = createClient();
    await supabase?.auth.signOut();
    router.push("/");
    router.refresh();
  }

  useEffect(() => {
    if (!loading && !session) router.replace("/login");
  }, [loading, session, router]);

  if (loading || !session) {
    return <p className="text-sm text-slate-200">読み込み中...</p>;
  }

  return (
    <div className="mx-auto max-w-2xl space-y-4">
      <InjuryMeCards token={token} />
      <SenseNotesCard token={token} />
      <nav aria-label="アカウント" className="divide-y divide-white/15 overflow-hidden rounded-2xl border border-white/20 glass bg-white/10">
        {menuLinks.map((item) => {
          const Icon = item.icon;
          return (
            <Link key={item.href} href={item.href} className={menuRowClass}>
              <Icon className="size-4 shrink-0 text-slate-300" aria-hidden />
              <span className="flex-1">{item.label}</span>
              <ChevronRight className="size-4 shrink-0 text-slate-400" aria-hidden />
            </Link>
          );
        })}
        <button type="button" onClick={logout} disabled={logoutLock.pending} className={menuRowClass}>
          <LogOut className="size-4 shrink-0 text-slate-300" aria-hidden />
          <span className="flex-1">ログアウト</span>
        </button>
        <Link href="/me/delete" className={menuRowClass}>
          <UserX className="size-4 shrink-0 text-slate-300" aria-hidden />
          <span className="flex-1">退会する</span>
          <ChevronRight className="size-4 shrink-0 text-slate-400" aria-hidden />
        </Link>
      </nav>
    </div>
  );
}
