import { currentLocale, englishLoaded, L } from "@/lib/i18n";
const jst = "Asia/Tokyo";

// 吹き出し・返答カード用。端末の設定によらず日本時間で、年を省いて表示する。
export function formatChatDateTime(iso: string) {
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return "";
  const parts = Object.fromEntries(new Intl.DateTimeFormat("ja-JP", {
    timeZone: jst, month: "numeric", day: "numeric", hour: "numeric", minute: "2-digit", hourCycle: "h23",
  }).formatToParts(date).map((part) => [part.type, part.value]));
  return `${parts.month}/${parts.day} ${parts.hour}:${parts.minute}`;
}

// 英語表示のときは英語の書式（"Nov 18, 2026, 11:05 AM"）にする。日本語表示は従来どおり（本体 #944、運営者決定 2026-11-18）。
export function formatDateTime(iso: string) {
  return new Intl.DateTimeFormat(currentLocale() === "en" && englishLoaded() ? "en-US" : "ja-JP", {
    timeZone: jst,
    dateStyle: "medium",
    timeStyle: "short",
  }).format(new Date(iso));
}

export function formatRelativeTime(iso: string, now = Date.now()) {
  const then = new Date(iso).getTime();
  if (Number.isNaN(then)) return "";
  const seconds = Math.max(0, Math.floor((now - then) / 1000));
  if (seconds < 60) return L("たった今");
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return L("{minutes}分前", { minutes });
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return L("{hours}時間前", { hours });
  const days = Math.floor(hours / 24);
  if (days < 7) return L("{days}日前", { days });
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
  return L("{v0}円", { v0: yen.toLocaleString("ja-JP") });
}

// 英語表示のときだけ、選手向けの価格をドルで見せるための固定レート（本体 #1085、運営者決定 2026-11-27）。
// 表示だけの換算で、請求・API・DB・Stripe・コーチ報酬は円のまま。レートの変更はこの定数の書き換え（PR）でのみ行う。
export const displayYenPerUsd = 160;

// 英語辞書を読み込み済みの英語表示のときだけドルにする（formatDateTime と同じ判定）。読み込み前は円のまま。
function showsUsd() {
  return currentLocale() === "en" && englishLoaded();
}

const usdAmount = new Intl.NumberFormat("en-US", { minimumFractionDigits: 2, maximumFractionDigits: 2 });
const usdCurrency = new Intl.NumberFormat("en-US", { style: "currency", currency: "USD" });

// 円をドルへ換算し、セント単位で四捨五入する。¥500 → 3.13。各金額は独立に換算する（差額から逆算しない）。
export function yenToDisplayUsd(yen: number) {
  return Math.round((yen * 100) / displayYenPerUsd) / 100;
}

// 選手向けの価格表示。英語表示は "$6.25"、日本語表示は formatYen と同じ "1,000円"。
export function formatDisplayPrice(yen: number) {
  return showsUsd() ? usdCurrency.format(yenToDisplayUsd(yen)) : formatYen(yen);
}

// 記号を文言側に持つ鍵（"スタンダード ¥{v0}" ↔ "Standard ${v0}"）に差し込む数字だけ。英語表示は "6.25"、日本語表示は "1,000"。
export function displayPriceAmount(yen: number) {
  return showsUsd() ? usdAmount.format(yenToDisplayUsd(yen)) : yen.toLocaleString("ja-JP");
}
