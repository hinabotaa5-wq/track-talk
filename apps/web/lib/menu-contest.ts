// エグ練大会: 11月〜3月に2週間ごとに開く。その回で一番エグかった練習を1人1件応募し、
// 運営が選んだトップ3にみんなで投票して1位を決める。日程は API が決める。

export const menuContestName = "エグ練大会";
export const menuContestMaxLength = 100;
export const menuContestPlaceholder = "例：300m×15本（レスト3分）。最後の3本は脚が動かなくて、次元が歪んだ";

export type MenuContestPhase = "entry" | "picking" | "voting";

export type MenuContestFinalist = {
  id: string;
  body: string;
  author_name: string | null;
  mine: boolean;
  votes?: number;
  winner?: boolean;
};

export type MenuContestRound = {
  start: string;
  number: number;
  phase: MenuContestPhase;
  entry_ends_at: string;
  vote_starts_at: string;
  vote_ends_at: string;
  my_entry: { body: string; updated_at: string } | null;
  finalists: MenuContestFinalist[];
  my_vote: string | null;
};

export type MenuContestResult = {
  start: string;
  number: number;
  finalists: MenuContestFinalist[];
};

export type MenuContestState = {
  round: MenuContestRound | null;
  next_starts_at?: string;
  last_result: MenuContestResult | null;
};

// 残り時間を「3日 4時間 12分」の形にする。1分未満は切り上げて「1分」にし、0 になったら null。
export function remainingParts(untilMs: number, nowMs: number) {
  const ms = untilMs - nowMs;
  if (!(ms > 0)) return null;
  const totalMinutes = Math.ceil(ms / 60000);
  return {
    days: Math.floor(totalMinutes / 1440),
    hours: Math.floor((totalMinutes % 1440) / 60),
    minutes: totalMinutes % 60,
    // 残り24時間を切ったら目立たせる。
    urgent: ms <= 24 * 60 * 60 * 1000,
  };
}

export function formatRemaining(parts: { days: number; hours: number; minutes: number }) {
  const out: string[] = [];
  if (parts.days > 0) out.push(`${parts.days}日`);
  if (parts.days > 0 || parts.hours > 0) out.push(`${parts.hours}時間`);
  out.push(`${parts.minutes}分`);
  return out.join(" ");
}

// いまの段階で数える締め切りと、その見出し。選考中は投票の始まりまでを数える。
export function menuContestDeadline(round: MenuContestRound) {
  switch (round.phase) {
    case "entry":
      return { label: "応募受付中", until: round.entry_ends_at, prefix: "応募終了まで" };
    case "picking":
      return { label: "運営が選考中", until: round.vote_starts_at, prefix: "投票開始まで" };
    case "voting":
      return { label: "投票受付中", until: round.vote_ends_at, prefix: "投票終了まで" };
  }
}

export function winners(result: MenuContestResult) {
  return result.finalists.filter((f) => f.winner);
}
