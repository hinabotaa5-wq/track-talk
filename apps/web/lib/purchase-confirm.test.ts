import { describe, expect, it } from "vitest";
import { reviewConfirmRows, ticketPackConfirmRows } from "./purchase-confirm";
import { installEnglish, setActiveLocale } from "./i18n";
import { en } from "./i18n-en";

describe("purchase-confirm", () => {
  it("カード決済の確認文で、仮押さえ取消しと返金の表記が状態で分かれている", () => {
    setActiveLocale("ja");
    const rows = reviewConfirmRows({
      method: "card",
      priceYen: 500,
      hasVideo: true,
      coachName: "テストコーチ",
    });
    const cancelRefundRow = rows.find((r) => r.label === "キャンセル・返金");
    expect(cancelRefundRow).toBeDefined();
    expect(cancelRefundRow?.value).toContain("仮押さえを取り消し（請求は確定しません）");
    expect(cancelRefundRow?.value).toContain("確定している場合は返金します");
    expect(cancelRefundRow?.value).not.toContain("期限までに返らない場合やコーチ都合の返却では全額を返金します");
  });

  it("英語辞書で 1 review の表記が消え、One ticket covers one One-time Plan request. 相当になっている", () => {
    installEnglish(en);
    setActiveLocale("en");
    const rows = ticketPackConfirmRows({
      quantity: 6,
      amountYen: 2500,
      validityMonths: 6,
    });
    const contentRow = rows.find((r) => r.label === "Message" || r.label === "内容");
    expect(contentRow).toBeDefined();
    expect(contentRow?.value).toContain("6 One-time Plan tickets (One ticket covers one One-time Plan request.)");
    expect(contentRow?.value).not.toContain("1 review");

    const cardRows = reviewConfirmRows({
      method: "card",
      priceYen: 500,
      hasVideo: true,
      coachName: "Test Coach",
    });
    const cardCancelRow = cardRows.find((r) => r.label === "Cancellation and refunds");
    expect(cardCancelRow).toBeDefined();
    expect(cardCancelRow?.value).toContain("the authorization is voided if the payment is not yet finalized (you will not be charged), or refunded if already finalized");
    setActiveLocale("ja");
  });

  it("distinguishes a returned reservation from a monetary refund in English", () => {
    installEnglish(en);
    setActiveLocale("en");
    try {
      const refund = ticketPackConfirmRows({ quantity: 6, amountYen: 2500, validityMonths: 6 })
        .find((row) => row.label === "Refunds")?.value;
      expect(refund).toContain("the one reserved ticket is returned");
      expect(refund).toContain("you receive a refund of ¥500 per ticket even after expiry");
      expect(refund).not.toContain("returned or refunded");
    } finally {
      setActiveLocale("ja");
    }
  });
});
