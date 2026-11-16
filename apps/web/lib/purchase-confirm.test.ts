import { describe, expect, it } from "vitest";
import { reviewConfirmRows, ticketPackConfirmRows } from "./purchase-confirm";
import { installEnglish, setActiveLocale } from "./i18n";
import { en } from "./i18n-en";

describe("purchase-confirm", () => {
  it("カード決済の確認文が運営者決定の短縮文と一致する", () => {
    setActiveLocale("ja");
    const rows = reviewConfirmRows({
      method: "card",
      priceYen: 500,
      hasVideo: true,
      coachName: "テストコーチ",
    });
    const cancelRefundRow = rows.find((r) => r.label === "キャンセル・返金");
    expect(cancelRefundRow).toBeDefined();
    expect(cancelRefundRow?.value).toBe(
      "次の画面でカードの代金を仮押さえし、回答が返った時に確定します。着手前はキャンセルでき、その場合は代金はかかりません。着手後はキャンセルできません。期限までに返らない場合やコーチが返却した場合は、代金はかかりません（確定済みの場合は返金します）。回答返却後の返金は利用規約に定める場合に限ります。",
    );
    for (const old of [
      "確保（仮押さえ）",
      "決済が確定していない場合は仮押さえを取り消し（請求は確定しません）",
      "確定している場合は返金します",
      "確保を取り消すので",
    ]) {
      expect(cancelRefundRow?.value).not.toContain(old);
    }
    expect(cancelRefundRow?.value).toContain("回答返却後の返金は利用規約に定める場合に限ります");
  });

  it("カード確認文の英語訳のキーが1件だけで、日本語文のままでも英語にフォールバックしない", () => {
    expect(Object.keys(en).filter((k) => k.startsWith("次の画面でカードの代金を"))).toEqual([
      "次の画面でカードの代金を仮押さえし、回答が返った時に確定します。着手前はキャンセルでき、その場合は代金はかかりません。着手後はキャンセルできません。期限までに返らない場合やコーチが返却した場合は、代金はかかりません（確定済みの場合は返金します）。回答返却後の返金は利用規約に定める場合に限ります。",
    ]);
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
    expect(cardCancelRow?.value).toBe(
      "Your card is authorized on the next screen and charged when the review is returned. You can cancel before the coach starts, in which case you won't be charged. You can't cancel once the coach starts. If the review isn't returned by the deadline or the coach returns it, you won't be charged (if the payment has already been finalized, you will be refunded). After the review is returned, refunds follow the Terms.",
    );
    expect(cardCancelRow?.value).not.toContain("the authorization is voided if the payment is not yet finalized");
    setActiveLocale("ja");
  });

  it("無料チケットの申込確認では、運営者・コーチ都合の終了の出し直しを書き分ける（本体 #797）", () => {
    setActiveLocale("ja");
    const rewardRows = reviewConfirmRows({
      method: "tickets",
      priceYen: 0,
      hasVideo: true,
      ticketKind: "reward",
    });
    const rewardCancel = rewardRows.find((r) => r.label === "キャンセル・チケットの戻り")?.value;
    expect(rewardCancel).toContain("有効期限を過ぎていても1枚を出し直します");
    expect(rewardCancel).not.toContain("有効期限を過ぎた分は失効します");

    const normalRows = reviewConfirmRows({
      method: "tickets",
      priceYen: 0,
      hasVideo: true,
      ticketKind: "normal",
    });
    const normalCancel = normalRows.find((r) => r.label === "キャンセル・チケットの戻り")?.value;
    expect(normalCancel).toContain("有効期限を過ぎた分は失効します");
    expect(normalCancel).not.toContain("出し直します");

    const packRefund = ticketPackConfirmRows({ quantity: 6, amountYen: 2500, validityMonths: 6 })
      .find((row) => row.label === "払戻し")?.value;
    expect(packRefund).toContain("有効期限を過ぎていても1枚を出し直します");
  });

  it("shows the free-ticket reissue exception in English too (本体 #797)", () => {
    installEnglish(en);
    setActiveLocale("en");
    try {
      const rewardCancel = reviewConfirmRows({
        method: "tickets",
        priceYen: 0,
        hasVideo: true,
        ticketKind: "reward",
      }).find((r) => r.label === "Cancellation and ticket returns")?.value;
      expect(rewardCancel).toContain("a new ticket is issued even after expiry");

      const packRefund = ticketPackConfirmRows({ quantity: 6, amountYen: 2500, validityMonths: 6 })
        .find((row) => row.label === "Refunds")?.value;
      expect(packRefund).toContain("a new ticket is issued even after expiry");
    } finally {
      setActiveLocale("ja");
    }
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
