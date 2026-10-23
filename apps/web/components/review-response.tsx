import Link from "next/link";
import { CoachAvatar } from "@/components/coach-photo";
import { coachEventLabels, coachTierLabels } from "@/lib/coach-recruit";
import { clarifyReasonLabels } from "@/lib/review";
import type { ReviewCoach, ReviewResponse } from "@/lib/types";

// 返答と一緒に届く、見てくれたコーチのプロフィール。
export function ReviewCoachCard({ coach }: { coach: ReviewCoach }) {
  return (
    <section className="space-y-2 rounded-xl border border-white/20 glass bg-white/10 p-3 text-sm">
      <h2 className="text-xs font-semibold text-slate-300">見てくれたコーチ</h2>
      <div className="flex items-center gap-3">
        <CoachAvatar url={coach.photo_url} name={coach.nickname} />
        <p className="font-bold text-white">
          {coach.nickname ?? "退会したユーザー"}
          <span className="ml-2 text-xs font-normal text-slate-300">{coachTierLabels[coach.tier]}</span>
        </p>
      </div>
      <dl className="space-y-1.5">
        <div>
          <dt className="text-xs text-slate-300">担当種目</dt>
          <dd className="text-slate-100">{coach.events.map((e) => coachEventLabels[e]).join("、")}</dd>
        </div>
        <div>
          <dt className="text-xs text-slate-300">記録</dt>
          <dd className="whitespace-pre-wrap text-slate-100">{coach.best_record}</dd>
        </div>
        <div>
          <dt className="text-xs text-slate-300">成績</dt>
          <dd className="whitespace-pre-wrap text-slate-100">{coach.achievements}</dd>
        </div>
        <div>
          <dt className="text-xs text-slate-300">競技年数</dt>
          <dd className="text-slate-100">{coach.years}年</dd>
        </div>
      </dl>
      {coach.id ? <Link href={`/coaches#coach-${coach.id}`} className="block text-sm text-blue-300 underline">このコーチに指名で依頼する</Link> : null}
    </section>
  );
}

// コーチの返答（良い点・直す点・ドリル・見本動画・補足）。利用者とコーチの両方の画面で使う。
export function ReviewResponseView({ response }: { response: ReviewResponse }) {
  if (response.hidden) {
    return (
      <section className="rounded-xl border border-white/20 glass bg-white/10 p-3 text-sm text-slate-200">
        この返答は運営が非表示にしました。
      </section>
    );
  }
  return (
    <section className="space-y-3 rounded-xl border border-white/20 glass bg-white/10 p-3 text-sm">
      <div className="space-y-1">
        <h2 className="text-xs font-semibold text-emerald-300">良い点</h2>
        <p className="whitespace-pre-wrap text-slate-100">{response.good_point}</p>
      </div>
      {response.fix_points.map((fix, i) => (
        <div key={i} className="space-y-1">
          <h2 className="text-xs font-semibold text-amber-300">直す点{response.fix_points.length > 1 ? ` ${i + 1}` : ""}</h2>
          <p className="whitespace-pre-wrap text-slate-100">{fix}</p>
        </div>
      ))}
      <div className="space-y-1">
        <h2 className="text-xs font-semibold text-blue-300">おすすめのドリル</h2>
        <p className="whitespace-pre-wrap text-slate-100">{response.drill}</p>
      </div>
      {response.demo_video_url ? (
        <div className="space-y-1">
          <h2 className="text-xs font-semibold text-slate-300">見本動画</h2>
          <video src={response.demo_video_url} controls playsInline preload="metadata" className="w-full rounded-lg bg-black" />
        </div>
      ) : null}
      {response.clarify_reason ? (
        <div className="space-y-1 border-t border-white/15 pt-3">
          {response.clarify_body ? <p className="whitespace-pre-wrap text-sm">確認質問：{response.clarify_body}</p> : null}
          <h2 className="text-xs font-semibold text-slate-300">補足（「{clarifyReasonLabels[response.clarify_reason]}」への答え）</h2>
          <p className="whitespace-pre-wrap text-slate-100">{response.clarification ?? "コーチの補足を待っています"}</p>
        </div>
      ) : null}
    </section>
  );
}
