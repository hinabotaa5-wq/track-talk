"use client";
import { L } from "@/lib/i18n";

import { PageLoading } from "@/components/page-loading";
import { useAuth } from "@/components/auth-provider";
import { apiFetch } from "@/lib/api";
import { BellRing, ClipboardCheck, Ticket, type LucideIcon } from "lucide-react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";

// 選手用ページ。コーチ画面（/coach）に合わせ、タイルを並べたダッシュボード形式にする。
// 運営者決定（2026-11-24、本体 #1003）：コーチ画面と同様に、行き先のタイルごとに件数バッジを付ける。
// 件数は円ボタンと同じ GET /v1/me/coaching/action-counts のタイル別の4フィールド（player_count をタイルごとに分けたもの）で、
// ログイン中だけ、マウント時と画面の再表示時（visibilitychange で visible／focus／bfcache 復帰の pageshow）に取り直す。取得を待たずにタイルは出し、
// 失敗したらバッジを出さないだけ。通知タイル（通知設定）にはバッジを付けない。ドット（オレンジ点）は選手側には付けない。
// タイル内部の見た目（アイコン・ラベル・角丸・間隔）はコーチ画面と同じ `.coach-tiles` の CSS 規則に乗せて揃える。
// コーチ画面の縦の余り配分（CoachTiles の ResizeObserver）は持ち込まない（#940）。
// 継続プランは休眠させたので（本体 #1007）、「継続プラン」「無料相談」のタイルは出さない。件数の取得と CountKey はそのまま残す。
type CountKey = "player_one_time_count" | "player_personal_count" | "player_support_count" | "player_ticket_count";
type TileCounts = Partial<Record<CountKey, number>>;

const tiles: { href: string; label: string; Icon: LucideIcon; countKey?: CountKey }[] = [
  { href: "/me/coaching/requests", label: "スタンダード", Icon: ClipboardCheck, countKey: "player_one_time_count" },
  { href: "/me/coaching/tickets", label: "チケット", Icon: Ticket, countKey: "player_ticket_count" },
  { href: "/me/push", label: "通知", Icon: BellRing },
];

function validCount(value: unknown) {
  return typeof value === "number" && Number.isFinite(value) && value > 0 ? Math.floor(value) : 0;
}

// 見た目はコーチ画面（/coach）の Tile と同一。件数はそのままの数字（99+ への丸めはしない）。
function Tile({ href, label, Icon, badge }: { href: string; label: string; Icon: LucideIcon; badge: number }) {
  const ariaLabel = badge > 0 ? L("{label}（{count}件）", { label, count: badge }) : undefined;
  return (
    <Link
      href={href}
      aria-label={ariaLabel}
      className="relative flex aspect-square flex-col items-center justify-center gap-1 rounded-xl border border-white/20 bg-white/10 px-1 py-3 text-center transition active:scale-[0.98]"
    >
      <Icon className="size-6" aria-hidden />
      <span className="coach-tile-label whitespace-nowrap text-[11px] font-semibold leading-tight">{label}</span>
      {badge > 0 ? (
        <span
          aria-hidden
          className="absolute -right-1 -top-1 flex h-5 min-w-5 items-center justify-center rounded-full bg-red-500 px-1 text-[11px] font-bold text-white"
        >
          {badge}
        </span>
      ) : null}
    </Link>
  );
}

export function PlayerDashboard() {
  const { session, loading, token } = useAuth();
  const router = useRouter();
  // 本体 #1009：件数は「どの利用者の分か」と組で持ち、表示中の利用者と一致するときだけ出す。
  // キーは利用者 ID（session がなければ token）。同じ利用者の token 更新では消さずに取り直す。
  const userKey = token ? session?.user?.id ?? token : null;
  const [loaded, setCounts] = useState<{ key: string | null; counts: TileCounts }>({ key: null, counts: {} });
  // ログアウト・利用者の切替直後は、前の件数を残さない。
  const counts = userKey && loaded.key === userKey ? loaded.counts : {};

  useEffect(() => {
    // 未ログインでは取得しない。取り直しのタイミングは円ボタン（coach-self-button）と同じ。
    if (!token || !userKey) return;
    let alive = true;
    const load = () => {
      apiFetch<TileCounts>("/v1/me/coaching/action-counts", { token })
        .then((data) => {
          if (!alive) return;
          setCounts({
            key: userKey,
            counts: {
              player_one_time_count: validCount(data?.player_one_time_count),
              player_personal_count: validCount(data?.player_personal_count),
              player_support_count: validCount(data?.player_support_count),
              player_ticket_count: validCount(data?.player_ticket_count),
            },
          });
        })
        // 失敗してもエラーは出さない。バッジを出さないだけで、タイルの表示・遷移は妨げない。
        .catch(() => {
          if (alive) setCounts({ key: userKey, counts: {} });
        });
    };
    const refresh = () => {
      if (document.visibilityState === "visible") load();
    };
    // 履歴キャッシュ（bfcache）から戻ったときも取り直す（本体 #1009）。
    const restored = (event: PageTransitionEvent) => {
      if (event.persisted) load();
    };
    load();
    document.addEventListener("visibilitychange", refresh);
    window.addEventListener("focus", refresh);
    window.addEventListener("pageshow", restored);
    return () => {
      alive = false;
      document.removeEventListener("visibilitychange", refresh);
      window.removeEventListener("focus", refresh);
      window.removeEventListener("pageshow", restored);
    };
  }, [token, userKey]);

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
          {tiles.map(({ href, label, Icon, countKey }) => (
            <Tile key={href} href={href} label={L(label)} Icon={Icon} badge={countKey ? counts[countKey] ?? 0 : 0} />
          ))}
        </nav>
      </div>
    </div>
  );
}
