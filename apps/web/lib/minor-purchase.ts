// 18歳未満が自分で払う動画添削・追加相談の、購入のたびの宣言。
// 文案は運営者が 2026-11-05 に確定。変えたら API の minorPurchaseDeclarationVersion も上げる。
export const minorPurchaseDeclarationLabel = "保護者に許可をもらいました";
export const minorPurchaseDeclarationNote = "未成年の方は、購入する前に保護者の許可を得てください。";

// API に送る本文。18歳以上には送らない（送っても使われない）。
export function minorDeclarationBody(minor: boolean, checked: boolean): { guardian_permission?: boolean } {
  return minor ? { guardian_permission: checked } : {};
}
