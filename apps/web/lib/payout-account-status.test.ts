import { describe, expect, it } from "vitest";
import { payoutAccountStatus, payoutAccountStatusLabels } from "./payout-account-status";

describe("payoutAccountStatus", () => {
  it.each([
    { flags: { stripe_linked: false, payouts_enabled: false }, expected: "unregistered" },
    { flags: { stripe_linked: true, payouts_enabled: false }, expected: "in_progress" },
    { flags: { stripe_linked: true, payouts_enabled: true }, expected: "registered" },
    // 口座 ID が消えていても、受け取れる状態なら登録済みとして扱う。
    { flags: { stripe_linked: false, payouts_enabled: true }, expected: "registered" },
  ])("maps $flags to $expected", ({ flags, expected }) => {
    expect(payoutAccountStatus(flags)).toBe(expected);
  });

  it("labels every status in Japanese", () => {
    expect(payoutAccountStatusLabels.registered).toBe("登録済み");
    expect(payoutAccountStatusLabels.in_progress).toBe("登録の途中・確認中");
    expect(payoutAccountStatusLabels.unregistered).toBe("未登録");
  });
});
