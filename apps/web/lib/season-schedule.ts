// シーズンの大会を始まる月ごとにまとめ、これからの大会を終わった大会より上に置く。

export type Scheduled = { starts_on: string; ends_on: string | null };

export type MonthGroup<T> = { key: string; label: string; items: T[] };

export type SeasonSchedule<T> = { upcoming: MonthGroup<T>[]; ended: MonthGroup<T>[] };

function day(iso: string) {
  return iso.slice(0, 10);
}

function lastDay(item: Scheduled) {
  return day(item.ends_on ?? item.starts_on);
}

// 大会は、最終日の翌日（日本時間）に終わったものとする。
export function isOver(item: Scheduled, today: string) {
  return lastDay(item) < today;
}

export function isOngoing(item: Scheduled, today: string) {
  return day(item.starts_on) <= today && !isOver(item, today);
}

function monthLabel(key: string) {
  return `${Number(key.slice(5, 7))}月`;
}

function groupByMonth<T extends Scheduled>(items: T[]): MonthGroup<T>[] {
  const groups: MonthGroup<T>[] = [];
  for (const item of items) {
    const key = day(item.starts_on).slice(0, 7);
    const last = groups.at(-1);
    if (last?.key === key) last.items.push(item);
    else groups.push({ key, label: monthLabel(key), items: [item] });
  }
  return groups;
}

function byStart(a: Scheduled, b: Scheduled) {
  return day(a.starts_on).localeCompare(day(b.starts_on)) || lastDay(a).localeCompare(lastDay(b));
}

// これからの月は今月から先。終わった大会は別の一覧にまとめ、新しい月を先にする。
// 今月に始まった大会でもそちらに入れる。
export function seasonSchedule<T extends Scheduled>(items: T[], today: string): SeasonSchedule<T> {
  const upcoming = items.filter((item) => !isOver(item, today)).sort(byStart);
  const ended = items.filter((item) => isOver(item, today)).sort((a, b) => byStart(b, a));
  return { upcoming: groupByMonth(upcoming), ended: groupByMonth(ended) };
}
