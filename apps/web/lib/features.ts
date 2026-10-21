// 提供前の機能を隠すためのフラグ。値は "1" か "true" のときだけ有効にする。
export function flagEnabled(value: string | undefined): boolean {
  return ["1", "true", "yes", "on"].includes((value ?? "").trim().toLowerCase());
}

// オンラインコーチング（生年月・保護者の同意・プッシュ通知・添削動画・フォーム添削・
// パーソナル指導・コーチの報酬）の画面。紹介ページ（/coaching）とコーチの応募（/coach/apply）は
// 提供前でも開けておく。プライバシーポリシーは「未提供」で書いているので、
// 提供を始めるまで本番では立てない。API 側の COACHING_ENABLED と揃えること。
export const coachingEnabled = flagEnabled(process.env.NEXT_PUBLIC_COACHING_ENABLED);

// 感覚メモの1週間のまとめ(AI)。感覚メモを Gemini に送る(プライバシーポリシーの委託先に書いてある)。
// API 側の SENSE_SUMMARY_ENABLED と揃えること。
export const senseSummaryEnabled = flagEnabled(process.env.NEXT_PUBLIC_SENSE_SUMMARY_ENABLED);
