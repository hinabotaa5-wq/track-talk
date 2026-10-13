import type { PracticeReputation } from "@/lib/types";

export function practiceScoreLabel(score: string) {
  if (score === "good") return "良い";
  if (score === "ok") return "普通";
  if (score === "bad") return "悪い";
  return score;
}

export function practiceReputationLine(reputation?: PracticeReputation) {
  if (!reputation) return "";
  if (reputation.good + reputation.ok + reputation.bad === 0) return "まだ評価がありません";
  return `良い ${reputation.good} / 普通 ${reputation.ok} / 悪い ${reputation.bad}`;
}

export function PracticeReputationSummary({
  reputation,
  heading,
}: {
  reputation?: PracticeReputation | null;
  heading: string;
}) {
  if (!reputation) return null;
  const line = practiceReputationLine(reputation);
  return (
    <section className="rounded-xl border bg-surface p-4">
      <h2 className="text-sm font-medium text-muted">{heading}</h2>
      <p className="mt-1 text-sm">{line}</p>
      {reputation.comments && reputation.comments.length > 0 ? (
        <ul className="mt-3 space-y-2">
          {reputation.comments.map((item) => (
            <li key={`${item.created_at}:${item.score}:${item.comment}`} className="text-sm">
              <span className="text-muted">{practiceScoreLabel(item.score)}</span>
              <span className="mt-1 block whitespace-pre-wrap">{item.comment}</span>
            </li>
          ))}
        </ul>
      ) : null}
    </section>
  );
}
