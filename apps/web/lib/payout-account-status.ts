// 受け取り口座の登録状態。コーチ画面の「報酬」と管理画面で文言を揃える。
// 判定の根拠は Stripe の実状態を映す2値。payouts_enabled は Stripe の
// payouts_enabled を読んで記録したもの（受け取れる状態）、stripe_linked は
// 口座 ID の有無（登録を始めたか）。
// payouts_enabled が真なら、口座 ID の有無にかかわらず登録済みとして扱う。

export type PayoutAccountStatus = "registered" | "in_progress" | "unregistered";

export function payoutAccountStatus(flags: { stripe_linked: boolean; payouts_enabled: boolean }): PayoutAccountStatus {
  if (flags.payouts_enabled) return "registered";
  if (flags.stripe_linked) return "in_progress";
  return "unregistered";
}

export const payoutAccountStatusLabels: Record<PayoutAccountStatus, string> = {
  registered: "登録済み",
  in_progress: "登録の途中・確認中",
  unregistered: "未登録",
};
