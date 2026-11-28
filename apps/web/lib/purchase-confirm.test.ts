import { describe, expect, it } from "vitest";
import { reviewConfirmRows } from "./purchase-confirm";
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

  it("英語のカード払いの確認文が運営者決定の短縮文の訳と一致する", () => {
    installEnglish(en);
    setActiveLocale("en");
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
    } finally {
      setActiveLocale("ja");
    }
  });

  // 本体 #1031：クーポンを使うときは、価格・クーポン（−¥500）・お支払い額の内訳を見せる。
  it("クーポン適用の確認は、価格・クーポン・お支払い額の内訳を出す", () => {
    setActiveLocale("ja");
    const base = { hasVideo: true, question: "相談", eventLabel: "100m", videoFileName: "run.mp4", videoFileSizeBytes: 1 };
    const rows = reviewConfirmRows({ ...base, method: "card", priceYen: 1000, couponYen: 500 });
    const value = (label: string) => rows.find((r) => r.label === label)?.value;
    expect(value("価格")).toBe("1,000円（税込）");
    expect(value("クーポン")).toBe("−500円（¥500クーポン1枚）");
    expect(value("お支払い額")).toBe("500円（税込）");
    expect(value("総額")).toBeUndefined();
    expect(value("お支払い方法")).toBeDefined();
    expect(value("クーポンの戻り")).toContain("クーポンが戻ります");
    // ¥2,000 の申込みなら ¥1,500（プレミアム相当の価格でも同じ計算）。
    const premium = reviewConfirmRows({ ...base, method: "card", priceYen: 2000, couponYen: 500 });
    expect(premium.find((r) => r.label === "お支払い額")?.value).toBe("1,500円（税込）");
    // 価格が ¥500 なら支払いは0円で、支払い方法・カードの行は出さない。
    const free = reviewConfirmRows({ ...base, method: "card", priceYen: 500, couponYen: 500 });
    expect(free.find((r) => r.label === "お支払い額")?.value).toBe("0円（税込）");
    expect(free.find((r) => r.label === "お支払い方法")).toBeUndefined();
    expect(free.find((r) => r.label === "キャンセル・クーポンの戻り")).toBeDefined();
    // クーポンなしは従来どおり総額。
    const plain = reviewConfirmRows({ ...base, method: "card", priceYen: 1000 });
    expect(plain.find((r) => r.label === "総額")?.value).toBe("1,000円（税込）");
    expect(plain.find((r) => r.label === "クーポン")).toBeUndefined();
  });

  // 本体 #1085 T4：英語表示では金額だけをドル（¥160=$1）にする。各金額は円から独立に換算し、差額から逆算しない。
  // そのため ¥1,000 − ¥500 ＝ ¥500 は $6.25 − $3.13 ＝ $3.13 と1セント合わない表示になるが、仕様として許容する。
  it("英語表示のクーポン適用は、価格・クーポン・お支払い額をそれぞれドルで出す（1セント差は仕様）", () => {
    installEnglish(en);
    setActiveLocale("en");
    try {
      const base = { hasVideo: true, question: "相談", eventLabel: "100m", videoFileName: "run.mp4", videoFileSizeBytes: 1 };
      const rows = reviewConfirmRows({ ...base, method: "card", priceYen: 1000, couponYen: 500 });
      const value = (ja: string) => rows.find((r) => r.label === en[ja])?.value;
      expect(value("価格")).toBe(en["{v0}（税込）"].replace("{v0}", "$6.25"));
      expect(value("クーポン")).toBe(en["−{v0}（¥500クーポン1枚）"].replace("{v0}", "$3.13"));
      expect(value("クーポン")).toContain("¥500");
      expect(value("お支払い額")).toBe(en["{v0}（税込）"].replace("{v0}", "$3.13"));
      const plain = reviewConfirmRows({ ...base, method: "card", priceYen: 1000 });
      expect(plain.find((r) => r.label === en["総額"])?.value).toBe(en["{v0}（税込）"].replace("{v0}", "$6.25"));
      const amounts = [...rows, ...plain].map((r) => r.value).filter((v) => v.includes("$"));
      expect(amounts.every((v) => !v.includes("円"))).toBe(true);
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

  // 本体 #1013 補足 B4: 選手情報は入力がある項目だけ、相談内容の直後に行を足す。
  it("選手情報は入力がある項目だけ相談内容の直後に行を出す", () => {
    setActiveLocale("ja");
    const input = {
      method: "card" as const,
      priceYen: 1000,
      hasVideo: true,
      question: "相談",
      eventLabel: "100m",
      videoFileName: "run.mp4",
      videoFileSizeBytes: 1,
    };
    const labelsOf = (rows: ReturnType<typeof reviewConfirmRows>) => rows.map((r) => r.label);
    const none = labelsOf(reviewConfirmRows(input));
    for (const label of ["PB（自己ベスト）", "目標タイム", "風速", "その時の体のコンディション"]) expect(none).not.toContain(label);
    expect(labelsOf(reviewConfirmRows({ ...input, athleteInfo: { pb: " ", goal_time: "" } }))).toEqual(none);
    const rows = reviewConfirmRows({ ...input, athleteInfo: { pb: "11秒20", goal_time: "10秒台", wind_speed: "+1.2", body_condition: "脚が張っていた" } });
    const labels = labelsOf(rows);
    const at = labels.indexOf("相談内容");
    expect(labels.slice(at, at + 5)).toEqual(["相談内容", "PB（自己ベスト）", "目標タイム", "風速", "その時の体のコンディション"]);
    expect(rows.slice(at + 1, at + 5).map((r) => r.value)).toEqual(["11秒20", "10秒台", "+1.2", "脚が張っていた"]);
    const partial = labelsOf(reviewConfirmRows({ ...input, athleteInfo: { wind_speed: "+1.2" } }));
    expect(partial.slice(at, at + 2)).toEqual(["相談内容", "風速"]);
    expect(partial).toHaveLength(none.length + 1);
  });
});


describe("reviewConfirmRows の「得られるもの」行（本体 #1086 W4、#1085 のドル表示との合成）", () => {
  const base = { method: "card" as const, priceYen: 1000, couponYen: 500, hasVideo: true, question: "相談", eventLabel: "100m", videoFileName: "run.mp4", videoFileSizeBytes: 1 };
  const titles = ["良い点", "改善点", "おすすめのドリル", "コーチなら明日から何をするか", "スタート局面の詳しい分析"];

  it("offerTitles を渡したときだけ内容の直後に「、」で連結した行を出す", () => {
    setActiveLocale("ja");
    const rows = reviewConfirmRows({ ...base, offerTitles: titles });
    const labels = rows.map((r) => r.label);
    expect(labels.indexOf("得られるもの")).toBe(labels.indexOf("内容") + 1);
    expect(rows.find((r) => r.label === "得られるもの")?.value).toBe(titles.join("、"));
    expect(reviewConfirmRows(base).some((r) => r.label === "得られるもの")).toBe(false);
    expect(reviewConfirmRows({ ...base, offerTitles: [] }).some((r) => r.label === "得られるもの")).toBe(false);
  });

  it("英語表示ではラベルだけ訳し、追加項目は原文のまま。同じ確認の金額はドルのまま", () => {
    installEnglish(en);
    setActiveLocale("en");
    try {
      const rows = reviewConfirmRows({ ...base, offerTitles: titles });
      expect(rows.find((r) => r.label === en["得られるもの"])?.value).toContain("スタート局面の詳しい分析");
      expect(rows.find((r) => r.label === en["クーポン"])?.value).toBe(en["−{v0}（¥500クーポン1枚）"].replace("{v0}", "$3.13"));
      expect(rows.find((r) => r.label === en["お支払い額"])?.value).toBe(en["{v0}（税込）"].replace("{v0}", "$3.13"));
    } finally {
      setActiveLocale("ja");
    }
  });
});
