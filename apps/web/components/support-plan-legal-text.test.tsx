import { renderToStaticMarkup } from "react-dom/server";
import { expect, it } from "vitest";
import { PrivacyPolicy } from "./privacy-policy";
import { SupportPlanCoachTerms, supportPlanCoachTerms } from "./support-plan-coach-terms";

it("プライバシーポリシー：終了時点で見られない・終わるまでにダウンロード・動画は添削7日／プラン30日・文章は1年・通報や紛争中は保全・開示は第11条", () => {
  const html = renderToStaticMarkup(<PrivacyPolicy />);
  expect(html).not.toContain("90日間は");
  expect(html).toContain("契約が終わった時点（期間の満了または途中での終了）で、選手・コーチ（18歳未満の選手の場合は保護者を含みます）のいずれからも画面で見られなくなります");
  expect(html).toContain("契約が終わるまでのあいだ、選手とコーチは文章とメニューをダウンロードできます");
  expect(html).toContain("やりとりの動画のファイルは契約が終わってから30日で削除し");
  expect(html).toContain("文章とメニューは指導の期間の終わりから1年で削除するまで保管します");
  expect(html).toContain("フォーム添削とその追加相談の動画のファイル（依頼の動画、コーチの見本動画、相談の質問と回答の動画）は、そのやりとりが終わってから7日で削除します");
  expect(html).toContain("確認を求めなかった場合は、確認を求められる返却から7日が過ぎた時");
  expect(html).toContain("閲覧の停止と削除をせずに保管します");
  expect(html).toContain("第11条の手続きで受け付けます");
});

it("コーチ向け条件：1ヶ月プランの報酬と精算の5項目を出す（中止延期は特別扱いせず期間どおり続く）", () => {
  expect(supportPlanCoachTerms.map((t) => t.title)).toEqual(["報酬の確定時期と仮払い", "コーチ都合の精算", "やむを得ない事情と判定の手続", "大会の中止・延期", "契約ごとの取引条件"]);
  const html = renderToStaticMarkup(<SupportPlanCoachTerms />);
  expect(html).toContain("1ヶ月プランの報酬と精算");
  expect(html).not.toContain("大会までプラン");
  expect(html).toContain("経過分（契約価格から選手への日割りの返金額を差し引いた額）の75%（1円未満切り捨て）で確定します");
  expect(html).toContain("両端を含む30日");
  expect(html).toContain("これは債務不履行に基づく返還・精算で");
  expect(html).toContain("1回に限り、期限なく異議を申し立てて再審査を求められます");
  expect(html).toContain("選手への全額返金はそのままにし、あなたには終了日までの経過分を支払います");
  expect(html).toContain("選手の目標の大会が中止・延期になっても、契約は期間どおり続きます");
  expect(html).toContain("フリーランス・事業者間取引適正化等法第3条に対応");
});
