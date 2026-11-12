// 動画添削の料金（本体 #569）。API の reviewPriceYen（apps/api/internal/httpx/review_payment.go）と
// チケットの商品（migrations 000159）と揃える。金額は API の応答を優先し、これは読み込み前の表示と文言に使う。

// 単発の動画添削1回の料金（税込）。どのコーチでも同じ。
export const reviewPriceYen = 500;
// コーチの報酬（単発の70%）。チケットで払った回も同じ額。
export const reviewCoachRewardYen = 350;
// チケット：6枚つづり 2,500円（税込）。1枚で添削1回。
export const ticketPackQuantity = 6;
export const ticketPackPriceYen = 2500;
// チケットの有効期限：決済完了日時から6か月。
export const ticketValidityMonths = 6;
