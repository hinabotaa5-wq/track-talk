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
  created_at: string;
};

export type RaceEvent = { event: string; event_label: string; kind: RaceMarkKind; wind: boolean; count: number };

export type RaceRecords = { events: RaceEvent[]; records: RaceRecord[] };

export const raceRecordMemoMaxLength = 300;

const distanceEvents = new Set(["long-jump", "triple-jump", "high-jump", "pole-vault", "shot-put", "discus", "hammer", "javelin"]);
const pointsEvents = new Set(["heptathlon", "decathlon"]);
const windEvents = new Set(["100m", "200m", "100mh", "110mh", "long-jump", "triple-jump"]);

// 記録を残せる種目。アプリ内の種目と同じ。七種・十種は、以前に付けた記録を見られるように点数の扱いだけ残してある。
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

// グラフの点の位置。横は日付、縦は記録で、よい記録ほど上に来る(トラックは縦軸を反転する)。
// records は1つの種目の、古い順。
export function raceChartPoints(records: RaceRecord[], kind: RaceMarkKind, box: RaceChartBox = raceChartBox): RaceChartPoint[] {
  if (records.length === 0) return [];
  const innerWidth = box.width - box.left - box.right;
  const innerHeight = box.height - box.top - box.bottom;
  const days = records.map((record) => dayNumber(record.raced_on));
  const firstDay = Math.min(...days);
  const daySpan = Math.max(...days) - firstDay;
  const marks = records.map((record) => record.mark);
  const low = Math.min(...marks);
  const markSpan = Math.max(...marks) - low;
  return records.map((record, index) => {
    const x = daySpan === 0 ? box.left + innerWidth / 2 : box.left + ((days[index] - firstDay) / daySpan) * innerWidth;
    const ratio = markSpan === 0 ? 0.5 : (record.mark - low) / markSpan;
    // ratio は記録の大きさ。トラックは小さいほど上、フィールドと混成は大きいほど上。
    const fromTop = kind === "time" ? ratio : 1 - ratio;
    return { record, x, y: box.top + fromTop * innerHeight };
  });
}
