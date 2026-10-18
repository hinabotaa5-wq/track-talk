// Groups a season's meets by the month they start in, with upcoming meets above finished ones.

export type Scheduled = { starts_on: string; ends_on: string | null };

export type MonthGroup<T> = { key: string; label: string; items: T[] };

export type SeasonSchedule<T> = { upcoming: MonthGroup<T>[]; ended: MonthGroup<T>[] };

function day(iso: string) {
  return iso.slice(0, 10);
}

function lastDay(item: Scheduled) {
  return day(item.ends_on ?? item.starts_on);
}

// A meet is over the day after its last day, in Japan time.
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

// Upcoming months run from the current month onward. Finished meets go to their own list with the
// latest month first, even when they started in the current month.
export function seasonSchedule<T extends Scheduled>(items: T[], today: string): SeasonSchedule<T> {
  const upcoming = items.filter((item) => !isOver(item, today)).sort(byStart);
  const ended = items.filter((item) => isOver(item, today)).sort((a, b) => byStart(b, a));
  return { upcoming: groupByMonth(upcoming), ended: groupByMonth(ended) };
}
