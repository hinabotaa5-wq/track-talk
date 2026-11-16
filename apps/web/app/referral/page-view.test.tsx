/** @vitest-environment jsdom */
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it } from "vitest";
import { installEnglish, setActiveLocale } from "@/lib/i18n";
import { en } from "@/lib/i18n-en";
import { ReferralGuideView } from "./page-view";

// 本体 #823 の説明ページの描画テスト。未ログイン前提の静的ページとして固定する。
let root: Root;
let container: HTMLDivElement;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  container = document.createElement("div"); document.body.appendChild(container); root = createRoot(container);
});
afterEach(async () => { await act(async () => root.unmount()); container.remove(); setActiveLocale("ja"); });

const JA_TEXTS = [
  "友達2人に紹介すると、1回無料！",
  "購入すると、紹介IDが発行されます。そのIDで友達2人が購入すると、紹介してくれた方に無料チケットがプレゼントされます。",
  "無料チケットは動画添削1回分です。有効期限は、プレゼントされてから6か月です。",
  "IDを使って購入する流れ",
  "友達から紹介IDを受け取る",
  "友達から6文字の紹介IDを教えてもらいます。",
  "コーチを選んで、申し込みに進む",
  "1回プランの都度払いでも、チケット6枚パックの購入でも使えます。",
  "購入の手続きで、紹介IDを入力する",
  "「紹介ID（友達からもらった場合）」の欄に、受け取ったIDを入力します。入力しないと、紹介として数えられません。",
  "そのまま購入を完了する",
  "購入が完了すると、紹介してくれた友達に「1人分」として数えられます。",
  "紹介する側の流れ",
  "紹介IDを使わずに購入すると、あなたの紹介IDが発行されます",
  "購入完了の画面とマイページに表示されます。",
  "紹介IDを友達に伝える",
  "マイページで紹介IDをコピーできます。",
  "友達2人が購入すると、無料チケットをプレゼント",
  "あと何人かは、マイページで確認できます。",
  "知っておいてほしいこと",
  "紹介IDの入力は任意です。空欄でも購入できます。",
  "使えない紹介IDを入力しても、購入は止まりません（紹介として数えられないだけです）。",
  "自分の紹介IDは、自分の購入には使えません。",
  "同じ友達は、同じ紹介者には1回だけ数えられます。",
  "対象は1回プランの都度払いとチケット6枚パックです。追加質問は対象外です。",
  "無料チケットは、もらった本人だけが使えます。払戻しはできません。",
  "あなたの紹介IDと進捗を見る",
  "コーチ一覧へ",
];

it("日本語で見出し・導入・全ステップ・注意書き・末尾リンクを表示する", async () => {
  setActiveLocale("ja");
  await act(async () => root.render(<ReferralGuideView />));
  const text = container.textContent ?? "";
  for (const expected of JA_TEXTS) expect(text).toContain(expected);
  expect(container.querySelector("h1")?.textContent).toBe("友達2人に紹介すると、1回無料！");
  expect(container.querySelectorAll("ol > li")).toHaveLength(7);
  expect(container.querySelector('a[href="/me"]')).not.toBeNull();
  expect(container.querySelector('a[href="/coaches"]')).not.toBeNull();
});

it("ステップ3の見本を表示し、実際の input 要素はない", async () => {
  setActiveLocale("ja");
  await act(async () => root.render(<ReferralGuideView />));
  const text = container.textContent ?? "";
  expect(text).toContain("紹介ID（友達からもらった場合）");
  expect(text).toContain("K7M3QX");
  expect(container.querySelector("input")).toBeNull();
});

it("英語表示では固定の英訳が出て、新規文の日本語が残らない", async () => {
  installEnglish(en);
  setActiveLocale("en");
  await act(async () => root.render(<ReferralGuideView />));
  const text = container.textContent ?? "";
  expect(text).toContain("How to purchase with a referral code");
  expect(text).toContain("The free ticket is good for one video review and expires 6 months after it is granted.");
  expect(text).toContain("Type the code into the “Referral code (from a friend)” field. If you skip it, the purchase will not count as a referral.");
  expect(text).toContain("How referring works");
  expect(text).toContain("Purchase without a referral code to get your own code");
  expect(text).toContain("Good to know");
  expect(text).toContain("See your referral code and progress");
  expect(text).toContain("Browse coaches");
  for (const ja of JA_TEXTS.slice(2)) expect(text).not.toContain(ja);
});
