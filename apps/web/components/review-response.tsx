import { L } from "@/lib/i18n";
import Link from "next/link";
import { CoachAvatar } from "@/components/coach-photo";
import { coachEventLabels, coachTierLabels } from "@/lib/coach-recruit";
import { coachingCoachListEnabled } from "@/lib/features";
import { clarifyReasonLabels } from "@/lib/review";
import type { ReviewCoach, ReviewResponse } from "@/lib/types";

// 返答と一緒に届く、見てくれたコーチのプロフィール。
export function ReviewCoachCard({ coach }: { coach: ReviewCoach }) {
  return (
    <section className="space-y-1 rounded-xl border border-white/20 glass bg-white/10 p-3 text-sm">
      <div className="flex items-center gap-3">
        <CoachAvatar url={coach.photo_url} name={coach.nickname} />
        <p className="font-bold text-white">
          {coach.nickname ?? L("退会したユーザー")}
          <span className="ml-2 text-xs font-normal text-slate-300">{coachTierLabels[coach.tier]}</span>
        </p>
      </div>
      <p className="text-slate-100">{coach.events.map((e) => coachEventLabels[e]).join(L("、"))}</p>
      <p className="whitespace-pre-wrap text-slate-100">{coach.best_record}</p>
      <p className="whitespace-pre-wrap text-slate-300">{coach.achievements}</p>
      {/* コーチの詳細はコーチ一覧と一緒に公開する。一覧を閉じている間は準備中の画面へ案内しない。 */}
      {coach.id && coachingCoachListEnabled ? <Link href={`/coaches/${encodeURIComponent(coach.id)}`} className="block text-sm text-blue-300 underline">{L("このコーチのプロフィールを見る")}</Link> : null}
    </section>
  );
}

// コーチの返答（良い点・直す点・ドリル・見本動画・補足）。利用者とコーチの両方の画面で使う。
export function ReviewResponseView({ response }: { response: ReviewResponse }) {
  if (response.hidden) {
    return (
      <section className="rounded-xl border border-white/20 glass bg-white/10 p-3 text-sm text-slate-200">{L("この返答は運営が非表示にしました。")}</section>
    );
  }
  return (
    <section className="space-y-3 rounded-xl border border-white/20 glass bg-white/10 p-3 text-sm">
      <div className="space-y-1">
        <h2 className="text-xs font-semibold text-emerald-300">{L("良い点")}</h2>
        <p className="whitespace-pre-wrap text-slate-100">{response.good_point}</p>
      </div>
      {response.fix_points.map((fix, i) => (
        <div key={i} className="space-y-1">
          <h2 className="text-xs font-semibold text-amber-300">{L("直す点{v0}", { v0: response.fix_points.length > 1 ? ` ${i + 1}` : "" })}</h2>
          <p className="whitespace-pre-wrap text-slate-100">{fix}</p>
        </div>
      ))}
      <div className="space-y-1">
        <h2 className="text-xs font-semibold text-blue-300">{L("おすすめのドリル")}</h2>
        <p className="whitespace-pre-wrap text-slate-100">{response.drill}</p>
      </div>
      {response.demo_video_url ? (
        <div className="space-y-1">
          <h2 className="text-xs font-semibold text-slate-300">{L("見本動画")}</h2>
          <video src={response.demo_video_url} controls playsInline preload="metadata" className="w-full rounded-lg bg-black" />
        </div>
      ) : null}
      {response.clarify_reason ? (
        <div className="space-y-1 border-t border-white/15 pt-3">
          {response.clarify_body ? <p className="whitespace-pre-wrap text-sm">{L("確認質問：{clarify_body}", { clarify_body: response.clarify_body })}</p> : null}
          {response.clarification ? (
            <>
              <h2 className="text-xs font-semibold text-slate-300">{L("補足（「{v0}」への答え）", { v0: clarifyReasonLabels[response.clarify_reason] })}</h2>
              <p className="whitespace-pre-wrap text-slate-100">{response.clarification}</p>
            </>
          ) : (
            <p className="text-xs text-slate-300">{L("確認質問（{v0}）", { v0: clarifyReasonLabels[response.clarify_reason] })}</p>
          )}
        </div>
      ) : null}
    </section>
  );
}
