import { L } from "@/lib/i18n";
import { coachText, coachTexts, originalTextClass } from "@/lib/coaching-display";
import { clarifyReasonLabels } from "@/lib/review";
import type { ReviewResponse } from "@/lib/types";
import { formatDateTime } from "@/lib/format";
import type { ReactNode } from "react";

// 回答ドキュメントに出すコーチ（顔写真と名前）。コーチ本人の画面では渡さない。
export type ReviewResponseCoach = { nickname: string | null; photo_url: string } | null | undefined;

// コーチの返答（良い点・直す点・ドリル・ひとこと・見本動画・確認質問と補足）。利用者とコーチの両方の画面で使う。
// 1回プランは会話ではなく完結した回答なので、項目ごとの吹き出しに分けず1枚の回答ドキュメントとして出す（本体 #984）。
// 海外コーチの回答3点は、和訳があれば和訳を主に・原文を直下に併記する（本体 #916）。コーチ本人の画面（viewer="coach"）は原文だけ。
export function ReviewResponseView({ response, viewer = "requester", coach }: { response: ReviewResponse; viewer?: "requester" | "coach"; coach?: ReviewResponseCoach }) {
  if (response.hidden) {
    return (
      <section className="rounded-xl border border-white/20 glass bg-white/10 p-3 text-sm text-slate-200">{L("この返答は運営が非表示にしました。")}</section>
    );
  }
  const mine = viewer === "coach";
  const good = coachText(response.good_point, response.good_point_ja, { mine });
  const fixes = coachTexts(response.fix_points, response.fix_points_ja, { mine });
  const drill = coachText(response.drill, response.drill_ja, { mine });
  const name = mine ? L("あなたの回答") : coach?.nickname ? L("{v0}コーチの回答", { v0: coach.nickname }) : L("コーチの回答");
  return (
    <article aria-label={L("コーチの回答")} className="space-y-4 rounded-2xl border border-white/20 glass bg-white/5 p-4">
      <header className="flex items-center gap-3 border-b border-white/15 pb-3">
        {!mine && coach?.photo_url ? (
          // eslint-disable-next-line @next/next/no-img-element
          <img src={coach.photo_url} alt="" className="h-11 w-11 shrink-0 rounded-full object-cover" />
        ) : null}
        <div className="min-w-0">
          <h2 className="text-base font-bold">{name}</h2>
          <p className="text-xs text-slate-300">
            {L("回答日")} <time dateTime={response.created_at}>{formatDateTime(response.created_at)}</time>
          </p>
        </div>
      </header>
      <DocSection title={L("良い点")} tone="text-emerald-300">
        <p className="whitespace-pre-wrap text-slate-100">{good.text}</p>
        {good.original !== null ? <p lang="en" className={originalTextClass}>{good.original}</p> : null}
      </DocSection>
      {fixes.map((fix, i) => (
        <DocSection key={i} title={L("直す点{v0}", { v0: fixes.length > 1 ? ` ${i + 1}` : "" })} tone="text-amber-300">
          <p className="whitespace-pre-wrap text-slate-100">{fix.text}</p>
          {fix.original !== null ? <p lang="en" className={originalTextClass}>{fix.original}</p> : null}
        </DocSection>
      ))}
      <DocSection title={L("おすすめのドリル")} tone="text-blue-300">
        <p className="whitespace-pre-wrap text-slate-100">{drill.text}</p>
        {drill.original !== null ? <p lang="en" className={originalTextClass}>{drill.original}</p> : null}
      </DocSection>
      {response.hitokoto ? (
        <DocSection title={L("ひとこと")} tone="text-violet-300">
          <p className="whitespace-pre-wrap text-slate-100">{response.hitokoto}</p>
        </DocSection>
      ) : null}
      {response.demo_video_url ? (
        <DocSection title={L("見本動画")} tone="text-slate-300">
          <video src={response.demo_video_url} controls playsInline preload="metadata" className="w-full max-w-[260px] max-h-[300px] rounded-lg bg-black" />
        </DocSection>
      ) : null}
      {response.clarify_reason ? (
        <DocSection title={L("確認質問（{v0}）", { v0: clarifyReasonLabels[response.clarify_reason] })} tone="text-slate-300">
          {response.clarify_body ? <p className="whitespace-pre-wrap text-slate-100">{response.clarify_body}</p> : null}
          {response.clarification ? (
            <>
              <h4 className="pt-1 text-xs font-semibold text-slate-300">{L("補足（「{v0}」への答え）", { v0: clarifyReasonLabels[response.clarify_reason] })}</h4>
              <p className="whitespace-pre-wrap text-slate-100">{response.clarification}</p>
            </>
          ) : null}
        </DocSection>
      ) : null}
    </article>
  );
}

function DocSection({ title, tone, children }: { title: string; tone: string; children: ReactNode }) {
  return (
    <section className="space-y-1 text-sm break-words">
      <h3 className={`text-xs font-semibold ${tone}`}>{title}</h3>
      {children}
    </section>
  );
}
