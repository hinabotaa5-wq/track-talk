"use client";

import { useAuth } from "@/components/auth-provider";
import { SenseNotesCard } from "@/components/sense-notes-card";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useRef, useState } from "react";
import { Award, Bookmark, ChevronRight, FileText, HeartPulse, KeyRound, LayoutGrid, LogOut, Mail, ScrollText, Shield, Smartphone, UserRoundPen, UserX, type LucideIcon } from "lucide-react";
import { createClient } from "@/lib/supabase";
import { useActionLock } from "@/lib/use-action-lock";
import { installGuideOpenEvent } from "@/lib/install-guide";

const accountLinks = [
  { href: "/me/posts", label: "自分の投稿", icon: LayoutGrid },
  { href: "/me/saved", label: "保存した投稿", icon: Bookmark },
  { href: "/me/injury", label: "自分のケガ履歴", icon: HeartPulse },
  { href: "/me/profile", label: "ニックネームの変更", icon: UserRoundPen },
  { href: "/me/account", label: "メールアドレスとパスワードの変更", icon: KeyRound },
  { href: "/me/badge", label: "バッジ", icon: Award },
] as const;

const infoLinks = [
  { href: "/me/contact", label: "お問い合わせ", icon: Mail },
  { href: "/guidelines", label: "ガイドライン", icon: ScrollText },
  { href: "/terms", label: "利用規約", icon: FileText },
  { href: "/privacy", label: "プライバシーポリシー", icon: Shield },
] as const;

const menuRowClass =
  "flex min-h-12 w-full items-center gap-3 px-4 text-left text-sm font-medium text-slate-100 transition hover:bg-white/12 active:scale-95 active:bg-white/16 disabled:opacity-50";

function MenuLink({ href, label, icon: Icon }: { href: string; label: string; icon: LucideIcon }) {
  return (
    <Link href={href} className={menuRowClass}>
      <Icon className="size-4 shrink-0 text-slate-300" aria-hidden />
      <span className="flex-1">{label}</span>
      <ChevronRight className="size-4 shrink-0 text-slate-400" aria-hidden />
    </Link>
  );
}

export default function MePage() {
  const { session, token, loading } = useAuth();
  const router = useRouter();
  const logoutLock = useActionLock();
  const [installed, setInstalled] = useState(true);
  // ログアウト中は、未ログインのリダイレクトでホームへの移動を上書きしない。
  const leaving = useRef(false);

  useEffect(() => {
    function sync() {
      const nav = navigator as Navigator & { standalone?: boolean };
      setInstalled(nav.standalone === true || window.matchMedia("(display-mode: standalone)").matches);
    }
    sync();
  }, []);

  async function logout() {
    if (!confirm("ログアウトしますか？")) return;
    if (!logoutLock.acquire()) return;
    leaving.current = true;
    const supabase = createClient();
    await supabase?.auth.signOut();
    router.push("/");
    router.refresh();
  }

  useEffect(() => {
    if (!loading && !session && !leaving.current) router.replace("/login");
  }, [loading, session, router]);

  if (loading || !session) {
    return <p className="text-sm text-slate-200">読み込み中...</p>;
  }

  return (
    <div className="mx-auto max-w-2xl space-y-4">
      <SenseNotesCard token={token} />
      <nav aria-label="アカウント" className="divide-y divide-white/15 overflow-hidden rounded-2xl border border-white/20 glass bg-white/10">
        {accountLinks.map((item) => (
          <MenuLink key={item.href} {...item} />
        ))}
        {installed ? null : (
          <button type="button" onClick={() => window.dispatchEvent(new Event(installGuideOpenEvent))} className={menuRowClass}>
            <Smartphone className="size-4 shrink-0 text-slate-300" aria-hidden />
            <span className="flex-1">ホーム画面に追加して使う</span>
            <ChevronRight className="size-4 shrink-0 text-slate-400" aria-hidden />
          </button>
        )}
        {infoLinks.map((item) => (
          <MenuLink key={item.href} {...item} />
        ))}
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
