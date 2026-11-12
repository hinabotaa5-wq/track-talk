/** @vitest-environment jsdom */
import { renderToStaticMarkup } from "react-dom/server";
import { expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import { SellerNote } from "./seller-note";
vi.mock("next/link", () => ({ default: ({ href, children, className }: { href: string; children: ReactNode; className?: string }) => <a href={href} className={className}>{children}</a> }));

it("フォーム添削・チケットは販売者1行と利用規約・特商法へのリンクだけを出す", () => {
  const html = renderToStaticMarkup(<SellerNote />);
  expect(html).toContain("販売者: 中窪日向（TrackTalk運営者）");
  expect(html).toContain('href="/terms"');
  expect(html).toContain('href="/legal/commerce"');
  // 詳細は特商法ページ側に寄せるため、申込画面では2段落の説明を出さない。
  for (const word of ["提供者は運営者", "運営者がコーチに委託", "お支払い先は運営者", "料金・キャンセル・返金は運営者が責任を持って対応", "収納代行", "代理受領", "預り金", "預かり"]) expect(html).not.toContain(word);
});

it("サポートプランはコーチが販売者で、代理受領と二重請求がないことを示し、運営者の表記へは案内しない", () => {
  const html = renderToStaticMarkup(<SellerNote coachName="山田" />);
  expect(html).toContain("このプランを出しているコーチ（山田）");
  expect(html).toContain("契約の相手はコーチ");
  expect(html).toContain("代金を請求する権利はコーチ");
  expect(html).toContain("代理受領の権限");
  expect(html).toContain("運営者へ支払った時点でコーチへの支払いは済み");
  expect(html).toContain("重ねて請求されることはありません");
  expect(html).not.toContain("/legal/commerce");
});
