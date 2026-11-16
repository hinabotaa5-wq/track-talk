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
      question: "相談",
      eventLabel: "100m",
      videoFileName: "run.mp4",
      videoFileSizeBytes: 1,
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
      question: "相談",
      eventLabel: "100m",
      videoFileName: "run.mp4",
      videoFileSizeBytes: 1,
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
      question: "相談",
      eventLabel: "100m",
      videoFileName: "run.mp4",
      videoFileSizeBytes: 1,
      ticketKind: "reward",
    });
    const rewardCancel = rewardRows.find((r) => r.label === "キャンセル・チケットの戻り")?.value;
    expect(rewardCancel).toContain("有効期限を過ぎていても1枚を出し直します");
    expect(rewardCancel).not.toContain("有効期限を過ぎた分は失効します");

    const normalRows = reviewConfirmRows({
      method: "tickets",
      priceYen: 0,
      hasVideo: true,
      question: "相談",
      eventLabel: "100m",
      videoFileName: "run.mp4",
      videoFileSizeBytes: 1,
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
        question: "相談",
        eventLabel: "100m",
        videoFileName: "run.mp4",
        videoFileSizeBytes: 1,
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

describe("reviewConfirmRows の申込内容の行（本体 #845）", () => {
  const base = { method: "card" as const, priceYen: 500, coachName: "テストコーチ" };
  const q = "  最初の行\n二行目  ";

  it("カード払い：種目・相談内容・動画が内容の直後に入り、相談内容だけ multiline", () => {
    setActiveLocale("ja");
    const rows = reviewConfirmRows({ ...base, hasVideo: true, question: q, eventLabel: "100m", videoFileName: "run.mp4", videoFileSizeBytes: 13107200 });
    expect(rows.map((r) => r.label)).toEqual(["内容", "種目", "相談内容", "動画", "回答期限", "総額", "お支払い方法", "キャンセル・返金"]);
    expect(rows[1].value).toBe("100m");
    expect(rows[2].value).toBe("最初の行\n二行目");
    expect(rows[3].value).toBe("run.mp4（12.5 MB）");
    expect(rows.filter((r) => r.multiline).map((r) => r.label)).toEqual(["相談内容"]);
    expect(rows.filter((r) => "multiline" in r).length).toBe(1);
  });

  it("動画なし：悩みのみ表記で動画行は なし", () => {
    setActiveLocale("ja");
    const rows = reviewConfirmRows({ ...base, hasVideo: false, question: "相談", eventLabel: "100m", videoFileName: null, videoFileSizeBytes: null });
    expect(rows[0].value).toContain("悩みのみ");
    expect(rows.find((r) => r.label === "動画")?.value).toBe("なし");
  });

  it.each([[500, "500 B"], [2048, "2 KB"], [1048576, "1.0 MB"], [1, "1 B"]])("サイズ整形 %i → %s", (size, text) => {
    setActiveLocale("ja");
    const rows = reviewConfirmRows({ ...base, hasVideo: true, question: "相談", eventLabel: "100m", videoFileName: "a.mp4", videoFileSizeBytes: size });
    expect(rows.find((r) => r.label === "動画")?.value).toBe(`a.mp4（${text}）`);
  });

  it.each(["normal", "reward"] as const)("チケット払い（%s）：先頭4行はカード払いと同じ形", (kind) => {
    setActiveLocale("ja");
    const rows = reviewConfirmRows({ method: "tickets", priceYen: 0, hasVideo: true, ticketKind: kind, question: "相談", eventLabel: "100m", videoFileName: "run.mp4", videoFileSizeBytes: 1 });
    expect(rows.slice(0, 5).map((r) => r.label)).toEqual(["内容", "種目", "相談内容", "動画", "回答期限"]);
    expect(rows.find((r) => r.label === "動画")?.value).toBe("run.mp4（1 B）");
  });

  it("PayPay でも同じ先頭4行", () => {
    setActiveLocale("ja");
    const rows = reviewConfirmRows({ method: "paypay", priceYen: 500, hasVideo: false, question: "相談", eventLabel: "100m", videoFileName: null, videoFileSizeBytes: null });
    expect(rows.slice(0, 5).map((r) => r.label)).toEqual(["内容", "種目", "相談内容", "動画", "回答期限"]);
  });

  it("英語：ラベルと動画の値だけ訳され、相談内容は原文のまま", () => {
    installEnglish(en);
    setActiveLocale("en");
    try {
      const withVideo = reviewConfirmRows({ ...base, hasVideo: true, question: "後半に失速します", eventLabel: "100m", videoFileName: "run.mp4", videoFileSizeBytes: 13107200 });
      expect(withVideo.slice(1, 4).map((r) => r.label)).toEqual(["Event ", "Your question", "Video"]);
      expect(withVideo[2].value).toBe("後半に失速します");
      expect(withVideo[3].value).toBe("run.mp4 (12.5 MB)");
      const without = reviewConfirmRows({ ...base, hasVideo: false, question: "x", eventLabel: "100m", videoFileName: null, videoFileSizeBytes: null });
      expect(without[3].value).toBe("None");
    } finally {
      setActiveLocale("ja");
    }
  });
});
