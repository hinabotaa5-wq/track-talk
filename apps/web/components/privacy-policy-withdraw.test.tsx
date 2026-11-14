import { renderToStaticMarkup } from "react-dom/server";
import { expect, it } from "vitest";
import { PrivacyPolicy } from "./privacy-policy";

// #717：進行中の契約などがあるあいだは退会できない（利用規約第15条・実装は退会を止める）ので、「提供中に退会すると削除」とは書かない。
it("プライバシーポリシー第10条：対応が終わるまでは退会できず、対応後の退会で指導の契約とやりとりを削除する", () => {
  const html = renderToStaticMarkup(<PrivacyPolicy />);
  expect(html).not.toContain("オンラインコーチングの提供中は、あなたの指導の契約とやりとり");
  expect(html).toContain("進行中の指導の契約、未使用・確保中のチケット、返金、まだ支払っていないコーチの報酬など、利用規約第15条に定める対応が終わるまでは、退会の手続きはできません。対応が終わったあとに退会すると、あなたの指導の契約とやりとり（文章と動画）も削除します。");
});
