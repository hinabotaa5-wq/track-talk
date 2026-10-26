"use client";

import { useAuth } from "@/components/auth-provider";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useRef, useState } from "react";
import { AtSign, Award, Bell, Bookmark, ChevronRight, FileText, HeartPulse, KeyRound, LayoutGrid, LogOut, Mail, ScrollText, Shield, Smartphone, UserRoundPen, UserX, type LucideIcon } from "lucide-react";
import { createClient } from "@/lib/supabase";
import { useActionLock } from "@/lib/use-action-lock";
import { unsubscribePushOnThisDevice } from "@/lib/sign-out";
import { useConfirm } from "@/components/confirm-dialog";

const accountLinks = [
  { href: "/me/posts", label: "自分の投稿", icon: LayoutGrid },
  { href: "/me/saved", label: "保存した投稿", icon: Bookmark },
  { href: "/me/injury", label: "自分のケガ履歴", icon: HeartPulse },
  { href: "/me/profile", label: "ニックネームと種目の変更", icon: UserRoundPen },
  { href: "/me/email", label: "メールアドレスの変更", icon: AtSign },
  { href: "/me/password", label: "パスワードの変更", icon: KeyRound },
  { href: "/me/badge", label: "バッジ", icon: Award },
  { href: "/me/push", label: "通知", icon: Bell },
] as const;

const infoLinks = [
  { href: "/guide", label: "利用ガイド", icon: FileText },
  { href: "/me/contact", label: "お問い合わせ", icon: Mail },
  { href: "/guidelines", label: "ガイドライン", icon: ScrollText },
  { href: "/terms", label: "利用規約", icon: FileText },
  { href: "/privacy", label: "プライバシーポリシー", icon: Shield },
] as const;

const menuRowClass =
  "flex min-h-12 w-full items-center gap-3 px-4 text-left text-sm font-medium text-slate-100 transition hover:bg-white/12 active:scale-[0.98] active:bg-white/16 disabled:opacity-50";

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
  const { session, profile, token, sessionLoading: loading } = useAuth();
  const router = useRouter();
  const logoutLock = useActionLock();
  const confirmDialog = useConfirm();
  const [logoutError, setLogoutError] = useState("");
  // ログアウト中は、未ログインのリダイレクトでホームへの移動を上書きしない。
  const leaving = useRef(false);

  async function logout() {
    if (!(await confirmDialog({ title: "ログアウトしますか？", confirmLabel: "ログアウト" }))) return;
    if (!logoutLock.acquire()) return;
    leaving.current = true;
    setLogoutError("");
    try {
      // 通知解除が応答しない場合でもログアウトへ進む。
      await unsubscribePushOnThisDevice(token);
      const supabase = createClient();
      if (!supabase) throw new Error("認証を読み込めませんでした");
      const { error } = await supabase.auth.signOut({ scope: "local" });
      // SDK は通信エラーでも端末のセッションを削除する。残っている場合だけ再試行を案内する。
      if (error) {
        const { data } = await supabase.auth.getSession();
        if (data.session) throw error;
      }
      router.replace("/");
      router.refresh();
    } catch {
      leaving.current = false;
      setLogoutError("ログアウトできませんでした。もう一度お試しください。");
      logoutLock.release();
    }
  }

  useEffect(() => {
    if (!loading && !session && !leaving.current) router.replace("/login");
  }, [loading, session, router]);

  if (loading || !session) {
    return <p className="text-sm text-slate-200">読み込み中...</p>;
  }

  return (
    <div className="mx-auto max-w-2xl space-y-4">
      <h1 className="text-xl font-bold text-slate-100">マイページ</h1>
      <nav aria-label="アカウント" className="divide-y divide-white/15 overflow-hidden rounded-2xl border border-white/20 glass bg-white/10">
        {profile?.is_admin ? <MenuLink href="/admin" label="アプリ管理" icon={Shield} /> : null}
        {accountLinks.map((item) => (
          <MenuLink key={item.href} {...item} />
        ))}
        <MenuLink href="/me/install" label="ホーム画面に追加して使う" icon={Smartphone} />
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
      {logoutError ? <p role="alert" className="text-sm text-red-400">{logoutError}</p> : null}
    </div>
  );
}
