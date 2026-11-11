// 動画添削の開始告知の文言（本体 #569）。日付を画面に直書きせず、環境変数 NEXT_PUBLIC_VIDEO_REVIEW_NOTICE で差し替える。
// 未設定・空の間は出さない（既定は非表示）。出すときは「11/14(土)に開始します！」のような短い1文を入れる。
// 告知は日本語だけで出す（運営者が入れた文言をそのまま出すので、英語表示でも訳さない）。
export function videoReviewStartNotice(value: string | undefined = process.env.NEXT_PUBLIC_VIDEO_REVIEW_NOTICE): string | null {
  const text = (value ?? "").trim();
  return text ? text : null;
}
