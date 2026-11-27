import { expect, it, vi } from "vitest";
vi.mock("./page-view", () => ({ ReferralGuideView: () => null }));
import { metadata } from "./page";
import { en as enDictionary } from "@/lib/i18n-en";

const description =
  "購入すると、紹介IDが発行されます。そのIDで友達2人が購入すると、紹介してくれた方に¥500クーポンがプレゼントされます。クーポンは申込みで使えます。有効期限は、プレゼントされてから6か月です。";
const oldDescription =
  "購入すると、紹介IDが発行されます。そのIDで友達2人が購入すると、紹介してくれた方に¥500クーポンがプレゼントされます。クーポンはスタンダードの申込みで使えます。有効期限は、プレゼントされてから6か月です。";

// #1062：ページ説明のクーポン文言からプラン名を外す（クーポンはスタンダード・プレミアム両方可）。
it("T1：metadata の description はプラン名なしで、title と他の文は従来どおり", () => {
  expect(metadata.title).toBe("2人に紹介すると500円分無料");
  expect(metadata.description).toBe(description);
  expect(metadata.description).not.toContain("スタンダードの申込みで使えます");
});

it("T2：英訳辞書は新キーだけを持ち、旧キーは残っていない", () => {
  expect(enDictionary[description]).toBe(
    "When you make a purchase, you get a referral code. When 2 friends buy with that code, the person who referred them gets a ¥500 coupon. The coupon can be used on a request and is valid for 6 months from when it is given.",
  );
  expect(enDictionary[oldDescription]).toBeUndefined();
});
