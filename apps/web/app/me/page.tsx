"use client";
import { LoadedReferralCard } from "@/components/coaching-referral";

import { PageLoading } from "@/components/page-loading";

import { useAuth } from "@/components/auth-provider";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useRef, useState } from "react";
import { AtSign, Award, Bell, Bookmark, ChevronRight, ClipboardCheck, FileText, HeartPulse, KeyRound, LayoutGrid, LogOut, Mail, MessagesSquare, ScrollText, Shield, ShieldCheck, Smartphone, Ticket, UserRoundPen, Users, UserX, type LucideIcon } from "lucide-react";
import { coachingEnabled } from "@/lib/features";
import { createClient } from "@/lib/supabase";
import { useActionLock } from "@/lib/use-action-lock";
import { signOutOnThisDevice } from "@/lib/sign-out";
import { useConfirm } from "@/components/confirm-dialog";
import { t, L } from "@/lib/i18n";
import { useLocale } from "@/components/locale-provider";
import { PushPermissionGuide } from "@/components/push-permission-guide";
import { ThemeSwitch } from "@/components/theme-switch";

const accountLinks = [
  { href: "/me/posts", get label() { return L("自分の投稿"); }, icon: LayoutGrid },
  { href: "/me/saved", get label() { return L("保存した投稿"); }, icon: Bookmark },
  { href: "/me/injury", get label() { return L("自分のケガ履歴"); }, icon: HeartPulse },
  { href: "/me/profile", get label() { return L("ニックネームと種目の変更"); }, icon: UserRoundPen },
  { href: "/me/email", get label() { return L("メールアドレスの変更"); }, icon: AtSign },
  { href: "/me/password", get label() { return L("パスワードの変更"); }, icon: KeyRound },
  { href: "/me/security", get label() { return L("二段階認証"); }, icon: ShieldCheck },
  { href: "/me/badge", get label() { return L("バッジ"); }, icon: Award },
] as const;

// 旧コーチタブの「利用中の方」と同じ順・同じ遷移先。コーチングが閉じている間は行き止めの入口を出さない。
const coachingLinks = [
  { href: "/me/coaching/tickets", get label() { return L("コーチングチケット"); }, icon: Ticket },
  { href: "/me/coaching/requests", get label() { return L("依頼した動画添削"); }, icon: ClipboardCheck },
  { href: "/coaching/support-consults", get label() { return L("1ヶ月プランの無料相談"); }, icon: MessagesSquare },
  { href: "/me/coaching/personal", get label() { return L("申し込んだサポートプラン"); }, icon: Users },
] as const;

const infoLinks = [
  { href: "/guide", get label() { return L("利用ガイド"); }, icon: FileText },
  { href: "/me/contact", get label() { return L("お問い合わせ"); }, icon: Mail },
  { href: "/guidelines", get label() { return L("ガイドライン"); }, icon: ScrollText },
  { href: "/terms", get label() { return L("利用規約"); }, icon: FileText },
  { href: "/privacy", get label() { return L("プライバシーポリシー"); }, icon: Shield },
] as const;

const menuRowClass =
  "flex min-h-12 w-full items-center gap-3 px-4 text-left text-sm font-medium text-slate-100 transition-colors hover:bg-white/12 active:bg-slate-500/15 disabled:opacity-50";

// マイページ内は運営者決定（2026-11-14）で全項目ブルーに統一する。機能割当色は他画面の仕様で、本ページには適用しない。サイズ・角丸・濃さは .tile で同一。
function MenuLink({ href, label, icon: Icon }: { href: string; label: string; icon: LucideIcon }) {
  return (
    <Link href={href} className={menuRowClass}>
      <span className="tile tile-blue" aria-hidden>
        <Icon className="size-4" />
      </span>
      <span className="flex-1">{label}</span>
      <ChevronRight className="size-4 shrink-0 text-slate-400" aria-hidden />
    </Link>
  );
}

export default function MePage() {
  const { session, profile, profileLoading, token, sessionLoading: loading, ensureProfile } = useAuth();
  const router = useRouter();
  const userId = session?.user?.id;
  const currentProfile = profile?.id === userId ? profile : null;
  const logoutLock = useActionLock();
  const confirmDialog = useConfirm();
  const [logoutError, setLogoutError] = useState("");
  // ログアウト中は、未ログインのリダイレクトでホームへの移動を上書きしない。
  const leaving = useRef(false);

  async function logout() {
    if (!userId) return;
    if (!(await confirmDialog({ title: L("ログアウトしますか？"), confirmLabel: L("ログアウト") }))) return;
    if (!logoutLock.acquire()) return;
    leaving.current = true;
    setLogoutError("");
    try {
      const supabase = createClient();
      if (!supabase) throw new Error(L("認証を読み込めませんでした"));
      await signOutOnThisDevice(userId, async () => {
        const { error } = await supabase.auth.signOut({ scope: "local" });
        // SDK は通信エラーでも端末のセッションを削除する。残っている場合だけ再試行を案内する。
        if (error && (await supabase.auth.getSession()).data.session) throw error;
      });
      router.replace("/");
      router.refresh();
    } catch {
      leaving.current = false;
      setLogoutError(L("ログアウトできませんでした。もう一度お試しください。"));
      logoutLock.release();
    }
  }

  // マイページを開いたとき、プロフィールが未取得のまま止まっていれば取り直す(#504)。
  useEffect(() => {
    if (userId) ensureProfile();
  }, [userId, ensureProfile]);

  useEffect(() => {
    if (!loading && !session && !leaving.current) router.replace("/login");
  }, [loading, session, router]);

  const locale = useLocale();
  if (loading || !session) {
    return <PageLoading screen label={t(locale, "common.loading")} />;
  }

  return (
    <div className="mx-auto max-w-2xl space-y-4">
      <h1 className="text-xl font-bold text-slate-100">{t(locale, "mypage.title")}</h1>
      <ThemeSwitch />
      <LoadedReferralCard token={token} />
      <nav aria-label={L("アカウント")} className="divide-y divide-slate-500/20 app-card overflow-hidden">
        {currentProfile?.is_admin ? (
          <MenuLink href="/admin" label={L("アプリ管理")} icon={Shield} />
        ) : !currentProfile && profileLoading ? (
          <div role="status" aria-busy="true" className="flex min-h-12 items-center gap-3 px-4">
            <span className="size-4 rounded bg-white/15 motion-safe:animate-pulse" aria-hidden="true" />
            <span className="h-3 w-24 rounded bg-white/15 motion-safe:animate-pulse" aria-hidden="true" />
            <span className="sr-only">{L("プロフィールを確認中")}</span>
          </div>
        ) : null}
        {accountLinks.map((item) => (
          <MenuLink key={item.href} {...item} />
        ))}
        {token && userId ? (
          <PushPermissionGuide
            key={userId}
            userId={userId}
            renderControl={(status, activate, busy) => {
              if (status === "granted" || status === "unsupported" || status === null) {
                return <MenuLink href="/me/push" label={L("通知")} icon={Bell} />;
              }
              return (
                <button type="button" className={menuRowClass} disabled={busy} onClick={activate}>
                  <span className="tile tile-blue" aria-hidden>
                    <Bell className="size-4" />
                  </span>
                  <span className="flex-1">
                    <span className="block">{L("通知設定")}</span>
                    <span className="text-xs text-slate-300">
                      {status === "denied" ? L("通知がオフになっています") : L("通知を受け取る")}
                    </span>
                  </span>
                  <ChevronRight className="size-4 shrink-0 text-slate-400" aria-hidden />
                </button>
              );
            }}
          />
        ) : <MenuLink href="/me/push" label={L("通知")} icon={Bell} />}
        <MenuLink href="/me/install" label={L("ホーム画面に追加して使う")} icon={Smartphone} />
        {infoLinks.map((item) => (
          <MenuLink key={item.href} {...item} />
        ))}
        <Link href="/me/delete" className={menuRowClass}>
          <span className="tile tile-blue" aria-hidden>
            <UserX className="size-4" />
          </span>
          <span className="flex-1">{L("退会する")}</span>
          <ChevronRight className="size-4 shrink-0 text-slate-400" aria-hidden />
        </Link>
        <button type="button" onClick={logout} disabled={logoutLock.pending} className={menuRowClass}>
          <span className="tile tile-blue" aria-hidden>
            <LogOut className="size-4" />
          </span>
          <span className="flex-1">{L("ログアウト")}</span>
        </button>
      </nav>
      {coachingEnabled ? (
        <section className="space-y-2">
          <h2 className="text-base font-bold text-slate-100">{L("コーチング")}</h2>
          <nav aria-label={L("コーチング")} className="divide-y divide-slate-500/20 app-card overflow-hidden">
            {coachingLinks.map((item) => (
              <MenuLink key={item.href} {...item} />
            ))}
          </nav>
        </section>
      ) : null}
      {logoutError ? <p role="alert" className="text-sm text-red-400">{logoutError}</p> : null}
    </div>
  );
}
