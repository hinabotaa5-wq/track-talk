"use client";
import { L } from "@/lib/i18n";

import Link from "next/link";
import { useEffect, useState } from "react";
import { useAuth } from "@/components/auth-provider";
import { PageLoading } from "@/components/page-loading";
import { CoachSnsEntry } from "@/components/coach-sns-entry";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { coachDisplayText } from "@/lib/coach-display";
import { coachEventLabels } from "@/lib/coach-recruit";
import { coachTicketPurchaseHref, type TicketSummary } from "@/lib/coaching-tickets";
import { supportPlanListEnabled } from "@/lib/features";
import { reviewPriceYen } from "@/lib/review-pricing";
import type { PublicCoach } from "@/lib/types";
import { ReferralBanner } from "../coaches/referral-banner";

// コーチタブの特集ページ（本体 #985）。特集モード（NEXT_PUBLIC_COACHING_FEATURED_COACH_ID）の間、/coaching の一覧の代わりに出す。
// 名前・写真・記録・種目・紹介文・実績・SNS はプロフィールデータから出し、コードに個人の固定文言を書かない。
// 固定の文はキャッチコピー・回答の見本・流れ・よくある質問だけ（すべて L() でキー化）。
// 外すもの：コーチカードの2列一覧・種目チップの絞り込み・1回プラン／継続プランの切り替え。

// 記録の欄は自由記入なので、区切り（改行・読点・スラッシュ・全角スラッシュ）でチップに分ける。
export function recordChips(text: string | null): string[] {
  return (text ?? "").split(/[\n、,，/／|｜]+/).map((part) => part.trim()).filter(Boolean);
}

type Loaded = { coach: PublicCoach | null } | { error: true };

export function FeaturedCoachView({ coachID }: { coachID: string }) {
  const [result, setResult] = useState<Loaded>();
  const { token } = useAuth();
  const [ticketSales, setTicketSales] = useState<{ token: string; canPurchase: boolean }>();

  useEffect(() => {
    let cancelled = false;
    apiFetch<PublicCoach>(`/v1/coaches/${encodeURIComponent(coachID)}`)
      .then((coach) => {
        if (!cancelled) setResult({ coach });
      })
      .catch((error: unknown) => {
        if (cancelled) return;
        setResult(error instanceof ApiRequestError && error.status === 404 ? { coach: null } : { error: true });
      });
    return () => { cancelled = true; };
  }, [coachID]);

  useEffect(() => {
    // チケットの新規販売を止めている間は「チケットを買う」を出さない（コーチ詳細と同じ判定）。
    if (!token) return;
    let cancelled = false;
    apiFetch<TicketSummary>("/v1/me/coaching/tickets", { token })
      .then((data) => {
        if (!cancelled) setTicketSales({ token, canPurchase: data.can_purchase });
      })
      .catch(() => {
        if (!cancelled) setTicketSales(undefined);
      });
    return () => { cancelled = true; };
  }, [token]);

  if (!result) return <PageLoading screen label={L("読み込み中...")} />;
  if ("error" in result) return <p role="alert" className="mx-auto max-w-md text-sm text-red-400">{L("読み込みに失敗しました")}</p>;
  const coach = result.coach;
  if (!coach) return <p className="mx-auto max-w-md text-sm text-slate-200">{L("このコーチは現在公開されていません。")}</p>;

  const name = coachDisplayText(coach.nickname, coach.nickname_en);
  const records = recordChips(coachDisplayText(coach.best_record, coach.best_record_en));
  const achievements = (coachDisplayText(coach.achievements, coach.achievements_en) ?? "").split("\n").map((line) => line.trim()).filter(Boolean);
  const hitokoto = coachDisplayText(coach.hitokoto, coach.hitokoto_en);
  const events = coach.events.map((event) => coachEventLabels[event]).join(L("・"));
  const open = coach.on_duty;
  const showTickets = !(ticketSales?.canPurchase === false && ticketSales.token === token);

  return (
    <div className="mx-auto max-w-md space-y-6 pb-32">
      <section aria-label={L("コーチの紹介")} className="overflow-hidden rounded-2xl border border-white/20 glass bg-white/5">
        {coach.photo_url ? (
          // eslint-disable-next-line @next/next/no-img-element
          <img src={coach.photo_url} alt={name ?? ""} className="aspect-[4/5] w-full object-cover" />
        ) : null}
        <div className="space-y-3 p-4">
          <p className={`w-fit rounded-full px-3 py-0.5 text-xs font-bold ${open ? "bg-blue-500/20 text-blue-300" : "bg-red-500/15 text-red-400"}`}>
            {open ? L("受付中") : L("受付停止中")}
          </p>
          <h1 className="text-2xl font-bold text-white">{name ? L("{v0}コーチ", { v0: name }) : L("コーチ")}</h1>
          {achievements.length ? <p className="text-sm text-slate-200">{achievements[0]}</p> : null}
          <p className="text-base font-semibold text-white">{L("スタートとフォームの悩みを、動画1本で。")}</p>
          <ul className="flex flex-wrap gap-1.5" aria-label={L("記録と対応種目")}>
            {records.map((record) => (
              <li key={record} className="rounded-full border border-amber-400/40 bg-amber-500/10 px-2.5 py-0.5 text-xs font-semibold text-amber-200">{record}</li>
            ))}
            {events ? <li className="rounded-full border border-blue-400/40 bg-blue-500/15 px-2.5 py-0.5 text-xs font-semibold text-blue-200">{L("対応種目 {v0}", { v0: events })}</li> : null}
          </ul>
        </div>
      </section>

      <ReferralBanner />

      <section aria-labelledby="featured-sample" className="space-y-3 text-sm">
        <h2 id="featured-sample" className="text-lg font-bold">{L("どんな添削が返ってくるか")}</h2>
        <ol className="grid grid-cols-3 gap-2 text-center text-xs">
          {[L("動画を1本送るだけ"), L("コーチが動画を見て回答"), L("24時間以内に返却")].map((step, i) => (
            <li key={i} className="rounded-xl border border-white/15 bg-white/5 p-2">
              <span className="block text-base font-bold text-blue-300">{i + 1}</span>
              {step}
            </li>
          ))}
        </ol>
        <article aria-label={L("回答の見本")} className="space-y-3 rounded-2xl border border-white/20 glass bg-white/5 p-4">
          <p className="text-xs text-slate-300">{L("回答の見本（実際の回答も同じ項目で届きます）")}</p>
          {[
            { title: L("良い点"), tone: "text-emerald-300", body: L("接地が体の真下に近く、ブレーキが小さい走りです。") },
            { title: L("直す点"), tone: "text-amber-300", body: L("後半に腕振りが横に流れ、上体が起き上がっています。") },
            { title: L("おすすめのドリル"), tone: "text-blue-300", body: L("壁押しドリル 10秒×3本で、前傾を保つ感覚をつかみましょう。") },
            { title: L("ひとこと"), tone: "text-violet-300", body: L("前半の良さを後半まで続けられれば、記録はまだ伸びます。") },
            { title: L("見本動画"), tone: "text-slate-300", body: L("必要なときは、コーチが動きの見本動画を添えます。") },
          ].map((item) => (
            <section key={item.title} className="space-y-1">
              <h3 className={`text-xs font-semibold ${item.tone}`}>{item.title}</h3>
              <p className="text-slate-100">{item.body}</p>
            </section>
          ))}
        </article>
      </section>

      <section aria-labelledby="featured-about" className="space-y-3 text-sm">
        <h2 id="featured-about" className="text-lg font-bold">{name ? L("{v0}コーチについて", { v0: name }) : L("コーチについて")}</h2>
        {hitokoto ? <p className="whitespace-pre-wrap break-words text-slate-100">{hitokoto}</p> : null}
        {achievements.length ? (
          <ul className="space-y-1.5" aria-label={L("成績や指導経験")}>
            {achievements.map((line, index) => <li key={index} className="border-l-2 border-blue-400/60 pl-2 break-words text-slate-100">{line}</li>)}
          </ul>
        ) : null}
        {(coach.sns_urls ?? []).length ? (
          <div className="flex flex-wrap gap-1.5">
            {(coach.sns_urls ?? []).map((entry) => <CoachSnsEntry key={entry} value={entry} />)}
          </div>
        ) : null}
      </section>

      <p className="text-xs text-slate-300">
        {supportPlanListEnabled ? <Link href={`/coaches/${encodeURIComponent(coach.id)}?tab=support`} className="underline">{L("継続プランを見る")}</Link> : L("継続プランは準備中です")}
      </p>

      <section aria-labelledby="featured-faq" className="space-y-2 text-sm">
        <h2 id="featured-faq" className="text-base font-bold">{L("よくある質問")}</h2>
        {[
          { q: L("返却までどのくらいかかりますか？"), a: L("申し込みが確定してから24時間以内に回答が届きます。") },
          { q: L("どんな動画を送ればいいですか？"), a: L("走っている姿が横から映った動画を1本送ってください。スマートフォンで撮った動画で大丈夫です。") },
          { q: L("受付が止まっているときはどうなりますか？"), a: L("受付停止中は依頼できません。受付が再開すると、このページの表示が「受付中」に変わります。") },
        ].map((item) => (
          <details key={item.q} className="rounded-xl border border-white/15 bg-white/5 p-3">
            <summary className="cursor-pointer font-semibold">{item.q}</summary>
            <p className="pt-2 text-slate-200">{item.a}</p>
          </details>
        ))}
      </section>

      <div className="glass fixed inset-x-0 bottom-[calc(3.5rem+env(safe-area-inset-bottom))] z-20 border-t border-white/20 bg-white/10 p-3 md:bottom-0 md:pb-[max(0.75rem,env(safe-area-inset-bottom))]">
        <div className="mx-auto flex max-w-md flex-col gap-2">
          {open ? null : <p className="text-center text-xs font-semibold text-red-400">{L("現在は受付を停止しています")}</p>}
          <div className="flex gap-2">
            {open ? (
              <Link href={`/coaching/new?coach=${encodeURIComponent(coach.id)}&pay=card`} className="flex min-h-11 flex-1 items-center justify-center rounded-md bg-blue-600 px-3 text-sm font-semibold text-pure-white transition active:scale-[0.98]">
                {L("動画添削を依頼する ¥{v0}", { v0: reviewPriceYen.toLocaleString("ja-JP") })}
              </Link>
            ) : (
              <span aria-disabled="true" className="flex min-h-11 flex-1 cursor-not-allowed items-center justify-center rounded-md bg-white/10 px-3 text-sm font-semibold text-slate-400">
                {L("動画添削を依頼する ¥{v0}", { v0: reviewPriceYen.toLocaleString("ja-JP") })}
              </span>
            )}
            {showTickets ? (
              <Link href={coachTicketPurchaseHref(coach.id)} className="flex min-h-11 items-center justify-center rounded-md border border-blue-400/60 px-3 text-sm font-semibold text-blue-300">
                {L("チケットを買う")}
              </Link>
            ) : null}
          </div>
        </div>
      </div>
    </div>
  );
}
