import { L } from "@/lib/i18n";
import { coachText, coachTexts, originalTextClass } from "@/lib/coaching-display";
import { clarifyReasonLabels } from "@/lib/review";
import type { ReviewResponse } from "@/lib/types";
import { formatChatDateTime } from "@/lib/format";

// コーチの返答（良い点・直す点・ドリル・見本動画・補足）。利用者とコーチの両方の画面で使う。
// 海外コーチの回答3点は、和訳があれば和訳を主に・原文を直下に併記する（本体 #916）。コーチ本人の画面（viewer="coach"）は原文だけ。
export function ReviewResponseView({ response, viewer = "requester" }: { response: ReviewResponse; viewer?: "requester" | "coach" }) {
  if (response.hidden) {
    return (
      <section className="rounded-xl border border-white/20 glass bg-white/10 p-3 text-sm text-slate-200">{L("この返答は運営が非表示にしました。")}</section>
    );
  }
  const mine = viewer === "coach";
  const good = coachText(response.good_point, response.good_point_ja, { mine });
  const fixes = coachTexts(response.fix_points, response.fix_points_ja, { mine });
  const drill = coachText(response.drill, response.drill_ja, { mine });
  const card = `max-w-[85%] space-y-1 rounded-xl px-3 py-2 text-sm break-words ${viewer === "coach" ? "self-end rounded-br-sm bg-blue-500/25" : "self-start rounded-bl-sm bg-white/10"}`;
  return (
    <section className="flex flex-col gap-3">
      <div className={card}>
        <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
          <h2 className="text-xs font-semibold text-emerald-300">{L("良い点")}</h2>
          <time dateTime={response.created_at} className="text-xs text-slate-300">{formatChatDateTime(response.created_at)}</time>
        </div>
        <p className="whitespace-pre-wrap text-slate-100">{good.text}</p>
        {good.original !== null ? <p lang="en" className={originalTextClass}>{good.original}</p> : null}
      </div>
      {fixes.map((fix, i) => (
        <div key={i} className={card}>
          <h2 className="text-xs font-semibold text-amber-300">{L("直す点{v0}", { v0: fixes.length > 1 ? ` ${i + 1}` : "" })}</h2>
          <p className="whitespace-pre-wrap text-slate-100">{fix.text}</p>
          {fix.original !== null ? <p lang="en" className={originalTextClass}>{fix.original}</p> : null}
        </div>
      ))}
      <div className={card}>
        <h2 className="text-xs font-semibold text-blue-300">{L("おすすめのドリル")}</h2>
        <p className="whitespace-pre-wrap text-slate-100">{drill.text}</p>
        {drill.original !== null ? <p lang="en" className={originalTextClass}>{drill.original}</p> : null}
      </div>
      {response.hitokoto ? (
        <div className={card}>
          <h2 className="text-xs font-semibold text-violet-300">{L("ひとこと")}</h2>
          <p className="whitespace-pre-wrap text-slate-100">{response.hitokoto}</p>
        </div>
      ) : null}
      {response.demo_video_url ? (
        <div className={card}>
          <h2 className="text-xs font-semibold text-slate-300">{L("見本動画")}</h2>
          <video src={response.demo_video_url} controls playsInline preload="metadata" className="w-full max-w-[260px] rounded-lg bg-black" />
        </div>
      ) : null}
      {response.clarify_reason ? (
        <div className={card}>
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
