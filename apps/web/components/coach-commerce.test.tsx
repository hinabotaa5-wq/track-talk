/** @vitest-environment jsdom */
import { renderToStaticMarkup } from "react-dom/server";
import { expect, it } from "vitest";
import { CoachCommerceDisclosure } from "./coach-commerce";

it("表記要のコーチは、氏名・住所・電話番号をプランのページに出す", () => {
  const html = renderToStaticMarkup(<CoachCommerceDisclosure commerce={{ required: true, registered: true, legal_name: "山田太郎", address: "東京都千代田区", phone: "090-1234-5678" }} />);
  expect(html).toContain("山田太郎");
  expect(html).toContain("東京都千代田区");
  expect(html).toContain("090-1234-5678");
});

it("表記不要と申告したコーチは、表記がないことだけを伝える", () => {
  const html = renderToStaticMarkup(<CoachCommerceDisclosure commerce={{ required: false }} />);
  expect(html).toContain("表記はありません");
  expect(html).not.toContain("住所");
});
