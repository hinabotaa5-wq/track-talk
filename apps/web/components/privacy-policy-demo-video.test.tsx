import { renderToStaticMarkup } from "react-dom/server";
import { expect, it } from "vitest";
import { documentConsentVersions } from "@/lib/document-consent";
import { PrivacyPolicy } from "./privacy-policy";

// 本体 #1091 T4・T6：7日での削除の対象は回答に付けた見本動画だけ。コーチのページの見本動画（#1086）の保管を切り分けて書く。
it("T4: 7日での削除は回答に付けた見本動画で、コーチのページの見本動画は掲載中は保管・掲載終了後は従来の扱い・退会時は削除", () => {
  const html = renderToStaticMarkup(<PrivacyPolicy />);
  expect(html).toContain("の動画のファイル（依頼の動画、コーチが回答に付けた見本動画、相談の質問と回答の動画）は、そのやりとりが終わってから7日で削除します。");
  expect(html).not.toContain("依頼の動画、コーチの見本動画、");
  expect(html).toContain("コーチのページに表示する見本動画（コーチ本人が掲載のためにアップロードし、コーチの申請と運営者の承認を経たもの）は、上の7日での削除の対象ではありません。");
  expect(html).toContain("コーチのページに掲載している間（掲載のための申請が承認待ち・切り替え待ちの間を含みます）は保管し、表示のために配信します。");
  expect(html).toContain("掲載をやめた動画は、従来の動画のファイルの扱いに戻り、どこにも使われていないファイルは削除します。コーチが退会した場合は削除します。");
  expect(html).toContain("回答に付けた見本動画として使われている動画は、その回答の動画の扱いに従います。");
  // 紹介に使う情報と退会時に削除するものにも見本動画を含める。
  expect(html).toContain("ひとこと、見本動画（コーチ本人が掲載のためにアップロードし、コーチの申請と運営者の承認を経たもの）を表示します。");
  expect(html).toContain("コーチの応募と顔写真、コーチのページの見本動画（回答に付けた見本動画として使われているものを除きます）、");
});

it("T6: 最終更新日は版（privacy-2026-11-27）の日付", () => {
  const html = renderToStaticMarkup(<PrivacyPolicy />);
  expect(documentConsentVersions.privacy).toBe("privacy-2026-11-27");
  expect(html).toContain("最終更新日: 2026年11月27日");
});
