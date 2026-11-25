// 動画添削の料金（本体 #569）。API の reviewPriceYen（apps/api/internal/httpx/review_payment.go）と
// チケットの商品（migrations 000159）と揃える（価格は本体 #983 で1回1,000円に改定。チケットの販売は本体 #1031 で終了）。金額は API の応答を優先し、これは読み込み前の表示と文言に使う。

// 単発の動画添削1回の料金（税込）。どのコーチでも同じ。
export const reviewPriceYen = 1000;
// コーチの報酬（単発の70%）。チケットで払った回も同じ額。
export const reviewCoachRewardYen = 700;
// チケットの有効期限：決済完了日時から6か月。
export const ticketValidityMonths = 6;
