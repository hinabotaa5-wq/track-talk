"use client";
import { L } from "@/lib/i18n";

import Link from "next/link";
import { useEffect, useState } from "react";
import { GraduationCap, PersonStanding } from "lucide-react";
import { useAuth } from "@/components/auth-provider";
import { apiFetch } from "@/lib/api";

// 一覧の右上に置く選手・コーチへの入口。運営者決定（2026-11-14／2026-11-17）で、ログイン状態・応募有無によらず常時出す。
// 文字ボタン2つでは切り替えピルが窮屈になるため、両方ともアイコンだけの円ボタン（44px）にする。
// 未ログインで押した場合は遷移先（/player・/coach）側のガードでログイン画面へ出る。応募状態などの取得はしない。
// 運営者決定（2026-11-17、本体 #891）：自分が対応しないと進まない用事があるときだけ、その件数を円の右上にバッジで出す。
// 通知ベルの未読数とは別系統で、件数は GET /v1/me/coaching/action-counts（ログイン中だけ取得）。取得に失敗したらバッジを出さないだけ。
const circleButtonClass =
  "relative inline-flex size-11 shrink-0 items-center justify-center rounded-full border border-white/25 bg-white/10 text-slate-100 transition hover:border-blue-400/50 hover:bg-blue-500/15 active:scale-[0.98]";

type ActionCounts = { player_count: number; coach_count: number };

// ベルのバッジと同じ規則：100 以上は「99+」。
function badgeText(count: number) {
  if (count > 99) return "99+";
  return String(count);
}

function validCount(value: unknown) {
  return typeof value === "number" && Number.isFinite(value) && value > 0 ? Math.floor(value) : 0;
}

function CountBadge({ count }: { count: number }) {
  if (count < 1) return null;
  return (
    <span
      aria-hidden
      className="pointer-events-none absolute -right-1 -top-1 flex min-w-5 items-center justify-center rounded-full bg-red-600 px-1 text-xs font-bold leading-5 text-pure-white"
    >
      {badgeText(count)}
    </span>
  );
}

const noCounts = { player: 0, coach: 0 };

export function CoachSelfButton() {
  const { session, token } = useAuth();
  // 本体 #1009：件数は「どの利用者の分か」と組で持ち、表示中の利用者と一致するときだけ出す。
  // キーは利用者 ID（session がなければ token）。同じ利用者の token 更新では消さずに取り直す。
  const userKey = token ? session?.user?.id ?? token : null;
  const [loaded, setCounts] = useState<{ key: string | null; player: number; coach: number }>({ key: null, ...noCounts });
  // ログアウト・利用者の切替直後は、前の件数を残さない。
  const counts = userKey && loaded.key === userKey ? loaded : noCounts;

  useEffect(() => {
    // 未ログインでは取得せず、バッジも出さない。
    if (!token || !userKey) return;
    let alive = true;
    const load = () => {
      apiFetch<ActionCounts>("/v1/me/coaching/action-counts", { token })
        .then((data) => {
          if (alive) setCounts({ key: userKey, player: validCount(data?.player_count), coach: validCount(data?.coach_count) });
        })
        // 失敗してもエラーは出さない。バッジを出さないだけで、遷移も妨げない。
        .catch(() => {
          if (alive) setCounts({ key: userKey, ...noCounts });
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

  const playerLabel = counts.player > 0 ? L("選手（未対応{count}件）", { count: counts.player }) : L("選手");
  const coachLabel = counts.coach > 0 ? L("コーチ（未対応{count}件）", { count: counts.coach }) : L("コーチ");
  return (
    <div className="flex items-center gap-2">
      <Link href="/player" aria-label={playerLabel} className={circleButtonClass}>
        <PersonStanding className="size-5 shrink-0 text-blue-300" aria-hidden />
        <CountBadge count={counts.player} />
      </Link>
      <Link href="/coach" aria-label={coachLabel} className={circleButtonClass}>
        <GraduationCap className="size-5 shrink-0 text-blue-300" aria-hidden />
        <CountBadge count={counts.coach} />
      </Link>
    </div>
  );
}
