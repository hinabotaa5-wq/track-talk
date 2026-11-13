"use client";

import { CoachAvatar } from "@/components/coach-photo";
import { CoachApplyLink } from "@/components/coach-apply-link";
import { PageLoading } from "@/components/page-loading";

import { useAuth } from "@/components/auth-provider";
import { CoachPhotoForm } from "@/components/coach-photo";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { coachApplyPath } from "@/lib/coach-recruit";
import { coachingEnabled } from "@/lib/features";
import { formatYen } from "@/lib/format";
import { t, type Locale, L } from "@/lib/i18n";
import { useLocale } from "@/components/locale-provider";
import type {
  CoachApplication,
  CoachEarnings,
  CoachIdentityStatus,
  CoachingContract,
  CoachRatingSummary,
  CoachRequests,
} from "@/lib/types";
import { useActionLock } from "@/lib/use-action-lock";
import {
  BellRing,
  FileText,
  Inbox,
  Landmark,
  Layers,
  MessagesSquare,
  PenLine,
  UserRound,
  Wallet,
} from "lucide-react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useCallback, useEffect, useState, type CSSProperties, type ReactNode } from "react";

// コーチ画面。承認されたコーチの入口はタイルの1画面にし、中身はタイル先の個別画面で扱う。
// 返答と補足は /coach/requests/{id} で書く。パーソナル指導の申し込みへの返事と、プランの作成は個別画面でする。


type ConsultationRow = {
  id: string;
  status: string;
};

// 身元登録の状態は、既存バッジの取得とは別枠で取る。取れなければ点を出さない。
async function fetchIdentityStatus(token: string | null): Promise<CoachIdentityStatus | null> {
  try {
    return await apiFetch<CoachIdentityStatus>("/v1/coach/me/identity-status", { token });
  } catch {
    return null;
  }
}

function Tile({
  href,
  label,
  Icon,
  badge,
  dot,
  locale,
}: {
  href: string;
  label: string;
  Icon: typeof Inbox;
  badge?: number;
  dot?: boolean;
  locale: Locale;
}) {
  const count = badge ?? 0;
  const ariaLabel = count > 0 ? t(locale, "coach.tile.count", { label, count }) : dot ? t(locale, "coach.tile.attention", { label }) : label;
  return (
    <Link
      href={href}
      aria-label={ariaLabel}
      className="relative flex aspect-square flex-col items-center justify-center gap-1 rounded-xl border border-white/20 bg-white/10 px-1 py-3 text-center transition active:scale-[0.98]"
    >
      <Icon className="size-6" aria-hidden />
      <span className="coach-tile-label whitespace-nowrap text-[11px] font-semibold leading-tight">{label}</span>
      {count > 0 ? (
        <span
          aria-hidden
          className="absolute -right-1 -top-1 flex h-5 min-w-5 items-center justify-center rounded-full bg-red-500 px-1 text-[11px] font-bold text-white"
        >
          {count}
        </span>
      ) : null}
      {count === 0 && dot ? (
        <span aria-hidden className="absolute -right-1 -top-1 size-3 rounded-full bg-amber-400" />
      ) : null}
    </Link>
  );
}

// タイルの枠（本体 #580）。タイルは横幅駆動（repeat(3, 1fr)＋正方形）で、大きさは CSS が決める。
// JS は縦の余りだけ配分する：行ギャップを最大 +8px → 上下パディングを最大 28px → 残りは中央寄せ
// （align-content: center、globals.css の .coach-tiles）。タイルの大きさは期限アラート帯の有無で変えない
// （横幅駆動なので高さの影響を受けない）。高さ不足の画面では基準値のままにしてページ全体でスクロールする
// （タイルを縮めない）。basePadBlock は CSS の --coach-pad-block の既定とそろえる。
const basePadBlock = 12;
const maxPadBlock = 28;
const maxGapAdd = 8;
function CoachTiles({ label, children }: { label: string; children: ReactNode }) {
  const [vertical, setVertical] = useState<{ rowGap: number; padBlock: number } | null>(null);
  const ref = useCallback((el: HTMLDivElement | null) => {
    if (!el || typeof ResizeObserver === "undefined") return;
    const compute = () => {
      const nav = el.firstElementChild;
      const tile = nav?.firstElementChild;
      if (!nav || !tile) return;
      const rowGapBase = Math.min(16, Math.max(12, window.innerWidth * 0.032));
      const tileSide = tile.getBoundingClientRect().width;
      if (!(tileSide > 0)) return;
      const rows = Math.ceil(nav.children.length / 3);
      const natural = tileSide * rows + rowGapBase * (rows - 1) + basePadBlock * 2;
      const leftover = el.clientHeight - natural;
      if (!(leftover > 0)) {
        setVertical((v) => (v === null ? v : null));
        return;
      }
      const gapAdd = rows > 1 ? Math.min(maxGapAdd, leftover / (rows - 1)) : 0;
      const padAdd = Math.min(maxPadBlock - basePadBlock, (leftover - gapAdd * (rows - 1)) / 2);
      const next = { rowGap: rowGapBase + gapAdd, padBlock: basePadBlock + padAdd };
      setVertical((v) =>
        v !== null && Math.abs(v.rowGap - next.rowGap) < 0.05 && Math.abs(v.padBlock - next.padBlock) < 0.05 ? v : next,
      );
    };
    const observer = new ResizeObserver(compute);
    observer.observe(el);
    return () => observer.disconnect();
  }, []);
  return (
    <div ref={ref} className="coach-tiles">
      <nav
        aria-label={label}
        className="grid gap-2"
        style={
          vertical
            ? ({ "--coach-row-gap": `${vertical.rowGap}px`, "--coach-pad-block": `${vertical.padBlock}px` } as CSSProperties)
            : undefined
        }
      >
        {children}
      </nav>
    </div>
  );
}

export function CoachDashboard() {
  const { session, profile, token, loading } = useAuth();
  const locale = useLocale();
  const router = useRouter();
  const [coach, setCoach] = useState<CoachApplication | null | undefined>(undefined);
  const [rating, setRating] = useState<CoachRatingSummary | null>(null);
  const [offers, setOffers] = useState<CoachRequests | null>(null);
  const [consultations, setConsultations] = useState(0);
  const [appliedContracts, setAppliedContracts] = useState(0);
  const [earnings, setEarnings] = useState<CoachEarnings | null>(null);
  const [identity, setIdentity] = useState<CoachIdentityStatus | null>(null);
  const [error, setError] = useState("");
  const { pending, acquire, release } = useActionLock();

  useEffect(() => {
    if (!loading && !session) {
      router.replace(`/login?next=${encodeURIComponent(window.location.pathname)}`);
    }
  }, [loading, session, router]);

  useEffect(() => {
    if (!token) return;
    let cancelled = false;
    apiFetch<{ application: CoachApplication | null; rating?: CoachRatingSummary }>("/v1/coach/me", { token })
      .then((c) => {
        if (cancelled) return;
        setCoach(c.application);
        setRating(c.rating ?? null);
      })
      .catch(() => {
        if (!cancelled) setError(t(locale, "coach.loadError"));
      });
    // バッジの件数は、既存の一覧取得の再利用で賄う。取れなくても入口は出す。
    apiFetch<CoachRequests>("/v1/coach/requests", { token })
      .then((d) => {
        if (cancelled) return;
        setOffers(d);
      })
      .catch(() => {});
    apiFetch<{ consultations: ConsultationRow[] }>("/v1/coach/consultations", { token })
      .then((d) => {
        if (!cancelled) setConsultations(d.consultations.length);
      })
      .catch(() => {});
    apiFetch<{ contracts: CoachingContract[] }>("/v1/coach/contracts", { token })
      .then((d) => {
        if (!cancelled) setAppliedContracts(d.contracts.filter((c) => c.status === "applied").length);
      })
      .catch(() => {});
    apiFetch<CoachEarnings>("/v1/coach/me/earnings", { token })
      .then((d) => {
        if (!cancelled) setEarnings(d);
      })
      .catch(() => {});
    void fetchIdentityStatus(token).then((d) => {
      if (!cancelled) setIdentity(d);
    });
    return () => {
      cancelled = true;
    };
  }, [token, locale]);

  async function save(next: { on_duty?: boolean; daily_cap?: number }) {
    if (!acquire()) return;
    setError("");
    try {
      const d = await apiFetch<{ application: CoachApplication }>("/v1/coach/me", {
        method: "PATCH",
        token,
        body: JSON.stringify(next),
      });
      setCoach(d.application);
    } catch (err) {
      setError(err instanceof ApiRequestError ? err.fields?.daily_cap || err.message : t(locale, "coach.saveError"));
    } finally {
      release();
    }
  }

  if (error && coach === undefined) return <p className="text-sm text-red-400">{error}</p>;
  if (loading || !session || coach === undefined) return <PageLoading screen label={t(locale, "common.loading")} />;

  if (!coach || coach.status !== "approved") {
    return (
      <div className="mx-auto max-w-md space-y-4">
        <h1 className="text-2xl font-bold">{t(locale, "coach.title")}</h1>
        <p className="text-sm leading-relaxed text-slate-200">{t(locale, "coach.notReady")}</p>
        <CoachApplyLink />
        {coach ? <Link href={coachApplyPath} className="block text-sm text-blue-300 underline">{t(locale, "coach.checkApplication")}</Link> : null}
      </div>
    );
  }

  if (!coachingEnabled) return (
    <div className="mx-auto max-w-md space-y-5">
      <h1 className="text-2xl font-bold">{t(locale, "coach.title")}</h1>
      <p className="text-sm leading-relaxed text-slate-200">{t(locale, "coach.preparing")}</p>
      <CoachPhotoForm application={coach} nickname={coach.name || profile?.nickname || null} token={token} onSaved={setCoach} />
      <Link href="/coach/about" className="block text-sm text-blue-300 underline">{t(locale, "coach.about")}</Link>
      <Link href="/coach/apply" className="block text-sm text-blue-300 underline">{t(locale, "coach.viewApplication")}</Link>
      <CoachWithdrawLink locale={locale} />
    </div>
  );

  const reviewingCount = offers?.reviewing.length ?? 0;
  const commerceDot = identity !== null && identity.registered === false && identity.applies === true;
  const payoutsDot = !coach.payouts_ready;
  const [, payoutMonth, payoutDay] = earnings?.next_payout_date?.split("-") ?? [];

  // スマホでは、ランチャーをヘッダーと下部ナビの間の高さいっぱいに広げる（本体 #580）。
  // main を縦の flex にし（app/layout.tsx の coach-launcher）、タイルの枠が残りの高さを占める。
  // タイルは3列の正方形で横幅いっぱいに広げ、縦の余りは行ギャップ→上下パディング→中央寄せの順で吸収する
  // （CoachTiles と globals.css の .coach-tiles）。収まらない低い画面ではタイルを縮めず、ページ全体が縦にスクロールする。
  return (
    <div className="coach-launcher mx-auto flex w-full max-w-md flex-col gap-3 max-md:flex-1">
      <section aria-label={t(locale, "coach.status")} className="space-y-2 rounded-2xl border border-white/20 glass bg-white/10 p-3">
        <div className="flex items-center gap-3">
          <CoachAvatar url={coach.photo_url} name={coach.name || profile?.nickname || null} size={40} />
          <div className="min-w-0 flex-1">
            <p className="text-base font-bold leading-tight">{coach.on_duty ? t(locale, "coach.onDuty") : t(locale, "coach.offDuty")}</p>
            {rating && rating.count > 0 ? (
              <p className="text-xs text-slate-200">{L("★{v0}（{count}件）", { v0: rating.average.toFixed(1), count: rating.count })}</p>
            ) : null}
          </div>
          <button
            type="button"
            role="switch"
            aria-checked={coach.on_duty}
            aria-label={t(locale, "coach.onDuty")}
            disabled={pending}
            onClick={() => void save({ on_duty: !coach.on_duty })}
            className={`relative h-8 w-14 shrink-0 rounded-full transition disabled:opacity-50 ${coach.on_duty ? "bg-blue-600" : "bg-white/25"}`}
          >
            <span
              className={`absolute top-1 size-6 rounded-full bg-white transition-all ${coach.on_duty ? "left-7" : "left-1"}`}
            />
          </button>
        </div>
        {error ? <p className="text-sm text-red-400">{error}</p> : null}
        {rating?.paused ? (
          <p className="rounded-xl border border-amber-400/40 bg-amber-500/15 p-3 text-sm text-slate-200">
            {t(locale, "coach.pausedNote")}
          </p>
        ) : null}
      </section>

      <CoachTiles label={t(locale, "coach.menu")}>
        <Tile locale={locale} href="/coach/reviewing" label={t(locale, "coach.tile.reviewing")} Icon={PenLine} badge={reviewingCount} />
        <Tile locale={locale} href="/coach/consultations" label={t(locale, "coach.tile.consultations")} Icon={MessagesSquare} badge={consultations} />
        <Tile locale={locale} href="/coach/contracts" label={t(locale, "coach.tile.contracts")} Icon={FileText} badge={appliedContracts} />
        <Tile locale={locale} href="/coach/support-consults" label={t(locale, "coach.tile.supportConsults")} Icon={MessagesSquare} />
        <Tile locale={locale} href="/coach/plans" label={t(locale, "coach.tile.plans")} Icon={Layers} />
        <Tile locale={locale} href="/coach/commerce" label={t(locale, "coach.tile.commerce")} Icon={Landmark} dot={commerceDot} />
        <Tile locale={locale} href="/coach/payouts" label={t(locale, "coach.tile.payouts")} Icon={Wallet} dot={payoutsDot} />
        <Tile locale={locale} href="/coach/profile" label={t(locale, "coach.tile.profile")} Icon={UserRound} />
        <Tile locale={locale} href="/me/push" label={t(locale, "coach.tile.push")} Icon={BellRing} />
      </CoachTiles>

      <footer className="space-y-1 border-t border-white/15 pt-3">
        {earnings ? (
          <p className="text-sm text-slate-200">
            {t(locale, "coach.unpaid")}{formatYen(earnings.unpaid_yen)}
            {earnings.next_payout_date && earnings.next_payout_yen != null
              ? ` · ${t(locale, "coach.nextPayout", { month: Number(payoutMonth), day: Number(payoutDay) })} ${formatYen(earnings.next_payout_yen)}`
              : null}
          </p>
        ) : null}
        <CoachWithdrawLink bare locale={locale} />
      </footer>
    </div>
  );
}

// コーチだけをやめる入口。アカウントの退会（マイページ）とは別にする。
function CoachWithdrawLink({ bare = false, locale }: { bare?: boolean; locale: Locale }) {
  const link = (
    <Link href="/coach/withdraw" className="text-sm text-slate-300 underline">
      {t(locale, "coach.withdraw")}
    </Link>
  );
  if (bare) {
    return (
      <div className="space-y-1">
        {link}
        <p className="text-xs text-slate-400">{t(locale, "coach.withdrawNote")}</p>
      </div>
    );
  }
  return (
    <section className="space-y-1 border-t border-white/15 pt-4">
      {link}
      <p className="text-xs text-slate-400">{t(locale, "coach.withdrawNote")}</p>
    </section>
  );
}
