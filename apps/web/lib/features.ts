// 提供前の機能を隠すためのフラグ。値は "1" か "true" のときだけ有効にする。
export function flagEnabled(value: string | undefined): boolean {
  return ["1", "true", "yes", "on"].includes((value ?? "").trim().toLowerCase());
}

// 現在は募集のみ。利用者向け画面は準備中にし、/coaching・/coach・/coach/apply・/coach/about
// から説明・応募・審査状況の確認を行える。APIのCOACHING_ENABLEDと揃えること。
export const coachingEnabled = flagEnabled(process.env.NEXT_PUBLIC_COACHING_ENABLED);

// 感覚メモを保存順に6個ずつまとめるふりかえり(AI)。感覚メモを Gemini に送る(プライバシーポリシーの委託先に書いてある)。
// API 側の SENSE_SUMMARY_ENABLED と揃えること。
export const senseSummaryEnabled = flagEnabled(process.env.NEXT_PUBLIC_SENSE_SUMMARY_ENABLED);

