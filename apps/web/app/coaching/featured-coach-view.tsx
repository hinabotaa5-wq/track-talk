"use client";
import { L } from "@/lib/i18n";

import Link from "next/link";
import { useEffect, useId, useState, type ReactNode } from "react";
import { CoachAvatar } from "@/components/coach-photo";
import { PageLoading } from "@/components/page-loading";
import { CoachSnsEntry } from "@/components/coach-sns-entry";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { coachDisplayText } from "@/lib/coach-display";
import { coachEventLabels } from "@/lib/coach-recruit";
import { markLinkPending } from "@/lib/pending-link";
import { premiumRequestHref } from "@/lib/premium";
import type { PublicCoach } from "@/lib/types";
import { CoachSelfButton } from "../coaches/coach-self-button";
import { ReferralBanner } from "../coaches/referral-banner";

// コーチタブの特集ページ（本体 #985、表示の手直しは #991）。特集モード（NEXT_PUBLIC_COACHING_FEATURED_COACH_ID）の間、/coaching の一覧の代わりに出す。
// 名前・写真・記録・種目・紹介文・実績・SNS はプロフィールデータから出し、コードに個人の固定文言を書かない。
// 固定の文は見出し文・流れ（スタンダード／プレミアムの各3ステップ）・固定 CTA だけ（すべて L() でキー化）。
// 並び（本体 #1054）：紹介バナー → 見出し文＋選手・コーチのボタン → コーチについて → スタンダードの流れ → プレミアムの流れ（＋下部固定 CTA）。
// 上部のスタンダード／プレミアムの切り替えは無くした（本体 #1054）。?tab=premium・?tab=support で開いても同じ内容を出す。
// コーチ一覧（coach-tabs.tsx）の切り替えは別物で、そちらはそのまま。よくある質問も #1054 で削除した（辞書のキーは残す）。
// 外すもの：コーチカードの2列一覧・種目チップの絞り込み。

// 記録の欄は自由記入なので、区切り（改行・読点・スラッシュ・全角スラッシュ）でチップに分ける。
export function recordChips(text: string | null): string[] {
  return (text ?? "").split(/[\n、,，/／|｜]+/).map((part) => part.trim()).filter(Boolean);
}

type Loaded = { coach: PublicCoach | null } | { error: true };

export function FeaturedCoachView({ coachID }: { coachID: string }) {
  const [result, setResult] = useState<Loaded>();

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

  // 初回読み込み中は、通常一覧と同じくストップウォッチだけにする。
  if (!result) return <PageLoading screen label={L("読み込み中...")} />;

  return (
    <div className="mx-auto max-w-md space-y-6 pb-32">
      <ReferralBanner />
      <div className="flex items-center gap-2">
        <p data-testid="featured-lead" className="min-w-0 flex-1 text-base font-bold">{L("フォームや練習内容を診断してもらおう！")}</p>
        <CoachSelfButton />
      </div>
      <FeaturedContent result={result} />
    </div>
  );
}

// チケットの販売は終了した（本体 #1031）ので、「チケットを買う」は出さない。
// 受付中の固定 CTA（本体 #1041）。タップした瞬間から下部タブと同じ「画面を開いています」の幕（ScreenLoader）を出す。
// React 起動前のタップは data-pending-link／data-screen-loader を起動スクリプトと CSS が拾う。
function FeaturedCtaLink({ href, children }: { href: string; children: ReactNode }) {
  const id = useId();
  return (
    <Link href={href} data-pending-link="" data-screen-loader="" onNavigate={() => markLinkPending(id, href)} className="flex min-h-11 flex-1 items-center justify-center rounded-md bg-[#EF442D] px-3 text-sm font-semibold text-pure-white transition active:scale-[0.98]">
      {children}
    </Link>
  );
}

function FeaturedContent({ result }: { result: Loaded }) {
  if ("error" in result) return <p role="alert" className="text-sm text-red-400">{L("読み込みに失敗しました")}</p>;
  const coach = result.coach;
  if (!coach) return <p className="text-sm text-slate-200">{L("このコーチは現在公開されていません。")}</p>;

  const name = coachDisplayText(coach.nickname, coach.nickname_en);
  const records = recordChips(coachDisplayText(coach.best_record, coach.best_record_en));
  const achievements = (coachDisplayText(coach.achievements, coach.achievements_en) ?? "").split("\n").map((line) => line.trim()).filter(Boolean);
  const hitokoto = coachDisplayText(coach.hitokoto, coach.hitokoto_en);
  // 固定 CTA の価格は、特集コーチのいま有効な価格（API の応答。本体 #1052／#1054）。古い API でスタンダード価格が無いときは価格を出さない。
  const standardLabel = typeof coach.review_price_yen === "number"
    ? L("スタンダード ¥{v0}", { v0: coach.review_price_yen.toLocaleString("ja-JP") })
    : L("スタンダード");
  // プレミアム価格がないコーチ（null・古い API で無い）はプレミアムボタンを出さず、スタンダードだけを全幅にする（本体 #1054）。
  const premiumLabel = typeof coach.premium_price_yen === "number"
    ? L("プレミアム ¥{v0}", { v0: coach.premium_price_yen.toLocaleString("ja-JP") })
    : null;
  const ctas = [
    { label: standardLabel, href: `/coaching/new?coach=${encodeURIComponent(coach.id)}&pay=card` },
    ...(premiumLabel ? [{ label: premiumLabel, href: premiumRequestHref(coach.id) }] : []),
  ];
  const events = coach.events.map((event) => coachEventLabels[event]).join(L("・"));
  const open = coach.on_duty;

  return (
    <>
      <section aria-labelledby="featured-about" className="space-y-3 text-sm">
        <div className="flex flex-col items-center gap-3">
          <CoachAvatar url={coach.photo_url} name={name} size={112} />
          <ul className="flex flex-wrap justify-center gap-1.5" aria-label={L("記録と対応種目")}>
            {records.map((record) => (
              <li key={record} className="rounded-full border border-amber-400/40 bg-amber-500/10 px-2.5 py-0.5 text-xs font-semibold text-amber-200">{record}</li>
            ))}
            {events ? <li className="rounded-full border border-blue-400/40 bg-blue-500/15 px-2.5 py-0.5 text-xs font-semibold text-blue-200">{L("対応種目 {v0}", { v0: events })}</li> : null}
          </ul>
        </div>
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

      <FeaturedSteps
        testID="featured-steps"
        heading={L("スタンダード")}
        steps={[L("動画を1本送るだけ"), L("コーチが動画を見て回答"), L("24時間以内に返却")]}
      />
      <FeaturedSteps
        testID="featured-premium-steps"
        heading={L("プレミアム")}
        steps={[L("枠ごとに動画を送るだけ（最大10本まで）"), L("コーチがまとめて見て診断"), L("24時間以内に総括で返却")]}
      />

      <div data-testid="featured-cta" className="glass fixed inset-x-0 bottom-[calc(3.5rem+env(safe-area-inset-bottom))] z-20 border-t border-white/20 bg-white/10 p-3 md:bottom-0 md:pb-[max(0.75rem,env(safe-area-inset-bottom))]">
        <div className="mx-auto flex max-w-md flex-col gap-2">
          {open ? null : <p className="text-center text-xs font-semibold text-red-400">{L("現在は受付を停止しています")}</p>}
          <div className="flex gap-2">
            {ctas.map((item) => open ? (
              <FeaturedCtaLink key={item.href} href={item.href}>
                {item.label}
              </FeaturedCtaLink>
            ) : (
              <span key={item.href} aria-disabled="true" className="flex min-h-11 flex-1 cursor-not-allowed items-center justify-center rounded-md bg-white/10 px-3 text-sm font-semibold text-slate-400">
                {item.label}
              </span>
            ))}
          </div>
        </div>
      </div>
    </>
  );
}

// 流れの3ステップ（本体 #1054）。スタンダードとプレミアムで同じ3列の見た目にし、どちらの流れか小見出しで示す。
function FeaturedSteps({ testID, heading, steps }: { testID: string; heading: string; steps: string[] }) {
  const id = useId();
  return (
    <section aria-labelledby={id} className="space-y-2">
      <h2 id={id} className="text-sm font-bold text-slate-100">{heading}</h2>
      <ol data-testid={testID} className="grid grid-cols-3 gap-2 text-center text-xs">
        {steps.map((step, i) => (
          <li key={i} className="rounded-xl border border-white/15 bg-white/5 p-2">
            <span className="block text-base font-bold text-blue-300">{i + 1}</span>
            {step}
          </li>
        ))}
      </ol>
    </section>
  );
}
