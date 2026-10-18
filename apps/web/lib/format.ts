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
export function todayInTokyo(now = new Date()) {
  return new Intl.DateTimeFormat("en-CA", { timeZone: jst }).format(now);
}
