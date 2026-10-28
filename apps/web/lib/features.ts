// 提供前の機能を隠すためのフラグ。値は "1" か "true" のときだけ有効にする。
export function flagEnabled(value: string | undefined): boolean {
  return ["1", "true", "yes", "on"].includes((value ?? "").trim().toLowerCase());
}

// 現在は募集のみ。利用者向け画面は準備中にし、/coaching・/coach・/coach/apply・/coach/about
// から説明・応募・審査状況の確認を行える。APIのCOACHING_ENABLEDと揃えること。
export const coachingEnabled = flagEnabled(process.env.NEXT_PUBLIC_COACHING_ENABLED);

// 有料の1対1のやりとり(契約のやりとり欄・追加相談・添削の確認質問と補足)。電気通信事業の届出が
// 受理されるまで立てない。空の間は読むだけにして、送る欄は出さない。APIのCOACHING_MESSAGING_ENABLEDと揃えること。
export const coachingMessagingEnabled = flagEnabled(process.env.NEXT_PUBLIC_COACHING_MESSAGING_ENABLED);

export const messagingClosedNote = "メッセージのやりとりは準備中です。届いた内容は読めます。";

// 感覚メモを保存順に6個ずつまとめるふりかえり(AI)。感覚メモを Claude(Anthropic)に送る(プライバシーポリシーの委託先に書いてある)。
// API 側の SENSE_SUMMARY_ENABLED と揃えること。
export const senseSummaryEnabled = flagEnabled(process.env.NEXT_PUBLIC_SENSE_SUMMARY_ENABLED);
