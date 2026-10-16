// 提供前の機能を隠すためのフラグ。値は "1" か "true" のときだけ有効にする。
export function flagEnabled(value: string | undefined): boolean {
  return ["1", "true", "yes", "on"].includes((value ?? "").trim().toLowerCase());
}

// オンラインコーチング（生年月・保護者の同意・プッシュ通知）の画面。
// プライバシーポリシーは「未提供」で書いているので、提供を始めるまで本番では立てない。
// API 側の COACHING_ENABLED と揃えること。
export const coachingEnabled = flagEnabled(process.env.NEXT_PUBLIC_COACHING_ENABLED);
