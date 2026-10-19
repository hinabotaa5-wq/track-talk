const jst = "Asia/Tokyo";

export function formatDateTime(iso: string) {
  return new Intl.DateTimeFormat("ja-JP", {
    timeZone: jst,
    dateStyle: "medium",
    timeStyle: "short",
  }).format(new Date(iso));
}

export function formatRelativeTime(iso: string, now = Date.now()) {
  const then = new Date(iso).getTime();
  if (Number.isNaN(then)) return "";
  const seconds = Math.max(0, Math.floor((now - then) / 1000));
  if (seconds < 60) return "たった今";
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}分前`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}時間前`;
  const days = Math.floor(hours / 24);
  if (days < 7) return `${days}日前`;
  return formatDateTime(iso);
}

export function formatDate(iso: string) {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) {
    return iso.slice(0, 10);
  }
  return new Intl.DateTimeFormat("ja-JP", {
    timeZone: jst,
    dateStyle: "medium",
  }).format(d);
}

// 日本時間の今日を "YYYY-MM-DD" で返す。starts_on や raced_on の日付と同じ形。
// datetime-local の入力欄（"2026-10-20T18:00"）用に、日時を日本時間の壁時計で文字にする。
// 端末のタイムゾーンで読み書きすると、海外にいる人や端末の設定が違う人の練習日時がずれる。
export function toJstInput(iso: string) {
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return "";
  const parts = Object.fromEntries(
    new Intl.DateTimeFormat("en-CA", {
      timeZone: jst,
      hourCycle: "h23",
      year: "numeric",
      month: "2-digit",
      day: "2-digit",
      hour: "2-digit",
      minute: "2-digit",
    })
      .formatToParts(date)
      .map((part) => [part.type, part.value]),
  );
  return `${parts.year}-${parts.month}-${parts.day}T${parts.hour}:${parts.minute}`;
}

// toJstInput の逆。入力欄の値を日本時間として読む。読めなければ Invalid Date。
export function fromJstInput(value: string) {
  if (!/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}$/.test(value)) return new Date(NaN);
  return new Date(`${value}:00+09:00`);
}

export function todayInTokyo(now = new Date()) {
  return new Intl.DateTimeFormat("en-CA", { timeZone: jst }).format(now);
}

// 金額を "1,000円" の形にする。
export function formatYen(yen: number) {
  return `${yen.toLocaleString("ja-JP")}円`;
}
