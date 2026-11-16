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
    expect(contentRow?.value).toContain("Only the purchaser can use them.");
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
      expect(refund).not.toContain("(Terms, Article 6)");
      expect(refund).toContain("divided by 6, multiplied by the number of unused tickets");
    } finally {
      setActiveLocale("ja");
    }
  });

  it("チケット購入の確認は4行（内容・総額・有効期限・払戻し）で、確認画面だけで完結する（本体 #844）", () => {
    setActiveLocale("ja");
    const rows = ticketPackConfirmRows({ quantity: 6, amountYen: 2500, validityMonths: 6, now: new Date("2026-11-20T10:00:00+09:00") });
    expect(rows.map((r) => r.label)).toEqual(["内容", "総額", "有効期限", "払戻し"]);
    expect(rows[0].value).toBe("1回プランのチケット6枚（1枚で1回プラン1件）。購入した本人だけが使えます。譲ることはできません。");
    expect(rows[1].value).toBe("2,500円（税込）。6枚分をまとめてカードで先に支払います。支払いが済むと購入が確定し、すぐ使えます。");
    expect(rows[2].value).toContain("2027/05/20 10:00");
    expect(rows[2].value).toContain("申し込んだ依頼が終了して戻るチケットも、有効期限を過ぎていれば失効します。");
    const refund = rows[3].value;
    expect(refund).toContain("購入者の都合による払戻しは原則として行いません");
    expect(refund).toContain("確保したチケット1枚を戻します。これは確保の解除で、金銭の払戻しには当たりません");
    expect(refund).not.toContain("有効期限を過ぎてから戻るチケットは失効します");
    expect(refund).toContain("有効期限を過ぎていても1枚を出し直します");
    expect(refund).toContain("（1）サービス終了時は、終了の時点で未使用のチケット（有効期限にかかわらず）について、購入金額を6で割った額に未使用の枚数を掛けた額（1円未満は四捨五入）を払い戻します。");
    expect(refund).toContain("（3）");
    expect(refund).toContain("チケット1枚につき500円");
    expect(refund).toContain("失効後でも1枚につき500円");
    expect(refund).not.toContain("利用規約第6条");
    expect(refund).not.toMatch(/当社|チケットを戻すか返金/);
  });
});
