import { renderToStaticMarkup } from "react-dom/server";
import { expect, it } from "vitest";
import { PrivacyPolicy } from "./privacy-policy";
import { SupportPlanCoachTerms, supportPlanCoachTerms } from "./support-plan-coach-terms";

it("プライバシーポリシー：終了後90日は閲覧のみ・ダウンロード・1年で削除・通報や紛争中は保全・開示は第11条", () => {
  const html = renderToStaticMarkup(<PrivacyPolicy />);
  expect(html).toContain("契約が終わった後（期間の満了または途中での終了）90日間は");
  expect(html).toContain("選手とコーチは文章とメニューをダウンロードできます");
  expect(html).toContain("指導の期間の終わりから1年で削除するまで保管します");
  expect(html).toContain("閲覧の停止と削除をせずに保管します");
  expect(html).toContain("第11条の手続きで受け付けます");
});

it("コーチ向け条件：大会までプランの報酬と精算の4項目を出す", () => {
  expect(supportPlanCoachTerms.map((t) => t.title)).toEqual(["報酬の確定時期と仮払い", "コーチ都合の精算", "やむを得ない事情と判定の手続", "大会の中止・延期"]);
  const html = renderToStaticMarkup(<SupportPlanCoachTerms />);
  expect(html).toContain("大会までプランの報酬と精算");
  expect(html).toContain("1回に限り異議を申し立てて再審査を求められます");
});
