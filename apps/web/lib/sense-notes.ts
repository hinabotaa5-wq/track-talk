// 感覚メモ: 練習で体を動かして感じた感覚を、本人だけが書き溜める。投稿とは別で、誰にも公開しない。

import { todayInTokyo } from "./format";

export const senseNoteMaxLength = 300;

// 器(ためるビン)1杯に入る感覚の数。
export const senseJarSize = 30;

export type SenseNote = {
  id: string;
  body: string;
  event: string | null;
  event_label: string | null;
  day: string;
  created_at: string;
  updated_at: string;
};

export type SenseNoteStats = {
  total: number;
  days: number;
  streak: number;
  today: number;
};

export const senseNotePlaceholder = "例: スタートの1歩目、地面を真下に押せた感じがした。腕は小さく速く振れた";

// いまの器の中身。満杯になったら次の器に移り、満杯にした器の数を filled で返す。
export function senseJarLevel(total: number) {
  const safe = Math.max(0, Math.floor(total));
  return { filled: Math.floor(safe / senseJarSize), current: safe % senseJarSize };
}

// 書いたあとに出すねぎらい。stats は書いたあとの値。
export function senseCheer(stats: SenseNoteStats) {
  const { filled, current } = senseJarLevel(stats.total);
  if (stats.total > 0 && current === 0) {
    return { title: "今日も頑張ったね", body: `器が感覚でいっぱいになった。${filled}杯目、おめでとう。` };
  }
  if (stats.total === 1) {
    return { title: "今日も頑張ったね", body: "はじめての感覚がたまった。体が覚えたことを、少しずつ残していこう。" };
  }
  if (stats.today > 1) {
    return { title: "今日も頑張ったね", body: `今日の感覚、${stats.today}つ目。気づきが多い日だったね。` };
  }
  if (stats.streak >= 2) {
    return { title: "今日も頑張ったね", body: `${stats.streak}日続けて書けてる。感覚が${stats.total}個たまったよ。` };
  }
  return { title: "今日も頑張ったね", body: `感覚が${stats.total}個たまったよ。また明日の体の声も聞かせてね。` };
}

// 一覧を日ごとにまとめる。notes は新しい順のまま渡す。
export function groupSenseNotesByDay(notes: SenseNote[]) {
  const groups: { day: string; notes: SenseNote[] }[] = [];
  for (const note of notes) {
    const last = groups[groups.length - 1];
    if (last && last.day === note.day) last.notes.push(note);
    else groups.push({ day: note.day, notes: [note] });
  }
  return groups;
}

// "2026-10-16" を "10月16日(金)" にする。年が今年でなければ年も付ける。
export function senseDayLabel(day: string, now = new Date()) {
  const [y, m, d] = day.split("-").map(Number);
  if (!y || !m || !d) return day;
  const week = "日月火水木金土"[new Date(Date.UTC(y, m - 1, d)).getUTCDay()];
  const thisYear = Number(todayInTokyo(now).slice(0, 4));
  return `${y === thisYear ? "" : `${y}年`}${m}月${d}日(${week})`;
}

// 器の中の粒の色。種目ごとに少しずつ違う色にして、積み重なりが見えるようにする。
const grainColors = ["#7dd3fc", "#a5b4fc", "#f9a8d4", "#fcd34d", "#6ee7b7", "#fdba74"];

export function senseGrainColor(index: number) {
  return grainColors[((index % grainColors.length) + grainColors.length) % grainColors.length];
}

// 1週間のまとめ(AI)。直近7日(今日を含む)の感覚メモから作る。
export type SenseSummary = {
  id: string;
  from_day: string;
  to_day: string;
  note_count: number;
  summary: { overview: string; good: string[]; concerns: string[]; next: string[] };
  created_at: string;
};

// next_available_at は、次のまとめを作れる時刻。すぐ作れるなら null。
export type SenseSummaryResponse = { summary: SenseSummary | null; next_available_at: string | null };

// まとめた期間。"10月14日(火)〜10月20日(月)"。
export function senseSummaryRangeLabel(summary: Pick<SenseSummary, "from_day" | "to_day">, now = new Date()) {
  return `${senseDayLabel(summary.from_day, now)}〜${senseDayLabel(summary.to_day, now)}`;
}

// 次のまとめを作れる日。"10月27日(火)"。すぐ作れるなら null。
export function senseSummaryNextLabel(nextAvailableAt: string | null, now = new Date()) {
  if (!nextAvailableAt) return null;
  return senseDayLabel(todayInTokyo(new Date(nextAvailableAt)), now);
}
