"use client";

import { CoachConsultationQueue } from "@/components/coaching-consultations";
import { CoachApplyLink } from "@/components/coach-apply-link";

import { useAuth } from "@/components/auth-provider";
import { CoachEarningsSection } from "@/components/coach-payouts";
import { CoachPhotoForm } from "@/components/coach-photo";
import { CoachContractsSection } from "@/components/coaching-contracts";
import { CoachPlansSection } from "@/components/coaching-plans";
import { Button } from "@/components/ui/button";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { coachApplyPath, coachEventLabels, coachTierLabels } from "@/lib/coach-recruit";
import { coachingEnabled } from "@/lib/features";
import { formatDateTime } from "@/lib/format";
import { reviewQuestionText } from "@/lib/review";
import type { CoachApplication, CoachingMe, CoachRatingSummary, CoachRequests, CoachReviewRequest } from "@/lib/types";
import { useActionLock } from "@/lib/use-action-lock";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useCallback, useEffect, useState } from "react";

// コーチ画面。承認されたコーチが「受付中」と1日の上限本数を決め、届いた打診を引き受けるか見送る。
// 返答と補足は、添削中の依頼から /coach/requests/{id} で書く。
// パーソナル指導の申し込みへの返事と、プランの作成も同じ画面でする。

const capOptions = [1, 2, 3, 5, 8, 10, 15, 20];

function minutesLeft(iso: string, now: number) {
  return Math.max(0, Math.ceil((new Date(iso).getTime() - now) / 60000));
}

function RequestSummary({ request }: { request: CoachReviewRequest }) {
  return (
    <div className="space-y-0.5">
      <p className="font-semibold text-white">
        {coachEventLabels[request.event]} · {coachTierLabels[request.tier]}
      </p>
      <p className="whitespace-pre-wrap break-words text-sm text-slate-200">相談内容: {reviewQuestionText(request)}</p>
      <p className="text-xs text-slate-300">返却の目安: {formatDateTime(request.due_at)}まで</p>
    </div>
  );
}

function CoachRequestsSection({ token }: { token: string | null }) {
  const [data, setData] = useState<CoachRequests | null>(null);
  const [error, setError] = useState("");
  // 読み込みの失敗は、引き受け・見送りの失敗と分ける（読み込めたら消える）。
  const [loadError, setLoadError] = useState("");
  const [now, setNow] = useState(() => Date.now());
  const [reload, setReload] = useState(0);
  const { pending, acquire, release } = useActionLock();

  const fetchRequests = useCallback(() => apiFetch<CoachRequests>("/v1/coach/requests", { token }), [token]);

  useEffect(() => {
    let cancelled = false;
    const load = () => {
      fetchRequests()
        .then((d) => {
          if (cancelled) return;
          setData(d);
          setNow(Date.now());
          setLoadError("");
        })
        .catch(() => {
          if (!cancelled) setLoadError("依頼を読み込めませんでした");
        });
    };
    load();
    // 打診の残り時間を出し直し、期限が過ぎた打診を消すため、30秒ごとに読み直す。
    const timer = window.setInterval(load, 30000);
    const refresh = () => {
      if (document.visibilityState === "visible") load();
    };
    document.addEventListener("visibilitychange", refresh);
    return () => {
      cancelled = true;
      window.clearInterval(timer);
      document.removeEventListener("visibilitychange", refresh);
    };
  }, [fetchRequests, reload]);

  async function answer(offerID: string, action: "accept" | "decline") {
    if (!acquire()) return;
    setError("");
    try {
      await apiFetch(`/v1/coach/offers/${offerID}/${action}`, { method: "POST", token });
    } catch (err) {
      setError(err instanceof ApiRequestError ? err.message : "送信に失敗しました");
    } finally {
      release();
      setReload((n) => n + 1);
    }
  }

  const shownError = error || loadError;
  if (!data) return shownError ? <p className="text-sm text-red-400">{shownError}</p> : null;

  return (
    <>
      <section className="space-y-2">
        <h2 className="font-bold">届いている依頼</h2>
        {data.offers.length === 0 ? <p className="text-sm text-slate-300">いま届いている依頼はありません</p> : null}
        <ul className="space-y-2">
          {data.offers.map((offer) => (
            <li key={offer.id} className="space-y-3 rounded-xl border border-blue-400/40 bg-blue-500/15 p-3">
              <RequestSummary request={offer.request} />
              <p className="text-sm font-semibold text-white">あと{minutesLeft(offer.expires_at, now)}分で次のコーチに回ります</p>
              <div className="flex gap-2">
                <Button type="button" className="flex-1" disabled={pending} onClick={() => void answer(offer.id, "accept")}>
                  引き受ける
                </Button>
                <button
                  type="button"
                  disabled={pending}
                  onClick={() => void answer(offer.id, "decline")}
                  className="min-h-11 flex-1 rounded-md border border-white/25 bg-white/10 px-4 text-sm font-semibold text-slate-100 transition active:scale-[0.98] disabled:opacity-50"
                >
                  見送る
                </button>
              </div>
            </li>
          ))}
        </ul>
        {shownError ? <p className="text-sm text-red-400">{shownError}</p> : null}
      </section>

      <section className="space-y-2">
        <h2 className="font-bold">添削中</h2>
        {data.reviewing.length === 0 ? <p className="text-sm text-slate-300">添削中の依頼はありません</p> : null}
        <ul className="space-y-3">
          {data.reviewing.map((request) => (
            <li key={request.id} className="space-y-3 rounded-xl border border-white/20 glass bg-white/10 p-3">
              <RequestSummary request={request} />
              {request.video_url ? (
                <video
                  src={request.video_url}
                  poster={request.thumbnail_url || undefined}
                  controls
                  playsInline
                  preload="metadata"
                  className="w-full rounded-lg bg-black"
                />
              ) : null}
              <Link
                href={`/coach/requests/${request.id}`}
                className="flex min-h-11 w-full items-center justify-center rounded-md bg-blue-600 px-4 text-sm font-semibold text-white"
              >
                {request.status === "clarifying" ? "補足を書く" : "返答を書く"}
              </Link>
            </li>
          ))}
        </ul>
      </section>
    </>
  );
}

export function CoachDashboard() {
  const { session, profile, token, loading } = useAuth();
  const router = useRouter();
  const [coach, setCoach] = useState<CoachApplication | null | undefined>(undefined);
  const [devices, setDevices] = useState(0);
  const [rating, setRating] = useState<CoachRatingSummary | null>(null);
  const [error, setError] = useState("");
  const { pending, acquire, release } = useActionLock();
  // Stripe で口座の登録が済んだと分かったら、受付を始められるようにする。
  const onPayoutsReady = useCallback(() => setCoach((c) => (c && !c.payouts_ready ? { ...c, payouts_ready: true } : c)), []);

  useEffect(() => {
    if (!loading && !session) {
      router.replace(`/login?next=${encodeURIComponent(window.location.pathname)}`);
    }
  }, [loading, session, router]);

  useEffect(() => {
    if (!token) return;
    let cancelled = false;
    Promise.all([
      apiFetch<{ application: CoachApplication | null; rating?: CoachRatingSummary }>("/v1/coach/me", { token }),
      coachingEnabled ? apiFetch<CoachingMe>("/v1/me/coaching", { token }) : Promise.resolve(null),
    ])
      .then(([c, me]) => {
        if (cancelled) return;
        setCoach(c.application);
        setRating(c.rating ?? null);
        setDevices(me?.push_devices ?? 0);
      })
      .catch(() => {
        if (!cancelled) setError("読み込みに失敗しました");
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
  if (loading || !session || coach === undefined) return <p className="text-sm text-slate-200">読み込み中...</p>;

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
    </div>
  );

  return (
    <div className="mx-auto max-w-md space-y-6">
      <div className="space-y-1">
        <h1 className="text-2xl font-bold">コーチ画面</h1>
        <p className="text-sm text-slate-300">
          {coachTierLabels[coach.tier]} · {coach.events.map((e) => coachEventLabels[e]).join("、")}
        </p>
      </div>

      <section className="space-y-3 rounded-2xl border border-white/20 glass bg-white/10 p-4">
        <CoachPhotoForm application={coach} nickname={coach.name || profile?.nickname || null} token={token} onSaved={setCoach} />
        <div className="flex items-center justify-between gap-3 border-t border-white/15 pt-3">
          <div>
            <p className="text-lg font-bold">{coach.on_duty ? "受付中" : "受付停止中"}</p>
            <p className="text-xs text-slate-300">受付中の間だけ、添削の依頼が通知で届きます。</p>
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
            className="rounded-md border border-white/25 bg-white/10 px-3 py-2 text-base text-slate-100"
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
        {rating && rating.count > 0 ? (
          <p className="text-sm text-slate-200">
            受けた評価: ★{rating.average.toFixed(1)}（{rating.count}件）
          </p>
        ) : null}
        {rating?.paused ? (
          <p className="rounded-xl border border-amber-400/40 bg-amber-500/15 p-3 text-sm text-white">
            評価の平均が★3を下回ったため、いまは依頼が回ってきません。
          </p>
        ) : null}
      </section>

      <CoachEarningsSection token={token} onReady={onPayoutsReady} />

      <CoachConsultationQueue token={token} />
      <CoachRequestsSection token={token} />

      <CoachContractsSection token={token} />

      <CoachPlansSection coach={coach} token={token} onCoachChange={setCoach} />

      <section className="space-y-2">
        <h2 className="font-bold">通知</h2>
        <p className="text-sm text-slate-300">
          依頼は通知で届き、30分以内に引き受けないと次のコーチに回ります。通知を受け取れる端末: {devices}台
        </p>
        <Link href="/me/coaching" className="text-sm text-blue-300 underline">
          通知の設定
        </Link>
      </section>
    </div>
  );
}
