import type { PracticeAttendance, PracticeReputation } from "@/lib/types";
import { TargetReport } from "./target-report";

export function practiceScoreLabel(score: string) {
  if (score === "good") return "良い";
  if (score === "ok") return "普通";
  if (score === "bad") return "悪い";
  if (score === "no_show") return "ドタキャン";
  return score;
}

// 参加希望者の参加率。承認されて評価された回数がない間は実績なしとして扱う。
export function practiceAttendanceLine(attendance?: PracticeAttendance) {
  if (!attendance || attendance.total === 0) return "参加実績はまだありません";
  const attended = attendance.total - attendance.no_show;
  const percent = Math.round((attended / attendance.total) * 100);
  return `参加率 ${percent}%（${attendance.total}回中 ドタキャン${attendance.no_show}回）`;
}

export function practiceReputationLine(reputation?: PracticeReputation) {
  if (!reputation) return "";
  if (reputation.good + reputation.ok + reputation.bad === 0) return "まだ評価がありません";
  return `良い ${reputation.good} / 普通 ${reputation.ok} / 悪い ${reputation.bad}`;
}

// reportNext があるときは、ひとことごとに通報ボタンを出す(ログイン後に戻る画面)。
export function PracticeReputationSummary({
  reputation,
  heading,
  reportNext,
}: {
  reputation?: PracticeReputation | null;
  heading: string;
  reportNext?: string;
}) {
  if (!reputation) return null;
  const line = practiceReputationLine(reputation);
  return (
    <section className="rounded-xl border border-white/20 glass bg-white/10 p-4">
      <h2 className="text-sm font-medium text-slate-200">{heading}</h2>
      <p className="mt-1 text-sm text-slate-200">{line}</p>
      {reputation.comments && reputation.comments.length > 0 ? (
        <ul className="mt-3 space-y-2">
          {reputation.comments.map((item) => (
            <li key={item.id} className="text-sm">
              <span className="flex items-center justify-between gap-3">
                <span className="text-slate-200">{practiceScoreLabel(item.score)}</span>
                {reportNext ? <TargetReport targetType="practice_rating" id={item.id} next={reportNext} noun="ひとこと" /> : null}
              </span>
              <span className="mt-1 block whitespace-pre-wrap">{item.comment}</span>
            </li>
          ))}
        </ul>
      ) : null}
    </section>
  );
}
