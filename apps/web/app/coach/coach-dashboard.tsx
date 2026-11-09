"use client";

import { CoachAvatar } from "@/components/coach-photo";
import { CoachApplyLink } from "@/components/coach-apply-link";
import { PageLoading } from "@/components/page-loading";
import { minutesLeft } from "@/components/coach-requests";

import { useAuth } from "@/components/auth-provider";
import { CoachPhotoForm } from "@/components/coach-photo";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { coachApplyPath } from "@/lib/coach-recruit";
import { coachingEnabled } from "@/lib/features";
import { formatYen } from "@/lib/format";
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
  CalendarDays,
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
import { useEffect, useState } from "react";

// コーチ画面。承認されたコーチの入口はタイルの1画面にし、中身はタイル先の個別画面で扱う。
// 返答と補足は /coach/requests/{id} で書く。パーソナル指導の申し込みへの返事と、プランの作成は個別画面でする。

const capOptions = [1, 2, 3, 5, 8, 10, 15, 20];

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
}: {
  href: string;
  label: string;
  Icon: typeof Inbox;
  badge?: number;
  dot?: boolean;
}) {
  const count = badge ?? 0;
  const ariaLabel = count > 0 ? `${label}（${count}件）` : dot ? `${label}（要確認）` : label;
  return (
    <Link
      href={href}
      aria-label={ariaLabel}
      className="relative flex flex-col items-center gap-1 rounded-xl border border-white/20 bg-white/10 px-1 py-3 text-center transition active:scale-[0.98]"
    >
      <Icon className="size-6" aria-hidden />
      <span className="text-[11px] font-semibold leading-tight">{label}</span>
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

export function CoachDashboard() {
  const { session, profile, token, loading } = useAuth();
  const router = useRouter();
  const [coach, setCoach] = useState<CoachApplication | null | undefined>(undefined);
  const [rating, setRating] = useState<CoachRatingSummary | null>(null);
  const [offers, setOffers] = useState<CoachRequests | null>(null);
  const [consultations, setConsultations] = useState(0);
  const [appliedContracts, setAppliedContracts] = useState(0);
  const [earnings, setEarnings] = useState<CoachEarnings | null>(null);
  const [identity, setIdentity] = useState<CoachIdentityStatus | null>(null);
  const [now, setNow] = useState(() => Date.now());
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
        if (!cancelled) setError("読み込みに失敗しました");
      });
    // バッジの件数は、既存の一覧取得の再利用で賄う。取れなくても入口は出す。
    apiFetch<CoachRequests>("/v1/coach/requests", { token })
      .then((d) => {
        if (cancelled) return;
        setOffers(d);
        setNow(Date.now());
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
  }, [token]);

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
      setError(err instanceof ApiRequestError ? err.fields?.daily_cap || err.message : "保存できませんでした");
    } finally {
      release();
    }
  }

  if (error && coach === undefined) return <p className="text-sm text-red-400">{error}</p>;
  if (loading || !session || coach === undefined) return <PageLoading screen label="読み込み中..." />;

  if (!coach || coach.status !== "approved") {
    return (
      <div className="mx-auto max-w-md space-y-4">
        <h1 className="text-2xl font-bold">コーチ画面</h1>
        <p className="text-sm leading-relaxed text-slate-200">コーチングは準備中です。競技や指導の経験を生かして、選手の次の一歩を支えるコーチを募集しています。</p>
        <CoachApplyLink />
        {coach ? <Link href={coachApplyPath} className="block text-sm text-blue-300 underline">応募状況・応募内容を確認する</Link> : null}
      </div>
    );
  }

  if (!coachingEnabled) return (
    <div className="mx-auto max-w-md space-y-5">
      <h1 className="text-2xl font-bold">コーチ画面</h1>
      <p className="text-sm leading-relaxed text-slate-200">コーチの応募は承認されています。現在は受付開始に向けて準備中です。機能の公開後、この画面から依頼への返答やプランの管理ができます。</p>
      <CoachPhotoForm application={coach} nickname={coach.name || profile?.nickname || null} token={token} onSaved={setCoach} />
      <Link href="/coach/about" className="block text-sm text-blue-300 underline">コーチ機能について</Link>
      <Link href="/coach/apply" className="block text-sm text-blue-300 underline">応募内容を確認する</Link>
      <CoachWithdrawLink />
    </div>
  );

  const offerCount = offers?.offers.length ?? 0;
  const shortest = offers && offers.offers.length > 0
    ? Math.min(...offers.offers.map((o) => minutesLeft(o.expires_at, now)))
    : 0;
  const reviewingCount = offers?.reviewing.length ?? 0;
  const commerceDot = identity !== null && identity.registered === false && identity.applies === true;
  const payoutsDot = !coach.payouts_ready;
  const [, payoutMonth, payoutDay] = earnings?.next_payout_date?.split("-") ?? [];

  return (
    <div className="mx-auto max-w-md space-y-3">
      <section aria-label="受付の状態" className="space-y-2 rounded-2xl border border-white/20 glass bg-white/10 p-3">
        <div className="flex items-center gap-3">
          <CoachAvatar url={coach.photo_url} name={coach.name || profile?.nickname || null} size={40} />
          <div className="min-w-0 flex-1">
            <p className="text-base font-bold leading-tight">{coach.on_duty ? "受付中" : "受付停止中"}</p>
            {rating && rating.count > 0 ? (
              <p className="text-xs text-slate-200">
                ★{rating.average.toFixed(1)}（{rating.count}件）
              </p>
            ) : null}
          </div>
          <button
            type="button"
            role="switch"
            aria-checked={coach.on_duty}
            aria-label="受付中"
            disabled={pending}
            onClick={() => void save({ on_duty: !coach.on_duty })}
            className={`relative h-8 w-14 shrink-0 rounded-full transition disabled:opacity-50 ${coach.on_duty ? "bg-blue-600" : "bg-white/25"}`}
          >
            <span
              className={`absolute top-1 size-6 rounded-full bg-white transition-all ${coach.on_duty ? "left-7" : "left-1"}`}
            />
          </button>
        </div>
        <label className="flex items-center justify-between gap-3 text-sm">
          <span>1日に受ける上限</span>
          <select
            className="rounded-md border border-white/25 bg-white/10 px-3 py-1.5 text-base text-slate-100"
            value={coach.daily_cap}
            disabled={pending}
            onChange={(e) => void save({ daily_cap: Number(e.target.value) })}
          >
            {capOptions.map((n) => (
              <option key={n} value={n}>
                {n}本
              </option>
            ))}
          </select>
        </label>
        {error ? <p className="text-sm text-red-400">{error}</p> : null}
        {rating?.paused ? (
          <p className="rounded-xl border border-amber-400/40 bg-amber-500/15 p-3 text-sm text-slate-200">
            評価の平均が★3を下回ったため、いまは依頼が回ってきません。
          </p>
        ) : null}
      </section>

      {offerCount > 0 ? (
        <Link
          href="/coach/offers"
          className="block rounded-xl border border-blue-400/40 bg-blue-500/15 p-3 text-sm font-semibold text-white"
        >
          新しい依頼が{offerCount}件あります — あと{shortest}分で終了
        </Link>
      ) : null}

      <nav aria-label="コーチのメニュー" className="grid grid-cols-4 gap-2">
        <Tile href="/coach/offers" label="届いた依頼" Icon={Inbox} badge={offerCount} />
        <Tile href="/coach/reviewing" label="添削中" Icon={PenLine} badge={reviewingCount} />
        <Tile href="/coach/consultations" label="追加相談" Icon={MessagesSquare} badge={consultations} />
        <Tile href="/coach/contracts" label="契約・申込み" Icon={FileText} badge={appliedContracts} />
        <Tile href="/coach/support-consults" label="無料相談" Icon={MessagesSquare} />
        <Tile href="/coach/plans" label="プラン管理" Icon={Layers} />
        <Tile href="/coach/availability" label="カレンダー" Icon={CalendarDays} />
        <Tile href="/coach/commerce" label="特商法・身元" Icon={Landmark} dot={commerceDot} />
        <Tile href="/coach/payouts" label="報酬・口座" Icon={Wallet} dot={payoutsDot} />
        <Tile href="/coach/profile" label="プロフィール" Icon={UserRound} />
        <Tile href="/me/push" label="通知設定" Icon={BellRing} />
      </nav>

      <footer className="space-y-1 border-t border-white/15 pt-3">
        {earnings ? (
          <p className="text-sm text-slate-200">
            未払い{formatYen(earnings.unpaid_yen)}
            {earnings.next_payout_date && earnings.next_payout_yen != null
              ? ` · 次回${Number(payoutMonth)}月${Number(payoutDay)}日 ${formatYen(earnings.next_payout_yen)}`
              : null}
          </p>
        ) : null}
        <CoachWithdrawLink bare />
      </footer>
    </div>
  );
}

// コーチだけをやめる入口。アカウントの退会（マイページ）とは別にする。
function CoachWithdrawLink({ bare = false }: { bare?: boolean }) {
  const link = (
    <Link href="/coach/withdraw" className="text-sm text-slate-300 underline">
      コーチをやめる
    </Link>
  );
  if (bare) {
    return (
      <div className="space-y-1">
        {link}
        <p className="text-xs text-slate-400">アカウントは残り、一般の利用者として使い続けられます。</p>
      </div>
    );
  }
  return (
    <section className="space-y-1 border-t border-white/15 pt-4">
      {link}
      <p className="text-xs text-slate-400">アカウントは残り、一般の利用者として使い続けられます。</p>
    </section>
  );
}
