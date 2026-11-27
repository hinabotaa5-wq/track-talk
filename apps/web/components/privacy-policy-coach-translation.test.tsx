import { renderToStaticMarkup } from "react-dom/server";
import { expect, it } from "vitest";
import { PrivacyPolicy } from "./privacy-policy";

// 項目15: コーチプロフィール翻訳の委託先を固定文面で追加し、最終更新日を実装日にする。
it("コーチプロフィールの翻訳の委託先と最終更新日", () => {
  const html = renderToStaticMarkup(<PrivacyPolicy />);
  expect(html).toContain("Anthropic（Claude API。米国。米国）: コーチのプロフィールの英語表示のための翻訳。コーチが承認されたときと、承認後にプロフィールを変更したときに、名前・ひとこと・成績や指導経験・記録を送ります。送った内容は、AI の学習には使われません");
  expect(html).toContain("最終更新日: 2026年11月27日");
  // 既存の Anthropic 2件は不変。
  expect(html).toContain("感覚メモのふりかえりの作成");
  expect(html).toContain("海外コーチとのやりとりの翻訳（第3条）");
});

it("adds video translation as its own processor list item", () => {
  const html = renderToStaticMarkup(<PrivacyPolicy />);
  expect(html).toContain("<li>Anthropic（Claude API。米国。米国）: フィードの動画投稿の英語表示のための翻訳。動画投稿が作成・編集されたときに、タイトルと本文を送ります。送った内容は、AI の学習には使われません</li>");
});
