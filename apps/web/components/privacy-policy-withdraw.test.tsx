import { renderToStaticMarkup } from "react-dom/server";
import { expect, it } from "vitest";
import { PrivacyPolicy } from "./privacy-policy";

// #717：進行中の契約などがあるあいだは退会できない（利用規約第15条・実装は退会を止める）ので、「提供中に退会すると削除」とは書かない。
it("プライバシーポリシー第10条：対応が終わるまでは退会できず、対応後の退会で指導の契約とやりとりを削除する", () => {
  const html = renderToStaticMarkup(<PrivacyPolicy />);
  expect(html).not.toContain("オンラインコーチングの提供中は、あなたの指導の契約とやりとり");
  expect(html).toContain("進行中の指導の契約、未使用・確保中のチケット、返金、まだ支払っていないコーチの報酬など、利用規約第15条に定める対応が終わるまでは、退会の手続きはできません。対応が終わったあとに退会すると、あなたの指導の契約とやりとり（文章と動画）も削除します。");
});

it("プライバシーポリシー第3条：単発・チケットと追加質問の購入前チェックを同じ扱いで記録する", () => {
  const html = renderToStaticMarkup(<PrivacyPolicy />);
  expect(html).toContain("1回プラン（単発・チケット）とその追加質問の購入では、年齢にかかわらず、購入の都度、決済に進む前に「未成年の方は保護者に許可を取ってください」の注意事項へのチェックを求め、アカウント、対象の購入の種別とID、チェックを受け付けた日時、確認した文面の版を記録します。");
  expect(html).not.toContain("1回プラン（単発・チケット）の購入では");
});

it("第7条の国外提供への同意案内に、退会手続きを完了できない場合の窓口を示す", () => {
  const html = renderToStaticMarkup(<PrivacyPolicy />);
  const start = html.indexOf("第7条（委託と国外での取り扱い）");
  const end = html.indexOf("第8条（第三者への提供）");
  expect(start).toBeGreaterThanOrEqual(0);
  expect(end).toBeGreaterThan(start);
  const article = html.slice(start, end);
  expect(article).toContain("個人情報の提供に同意しない場合");
  expect(article).toContain("第10条に定める事情により退会手続きを完了できないとき");
  expect(article).toContain('href="/contact"');
  expect(article).toContain("マイページ「お問い合わせ」からご連絡ください。運営者が個別に対応します。");
});
