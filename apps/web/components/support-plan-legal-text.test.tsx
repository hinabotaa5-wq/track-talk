import { renderToStaticMarkup } from "react-dom/server";
import { expect, it } from "vitest";
import { PrivacyPolicy } from "./privacy-policy";
import { SupportPlanCoachTerms } from "./support-plan-coach-terms";

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

it("コーチ向けの報酬と精算を指定の3段落で表示する", () => {
  const html = renderToStaticMarkup(<SupportPlanCoachTerms />);
  expect(html).toContain("1ヶ月プランの報酬と精算");
  expect(html).toContain("<p>報酬は経過分の75%です。月末までの分を翌月15日に仮払いし（3,000円未満は繰越し）、契約終了時に精算します。解約料はありません。</p>");
  expect(html).toContain("<p>コーチ都合で終了した場合は選手へ全額返金し、受け取った仮払いはその75%まで返還いただきます。やむを得ない事情の場合は通常の精算です。</p>");
  expect(html).toContain("<p>日割りの計算方法、異議申立て、契約ごとの条件などの詳細は、契約成立時の通知と利用規約をご確認ください。</p>");
  expect(html).not.toContain("報酬の確定時期と仮払い");
  expect(html).not.toContain("コーチ都合の精算");
  expect(html).not.toContain("やむを得ない事情と判定の手続");
});
