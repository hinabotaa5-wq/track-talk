import { readFileSync } from "node:fs";
import path from "node:path";
import { expect, it } from "vitest";
import { availableReviewPayMethods, reviewPayNotice } from "./review";

// 購入画面に出す支払い方法を、API が受け付ける方法（reviewPaymentMethods）と揃える。
it("動画添削・追加相談の支払い方法を API の定義と一致させる", () => {
  const source = readFileSync(path.resolve(process.cwd(), "../api/internal/httpx/coaching_payment.go"), "utf8");
  const match = source.match(/var reviewPaymentMethods = \[\]string\{([^}]*)\}/);
  expect(match).not.toBeNull();
  const api = [...match![1].matchAll(/"([a-z_]+)"/g)].map((m) => m[1]);
  expect(availableReviewPayMethods).toEqual(api);
});

it("PayPay が使えない間は、購入画面にカードのみの案内を出す", () => {
  expect(availableReviewPayMethods).toEqual(["card"]);
  expect(reviewPayNotice).toBe("現在、お支払いはクレジットカードのみとなります。PayPayでのお支払いにも対応予定です（時期は未定です）。対応が整い次第、この画面でご案内します。");
});
