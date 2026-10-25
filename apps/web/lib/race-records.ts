// 競技会の記録: 日付・種目・記録・風・感想を本人だけが残し、種目ごとの推移をグラフで見る。感覚メモの延長。

import { appEvents } from "./practice-events";

// 記録の持ち方。time は1/100秒(小さいほどよい)、distance はcm、points は点(どちらも大きいほどよい)。
export type RaceMarkKind = "time" | "distance" | "points";

export type RaceRecord = {
  id: string;
  raced_on: string;
  event: string;
  event_label: string;
  mark: number;
  // 0.1m/s単位(+1.8m/s は 18)。風の公認が関係する種目だけ。
  wind: number | null;
  wind_aided: boolean;
  is_pb: boolean;
  memo: string;
  // 記録を出した大会の名前。書かなければ空。
  meet_name: string;
  round?: string;
  created_at: string;
};

export type RaceEvent = { event: string; event_label: string; kind: RaceMarkKind; wind: boolean; count: number };

export type RaceRecords = { events: RaceEvent[]; records: RaceRecord[] };

export const raceRecordRoundMaxLength = 20;
export const raceRoundSuggestions = ["予選", "準決勝", "決勝", "準々決勝", "タイムレース決勝"] as const;

export const raceRecordMemoMaxLength = 300;
export const raceRecordMeetNameMaxLength = 60;

const distanceEvents = new Set(["long-jump", "triple-jump", "high-jump", "pole-vault", "shot-put", "discus", "hammer", "javelin"]);
const pointsEvents = new Set(["heptathlon", "decathlon"]);
const windEvents = new Set(["100m", "200m", "100mh", "110mh", "long-jump", "triple-jump"]);

// 記録を残せる種目。アプリ内の種目と同じ。七種・十種は点数で残す。
export const raceRecordEvents = appEvents;

export function raceMarkKind(event: string): RaceMarkKind {
  if (distanceEvents.has(event)) return "distance";
  if (pointsEvents.has(event)) return "points";
  return "time";
}

export function raceEventHasWind(event: string) {
  return windEvents.has(event);
}

// 全角の数字や記号を半角にそろえる。
function toHalfWidth(text: string) {
  return text
    .trim()
    .replace(/[０-９]/g, (c) => String.fromCharCode(c.charCodeAt(0) - 0xfee0))
    .replace(/[．。]/g, ".")
    .replace(/[：]/g, ":")
    .replace(/[＋]/g, "+")
    .replace(/[－−ー]/g, "-");
}

// 見たままの記録("10.85"、"1:58.32"、"7.05"、"5800")を、保存する整数にする。読めなければ null。
export function parseRaceMark(text: string, kind: RaceMarkKind): number | null {
  const value = toHalfWidth(text).replace(/(秒|m|点|,)/g, "");
  if (kind === "points") {
    return /^\d{1,5}$/.test(value) && Number(value) > 0 ? Number(value) : null;
  }
  if (kind === "distance") {
    const match = /^(\d{1,3})(?:\.(\d{1,2}))?$/.exec(value);
    if (!match) return null;
    const cm = Number(match[1]) * 100 + Number((match[2] ?? "").padEnd(2, "0"));
    return cm > 0 ? cm : null;
  }
  const match = /^(?:(\d{1,2}):)?(\d{1,3})(?:\.(\d{1,2}))?$/.exec(value);
  if (!match) return null;
  const minutes = match[1] ? Number(match[1]) : 0;
  const seconds = Number(match[2]);
  // 分を書いたときは、秒は2桁の0〜59。
  if (match[1] && (match[2].length !== 2 || seconds >= 60)) return null;
  const hundredths = (minutes * 60 + seconds) * 100 + Number((match[3] ?? "").padEnd(2, "0"));
  return hundredths > 0 ? hundredths : null;
}

// 保存した整数を、見せる形にする。
export function formatRaceMark(mark: number, kind: RaceMarkKind) {
  if (kind === "points") return `${mark.toLocaleString("ja-JP")}点`;
  if (kind === "distance") return `${Math.floor(mark / 100)}.${String(mark % 100).padStart(2, "0")}m`;
  const whole = Math.floor(mark / 100);
  const fraction = String(mark % 100).padStart(2, "0");
  if (whole < 60) return `${whole}.${fraction}`;
  return `${Math.floor(whole / 60)}:${String(whole % 60).padStart(2, "0")}.${fraction}`;
}

// 入力欄に戻すときの形(単位を付けない)。
export function raceMarkInput(mark: number, kind: RaceMarkKind) {
  return formatRaceMark(mark, kind).replace(/(m|点|,)$/g, "").replace(/,/g, "");
}

// 風("+1.8"、"-0.5"、"0")を0.1m/s単位の整数にする。空なら undefined、読めなければ null。
export function parseRaceWind(text: string): number | null | undefined {
  const value = toHalfWidth(text).replace(/m\/?s$/i, "");
  if (value === "") return undefined;
  const match = /^([+-])?(\d)(?:\.(\d))?$/.exec(value);
  if (!match) return null;
  const tenths = Number(match[2]) * 10 + Number(match[3] ?? "0");
  return match[1] === "-" ? -tenths : tenths;
}

// 数字だけのキーボードでは + と - が打てないので、ボタンで符号を付け替える。符号が無ければ向かい風(-)にする。
export function toggleRaceWindSign(text: string) {
  const value = text.trim();
  const rest = value.replace(/^[+＋\-－−ー±]+/, "");
  return /^[\-－−ー]/.test(value) ? `+${rest}` : `-${rest}`;
}

export function formatRaceWind(wind: number) {
  const sign = wind > 0 ? "+" : wind < 0 ? "-" : "±";
  const abs = Math.abs(wind);
  return `${sign}${Math.floor(abs / 10)}.${abs % 10}`;
}

// 種目は練習相手募集と同じ並びで出す。
export function sortRaceEvents(events: RaceEvent[]) {
  const order = new Map<string, number>(raceRecordEvents.map((item, index) => [item.value, index]));
  return [...events].sort((a, b) => (order.get(a.event) ?? 99) - (order.get(b.event) ?? 99));
}

export type RaceChartPoint = { record: RaceRecord; x: number; y: number };

export type RaceChartBox = { width: number; height: number; left: number; right: number; top: number; bottom: number };

export const raceChartBox: RaceChartBox = { width: 320, height: 180, left: 48, right: 12, top: 14, bottom: 26 };

function dayNumber(day: string) {
  const [y, m, d] = day.split("-").map(Number);
  return Date.UTC(y, m - 1, d) / 86_400_000;
}

// 同日の記録は大会・ラウンド順に並べ、入力した順が逆でも決勝を最後にする。
const roundOrder: Record<string, number> = { "予備予選": 1, "予選": 2, "準々決勝": 3, "準決勝": 4, "決勝": 5, "タイムレース決勝": 5 };
export function sortRaceRecords(records: RaceRecord[]) {
  return [...records].sort((a, b) => a.raced_on.localeCompare(b.raced_on)
    || a.meet_name.localeCompare(b.meet_name)
    || (roundOrder[a.round ?? ""] ?? 0) - (roundOrder[b.round ?? ""] ?? 0)
    || a.created_at.localeCompare(b.created_at) || a.id.localeCompare(b.id));
}

// 小さな変化でグラフ全体を上下しないよう、記録の20%以上の幅と上下の余白を取る。
export function raceChartDomain(marks: number[], allowNegative = false) {
  if (!marks.length) return { low: 90, high: 110 };
  const min = Math.min(...marks), max = Math.max(...marks);
  const middle = (min + max) / 2;
  const span = Math.max((max - min) * 2, middle * 0.2, 2);
  const low = allowNegative ? middle - span / 2 : Math.max(0, middle - span / 2);
  return { low, high: low + span };
}

// 日付の間隔を基準にしつつ、同日の記録には最大12pxずつ横幅を確保する。
function raceChartX(records: RaceRecord[], box: RaceChartBox) {
  const sorted = sortRaceRecords(records);
  const byDay = new Map<string, RaceRecord[]>();
  for (const record of sorted) {
    const group = byDay.get(record.raced_on) ?? [];
    group.push(record);
    byDay.set(record.raced_on, group);
  }
  const days = [...byDay.keys()];
  const first = dayNumber(days[0] ?? "1970-01-01");
  const span = dayNumber(days.at(-1) ?? "1970-01-01") - first;
  const width = box.width - box.left - box.right;
  const extras = sorted.length - days.length;
  const gap = Math.min(12, width / Math.max(1, extras * 2));
  const dateWidth = width - extras * gap;
  const positions = new Map<string, number>();
  let preceding = 0;
  for (const [day, group] of byDay) {
    group.forEach((record, i) => positions.set(record.id, box.left + (span
      ? (dayNumber(day) - first) / span * dateWidth + (preceding + i) * gap
      : sorted.length > 1 ? i / (sorted.length - 1) * width : width / 2)));
    preceding += group.length - 1;
  }
  return positions;
}

export function raceChartPoints(records: RaceRecord[], kind: RaceMarkKind, box: RaceChartBox = raceChartBox): RaceChartPoint[] {
  const sorted = sortRaceRecords(records);
  const xs = raceChartX(sorted, box);
  const { low, high } = raceChartDomain(sorted.map((r) => r.mark));
  return sorted.map((record) => {
    const ratio = (record.mark - low) / (high - low);
    return { record, x: xs.get(record.id)!, y: box.top + (kind === "time" ? ratio : 1 - ratio) * (box.height - box.top - box.bottom) };
  });
}

// 単位の異なる種目は初回=100で比較する。タイム短縮・距離や得点の増加が上になる。
export function raceOverviewSeries(records: RaceRecord[], box: RaceChartBox = raceChartBox) {
  const ordered = sortRaceRecords(records);
  const xs = raceChartX(ordered, box);
  const groups = new Map<string, RaceRecord[]>();
  for (const record of ordered) groups.set(record.event, [...(groups.get(record.event) ?? []), record]);
  const series = sortRaceEvents([...groups].map(([event, rows]) => ({ event, event_label: rows[0].event_label, kind: raceMarkKind(event), wind: false, count: rows.length }))).map((event, index) => {
    const rows = groups.get(event.event)!;
    const baseline = rows[0].mark;
    return { ...event, color: `hsl(${(index * 137.5 + 195) % 360} 85% 72%)`, points: rows.map((record) => ({ record, index: 100 + (event.kind === "time" ? baseline - record.mark : record.mark - baseline) / baseline * 100 })) };
  });
  const domain = raceChartDomain([100, ...series.flatMap((s) => s.points.map((p) => p.index))], true);
  return { domain, series: series.map((s) => ({ ...s, points: s.points.map((p) => ({ ...p, x: xs.get(p.record.id)!, y: box.top + (domain.high - p.index) / (domain.high - domain.low) * (box.height - box.top - box.bottom) })) })) };
}
