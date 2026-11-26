"use client";
import { L } from "@/lib/i18n";

import Link from "next/link";
import { Suspense, useEffect, useId, useState, type ReactNode } from "react";
import { MotionConfig } from "motion/react";
import { usePathname, useSearchParams } from "next/navigation";
import { CoachAvatar } from "@/components/coach-photo";
import { PageLoading } from "@/components/page-loading";
import { CoachSnsEntry } from "@/components/coach-sns-entry";
import { ViewSwitch } from "@/components/view-switch";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { coachDisplayText } from "@/lib/coach-display";
import { coachEventLabels } from "@/lib/coach-recruit";
import { markLinkPending } from "@/lib/pending-link";
import type { PublicCoach } from "@/lib/types";
import { CoachSelfButton } from "../coaches/coach-self-button";
import { ReferralBanner } from "../coaches/referral-banner";

// コーチタブの特集ページ（本体 #985、表示の手直しは #991）。特集モード（NEXT_PUBLIC_COACHING_FEATURED_COACH_ID）の間、/coaching の一覧の代わりに出す。
// 名前・写真・記録・種目・紹介文・実績・SNS はプロフィールデータから出し、コードに個人の固定文言を書かない。
// 固定の文は流れ（3ステップ）・よくある質問だけ（すべて L() でキー化）。
// 並び（#991）：紹介バナー → 切り替え＋選手・コーチのボタン → コーチについて → 3ステップ → よくある質問（＋下部固定 CTA）。
// 切り替えは通常一覧（coach-tabs.tsx）と同じ「スタンダード」と「プレミアム」の2択（本体 #1026）。
// プレミアム選択中は特集コンテンツと固定 CTA を出さず、「プレミアムは準備中です」の一文だけを出す（価格・説明・申込みの導線は出さない）。
// 紹介バナーと切り替え＋選手・コーチのボタンの行はプレミアム選択中も出す。スタンダードに戻してもコーチ情報は取り直さない。
// 継続プランは休眠させたので（本体 #1007）、継続プランの選択肢は出さない。?tab=support で開かれてもスタンダード側を出し、
// 「継続プランは準備中です」は描画しない。
// 外すもの：コーチカードの2列一覧・種目チップの絞り込み。

// 記録の欄は自由記入なので、区切り（改行・読点・スラッシュ・全角スラッシュ）でチップに分ける。
export function recordChips(text: string | null): string[] {
  return (text ?? "").split(/[\n、,，/／|｜]+/).map((part) => part.trim()).filter(Boolean);
}

type Loaded = { coach: PublicCoach | null } | { error: true };
type Tab = "video" | "premium";

export function FeaturedCoachView({ coachID }: { coachID: string }) {
  return (
    <Suspense>
      <FeaturedCoachTabs coachID={coachID} />
    </Suspense>
  );
}

function FeaturedCoachTabs({ coachID }: { coachID: string }) {
  const pathname = usePathname();
  const params = useSearchParams();
  const base = pathname === "/coaches" ? "/coaches" : "/coaching";
  // 直接リンク・共有 URL でプレミアム側から開けるよう、初回だけ URL を見る。以後はその場の状態が正本。
  const [tab, setTab] = useState<Tab>(() => (params.get("tab") === "premium" ? "premium" : "video"));
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

  const views = [
    { id: "video", label: L("スタンダード") },
    { id: "premium", label: L("プレミアム") },
  ] as const;
  const hrefOf = (id: Tab) => (id === "premium" ? `${base}?tab=premium` : base);
  // その場の状態で即座に切り替え、URL は書き換えるだけにする（router.replace は読み込みが挟まるので使わない）。
  function select(next: Tab) {
    if (next === tab) return;
    setTab(next);
    window.history.replaceState(null, "", hrefOf(next));
  }

  return (
    <div className="mx-auto max-w-md space-y-6 pb-32">
      <ReferralBanner />
      <MotionConfig reducedMotion="user">
        <div className="flex items-center gap-2">
          <div className="min-w-0 flex-1">
            <ViewSwitch
              label={L("コーチの一覧を切り替える")}
              layoutId="coach-list-view-pill"
              initialPillStatic
              views={views}
              current={tab}
              onSelect={select}
              href={hrefOf}
            />
          </div>
          <CoachSelfButton />
        </div>
      </MotionConfig>
      {tab === "premium" ? (
        <p className="py-8 text-center text-sm text-slate-200">{L("プレミアムは準備中です")}</p>
      ) : (
        <FeaturedContent result={result} />
      )}
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
  // 固定 CTA の価格は、特集コーチのいま有効な価格（API の応答。本体 #1052）。古い API で無いときは価格を出さない。
  const ctaLabel = typeof coach.review_price_yen === "number"
    ? L("このコーチに依頼する ¥{v0}", { v0: coach.review_price_yen.toLocaleString("ja-JP") })
    : L("このコーチに依頼する");
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

      <ol data-testid="featured-steps" className="grid grid-cols-3 gap-2 text-center text-xs">
        {[L("動画を1本送るだけ"), L("コーチが動画を見て回答"), L("24時間以内に返却")].map((step, i) => (
          <li key={i} className="rounded-xl border border-white/15 bg-white/5 p-2">
            <span className="block text-base font-bold text-blue-300">{i + 1}</span>
            {step}
          </li>
        ))}
      </ol>

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

      <div data-testid="featured-cta" className="glass fixed inset-x-0 bottom-[calc(3.5rem+env(safe-area-inset-bottom))] z-20 border-t border-white/20 bg-white/10 p-3 md:bottom-0 md:pb-[max(0.75rem,env(safe-area-inset-bottom))]">
        <div className="mx-auto flex max-w-md flex-col gap-2">
          {open ? null : <p className="text-center text-xs font-semibold text-red-400">{L("現在は受付を停止しています")}</p>}
          <div className="flex gap-2">
            {open ? (
              <FeaturedCtaLink href={`/coaching/new?coach=${encodeURIComponent(coach.id)}&pay=card`}>
                {ctaLabel}
              </FeaturedCtaLink>
            ) : (
              <span aria-disabled="true" className="flex min-h-11 flex-1 cursor-not-allowed items-center justify-center rounded-md bg-white/10 px-3 text-sm font-semibold text-slate-400">
                {ctaLabel}
              </span>
            )}
          </div>
        </div>
      </div>
    </>
  );
}
