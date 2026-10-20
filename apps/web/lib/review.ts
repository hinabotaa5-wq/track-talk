import type { ClarifyReason, CoachEvent, HelpfulAnswer, ReviewFocus, ReviewPayMethod, ReviewStatus } from "./types";

// 添削を受け付ける種目。最初は100mだけにし、コーチが増えたら足す（API の reviewEvents と合わせる）。
export const reviewEvents: CoachEvent[] = ["100m"];

// 見てほしい点。並びは画面に出す順。自由記述は置かない（連絡先の受け渡しを防ぐため）。
export const reviewFocusLabels: Record<ReviewFocus, string> = {
  start: "スタート",
  acceleration: "加速",
  "top-speed": "トップスピード",
  takeoff: "踏切",
  arm: "腕振り",
  contact: "接地",
  rhythm: "リズム",
  other: "その他",
};

export const reviewFocusOptions = Object.keys(reviewFocusLabels) as ReviewFocus[];

export const reviewStatusLabels: Record<ReviewStatus, string> = {
  awaiting_payment: "お支払い待ち",
  queued: "コーチを探しています",
  offered: "コーチを探しています",
  in_review: "添削中",
  returned: "返却済み",
  clarifying: "補足を待っています",
  expired: "期限切れ",
};

export const reviewPayMethodLabels: Record<ReviewPayMethod, string> = {
  card: "カード・Apple Pay・Google Pay",
  paypay: "PayPay",
};

// 「ここがわからなかった」の選択肢。自由記述は置かない（やりとりは一方通行のまま）。
export const clarifyReasonLabels: Record<ClarifyReason, string> = {
  term: "用語がわからない",
  drill: "ドリルのやり方がわからない",
  fix: "直す点の意味がわからない",
  other: "その他",
};

export const helpfulLabels: Record<HelpfulAnswer, string> = {
  yes: "役に立った",
  some: "少し",
  no: "役に立たなかった",
};

// 返答の1項目の上限（サーバーと同じ）。
export const maxResponseChars = 300;

// 添削の動画は30秒まで。少しの誤差はサーバーと同じく許す。
export const maxReviewSeconds = 30;
const secondsSlop = 1;
const videoDurationTimeoutMs = 10_000;

// videoDuration は、選んだ動画の長さ（秒）を読む。読めなければ null。
export function videoDuration(file: File): Promise<number | null> {
  return new Promise((resolve) => {
    const url = URL.createObjectURL(file);
    const video = document.createElement("video");
    video.preload = "metadata";
    // 読めない形式でメタデータも失敗も来ないと待ち続けて画面が固まるので、一定時間で諦める。
    const timer = window.setTimeout(() => done(null), videoDurationTimeoutMs);
    const done = (value: number | null) => {
      window.clearTimeout(timer);
      video.onloadedmetadata = null;
      video.onerror = null;
      video.removeAttribute("src");
      video.load();
      URL.revokeObjectURL(url);
      resolve(value);
    };
    video.onloadedmetadata = () => done(Number.isFinite(video.duration) ? video.duration : null);
    video.onerror = () => done(null);
    video.src = url;
  });
}

export function tooLong(seconds: number | null) {
  return seconds !== null && seconds > maxReviewSeconds + secondsSlop;
}
