import type { ReviewFocus, ReviewStatus } from "./types";

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
  queued: "コーチを探しています",
  offered: "コーチを探しています",
  in_review: "添削中",
  returned: "返却済み",
  clarifying: "補足を待っています",
  expired: "期限切れ",
};

// 添削の動画は30秒まで。少しの誤差はサーバーと同じく許す。
export const maxReviewSeconds = 30;
const secondsSlop = 1;

// videoDuration は、選んだ動画の長さ（秒）を読む。読めなければ null。
export function videoDuration(file: File): Promise<number | null> {
  return new Promise((resolve) => {
    const url = URL.createObjectURL(file);
    const video = document.createElement("video");
    video.preload = "metadata";
    const done = (value: number | null) => {
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
