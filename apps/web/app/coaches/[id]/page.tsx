"use client";
import { L } from "@/lib/i18n";

import { PageLoading } from "@/components/page-loading";
import { useAuth } from "@/components/auth-provider";

import { CoachAvatar } from "@/components/coach-photo";
import { CoachSnsEntry } from "@/components/coach-sns-entry";
import { CoachServicePlans } from "@/components/coach-service-plans";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { formatYen } from "@/lib/format";
import { getCoachDetailPreview } from "@/lib/coach-detail-preview";
import { coachingServiceLabels } from "@/lib/coaching-plan";
import { coachEventLabels } from "@/lib/coach-recruit";
import { coachTicketPurchaseHref, ticketKindCounts, type TicketKind, type TicketSummary } from "@/lib/coaching-tickets";
import { reviewPriceYen, ticketPackPriceYen, ticketPackQuantity } from "@/lib/review-pricing";
import type { PublicCoach } from "@/lib/types";
import { CreditCard, Gift, Ticket, Trophy, type LucideIcon } from "lucide-react";
import Link from "next/link";
import { useParams } from "next/navigation";
import { useEffect, useState, useSyncExternalStore } from "react";

type ReviewEntry = { key: "card" | TicketKind; Icon: LucideIcon; label: string; note?: string; href?: string };

export default function CoachDetailPage() {
  const { id } = useParams<{ id: string }>();
  return <CoachDetail key={id} id={id} />;
}

const subscribeToPreview = () => () => {};
const noServerPreview = () => undefined;

// 動画添削の受付状態。ログイン済みのときだけ nominees（受付中のコーチだけが返る正本）で確かめる。
// 未ログイン、または受付状態の取得に失敗した場合は有効なまま出す（最終確認は申込フロー側が担う）。
type Availability = "available" | "unavailable" | "unknown";

function CoachDetail({ id }: { id: string }) {
  const preview = useSyncExternalStore(subscribeToPreview, () => getCoachDetailPreview(id), noServerPreview);
  const [result, setResult] = useState<{ coach: PublicCoach | null } | { error: true }>();
  const { token } = useAuth();
  const [availability, setAvailability] = useState<Availability>("unknown");
  const [ticketSales, setTicketSales] = useState<{ token: string; canPurchase: boolean; counts: Record<TicketKind, number> | null }>();

  useEffect(() => {
    let cancelled = false;
    apiFetch<PublicCoach>(`/v1/coaches/${encodeURIComponent(id)}`)
      .then((coach) => {
        if (!cancelled) setResult({ coach });
      })
      .catch((error: unknown) => {
        if (!cancelled) {
          setResult(error instanceof ApiRequestError && error.status === 404 && error.code === "coach_not_public"
            ? { coach: null }
            : { error: true });
        }
      });
    return () => { cancelled = true; };
  }, [id]);

  const coach = result && "coach" in result ? result.coach : preview;
  const eventsKey = coach?.events.join(",");
  const coachID = coach && "id" in coach ? coach.id : undefined;

  useEffect(() => {
    if (!token || !coachID || !eventsKey) return;
    let cancelled = false;
    const events = eventsKey.split(",");
    Promise.all(events.map((event) =>
      apiFetch<{ coaches: { id: string }[] }>(`/v1/coaching/nominees?event=${encodeURIComponent(event)}`, { token })
        .then((r): { id: string }[] => r.coaches)
        // 取得に失敗した種目は判定に使わない。全種目で失敗したら有効なままにする。
        .catch((): { id: string }[] | null => null),
    )).then((results) => {
      if (cancelled) return;
      if (results.every((list) => list === null)) return;
      setAvailability(results.some((list) => (list ?? []).some((c) => c.id === coachID)) ? "available" : "unavailable");
    });
    return () => { cancelled = true; };
  }, [token, coachID, eventsKey]);

  useEffect(() => {
    if (!token || !coachID) return;
    let cancelled = false;
    apiFetch<TicketSummary>("/v1/me/coaching/tickets", { token })
      .then((data) => {
        if (!cancelled) setTicketSales({ token, canPurchase: data.can_purchase, counts: ticketKindCounts(data) });
      })
      .catch(() => {
        if (!cancelled) setTicketSales(undefined);
      });
    return () => { cancelled = true; };
  }, [token, coachID]);

  const achievementLines = coach?.achievements ? coach.achievements.split("\n").map((line) => line.trim()).filter(Boolean) : [];
  const ctaHref = `/coaching/new?coach=${encodeURIComponent(id)}`;
  const ctaEnabled = availability !== "unavailable";
  // チケット購入画面へのリンク。購入を完了したらこのコーチ詳細に戻る（本体 #691。2つの入口で同じ戻り先）。
  const ticketPurchaseHref = coachTicketPurchaseHref(coachID);
  const showTickets = !(ticketSales?.canPurchase === false && ticketSales.token === token);
  // 種別ごとの残り。未ログイン・取得失敗・古い API では枚数を出さない（#684 と同じく、取れなくても入口は出す）。
  const counts = ticketSales && ticketSales.token === token ? ticketSales.counts : null;
  // 動画添削の入口（本体 #687）：単発・通常チケット・無料チケット。支払い方式を選んでから申込画面へ進む。
  const entries: ReviewEntry[] = [
    { key: "card", Icon: CreditCard, label: L("単発"), note: formatYen(reviewPriceYen), href: `${ctaHref}&pay=card` },
    {
      key: "normal", Icon: Ticket, label: L("通常チケット"),
      note: counts ? L("残り {n} 枚", { n: counts.normal }) : undefined,
      // 通常チケットが0枚なら購入画面へ（#684 の「購入する」と同じ遷移先・同じ戻り先）。
      href: counts?.normal === 0 ? ticketPurchaseHref : `${ctaHref}&pay=normal`,
    },
    {
      key: "reward", Icon: Gift, label: L("無料チケット"),
      note: counts ? L("残り {n} 枚", { n: counts.reward }) : undefined,
      // 無料チケットが0枚なら押せない。
      href: counts?.reward === 0 ? undefined : `${ctaHref}&pay=reward`,
    },
  ];

  return (
    <div className="mx-auto max-w-md space-y-6">
      {result && "error" in result ? <p role="alert" className="text-sm text-red-400">{L("コーチの詳細を読み込めませんでした")}</p> : coach === undefined ? <PageLoading screen label={L("読み込み中...")} /> : coach === null ? <p className="text-sm text-slate-200">{L("このコーチは現在公開されていません。")}</p> : <>
        <section className="glass space-y-4 rounded-2xl border border-white/20 bg-white/10 p-4 text-sm" aria-label={L("コーチのプロフィール")}>
          <div className="flex items-start gap-4">
            <CoachAvatar url={coach.photo_url} name={coach.nickname} size={112} />
            <div className="min-w-0 flex-1 space-y-1.5 break-words">
              <h1 className="text-2xl font-bold text-white">{coach.nickname ?? L("名前なし")}</h1>
              {coach.events.length ? <ul className="flex flex-wrap gap-1.5" aria-label={L("担当種目")}>
                {coach.events.map((event) => <li key={event} className="rounded-full border border-blue-400/40 bg-blue-500/15 px-2.5 py-0.5 text-xs font-semibold text-blue-200">{coachEventLabels[event]}</li>)}
              </ul> : null}
            </div>
          </div>
          <dl className="space-y-4">
            {coach.best_record ? <div>
              <dt className="text-xs text-slate-300">{L("記録")}</dt>
              <dd className="mt-1 flex items-center gap-2 text-lg font-bold text-white"><Trophy className="size-5 shrink-0 text-amber-400" aria-hidden /><span className="break-words">{coach.best_record}</span></dd>
            </div> : null}
            {achievementLines.length ? <div>
              <dt className="text-xs text-slate-300">{L("成績や指導経験")}</dt>
              <dd className="mt-1">
                <ul className="space-y-1.5">
                  {achievementLines.map((line, index) => <li key={index} className="border-l-2 border-blue-400/60 pl-2 break-words text-slate-100">{line}</li>)}
                </ul>
              </dd>
            </div> : null}
            {(coach.service_types ?? []).length ? <div>
              <dt className="text-xs text-slate-300">{L("担当サービス")}</dt>
              <dd className="whitespace-pre-wrap break-words text-slate-100">{(coach.service_types ?? []).map((service) => coachingServiceLabels[service]).join(L("・"))}</dd>
            </div> : null}
            {coach.hitokoto ? <div>
              <dt className="text-xs text-slate-300">{L("ひとこと")}</dt>
              <dd className="whitespace-pre-wrap break-words text-slate-100">{coach.hitokoto}</dd>
            </div> : null}
            {(coach.sns_urls ?? []).length ? <div>
              <dt className="text-xs text-slate-300">{L("SNS・ホームページ")}</dt>
              <dd className="mt-1 flex flex-wrap gap-1.5">
                {(coach.sns_urls ?? []).map((entry) => <CoachSnsEntry key={entry} value={entry} />)}
              </dd>
            </div> : null}
          </dl>
        </section>
        <section className="space-y-3 text-sm" aria-labelledby="coach-video-review">
          <h2 id="coach-video-review" className="text-lg font-bold">{L("動画添削")}</h2>
          <p className="text-slate-200">{L("1回 {v0}（税込）・チケットも使えます", { v0: formatYen(reviewPriceYen) })}</p>
          <ul className="grid grid-cols-3 gap-2" aria-label={L("お支払い方法を選んで依頼する")}>
            {entries.map(({ key, Icon, label, note, href }) => {
              const body = <>
                <Icon className="size-5" aria-hidden />
                <span className="text-sm font-semibold">{label}</span>
                {note ? <span className="text-xs">{note}</span> : null}
              </>;
              const base = "flex min-h-20 w-full flex-col items-center justify-center gap-1 rounded-md px-2 py-2 text-center";
              return <li key={key} className="min-w-0" data-entry={key}>
                {ctaEnabled && href ? <Link
                  href={href}
                  className={`${base} bg-blue-600 text-pure-white transition active:scale-[0.98]`}
                >{body}</Link> : <span
                  aria-disabled="true"
                  className={`${base} cursor-not-allowed bg-white/10 text-slate-400`}
                >{body}</span>}
              </li>;
            })}
          </ul>
          {ctaEnabled ? null : <p className="text-xs text-slate-300">{L("現在は受付していません")}</p>}
        </section>
        {showTickets ? <section className="space-y-3 text-sm" aria-labelledby="coach-tickets">
          <h2 id="coach-tickets" className="text-lg font-bold">{L("チケット")}</h2>
          <p className="text-slate-200">{L("チケット{q}枚で{v0}です。添削1回につき1枚消費します。", { q: ticketPackQuantity, v0: formatYen(ticketPackPriceYen) })}</p>
          <Link
            href={ticketPurchaseHref}
            className="flex min-h-11 w-full items-center justify-center rounded-md border border-white/25 bg-white/10 px-4 text-sm font-semibold text-slate-100 transition hover:bg-white/16 active:scale-[0.98]"
          >{L("購入する")}</Link>
        </section> : null}
        <section className="space-y-3 text-sm" aria-labelledby="coach-plans">
          <h2 id="coach-plans" className="text-lg font-bold">{L("サポートプラン")}</h2>
          <CoachServicePlans key={coach.id} plans={coach.plans ?? []} coachID={coach.id} consult={coach.support_consult} />
        </section>
      </>}
    </div>
  );
}
