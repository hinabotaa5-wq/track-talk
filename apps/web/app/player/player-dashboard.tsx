"use client";
import { L } from "@/lib/i18n";

import { PageLoading } from "@/components/page-loading";
import { useAuth } from "@/components/auth-provider";
import { BellRing, ClipboardCheck, MessagesSquare, Ticket, Users, type LucideIcon } from "lucide-react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect } from "react";

// 選手用ページ。コーチ画面（/coach）に合わせ、タイルを並べたダッシュボード形式にする。
// タイルは静的リンクのみで、件数などのデータ取得は行わない（バッジ・ドットも付けない）。
// タイル内部の見た目（アイコン・ラベル・角丸・間隔）はコーチ画面と同じ `.coach-tiles` の CSS 規則に乗せて揃える。
// コーチ画面の縦の余り配分（CoachTiles の ResizeObserver）は持ち込まない（#940）。
const tiles: { href: string; label: string; Icon: LucideIcon }[] = [
  { href: "/me/coaching/requests", label: "1回プラン", Icon: ClipboardCheck },
  { href: "/me/coaching/personal", label: "継続プラン", Icon: Users },
  { href: "/coaching/support-consults", label: "無料相談", Icon: MessagesSquare },
  { href: "/me/coaching/tickets", label: "チケット", Icon: Ticket },
  { href: "/me/push", label: "通知", Icon: BellRing },
];

function Tile({ href, label, Icon }: { href: string; label: string; Icon: LucideIcon }) {
  return (
    <Link
      href={href}
      className="relative flex aspect-square flex-col items-center justify-center gap-1 rounded-xl border border-white/20 bg-white/10 px-1 py-3 text-center transition active:scale-[0.98]"
    >
      <Icon className="size-6" aria-hidden />
      <span className="coach-tile-label whitespace-nowrap text-[11px] font-semibold leading-tight">{label}</span>
    </Link>
  );
}

export function PlayerDashboard() {
  const { session, loading } = useAuth();
  const router = useRouter();

  useEffect(() => {
    if (!loading && !session) {
      router.replace("/login?next=/player");
    }
  }, [loading, session, router]);

  if (loading || !session) return <PageLoading screen label={L("読み込み中...")} />;

  return (
    <div className="mx-auto w-full max-w-md space-y-4">
      <h1 className="text-2xl font-bold">{L("選手")}</h1>
      <div className="coach-tiles">
        <nav aria-label={L("選手")} className="grid gap-2">
          {tiles.map(({ href, label, Icon }) => (
            <Tile key={href} href={href} label={L(label)} Icon={Icon} />
          ))}
        </nav>
      </div>
    </div>
  );
}
