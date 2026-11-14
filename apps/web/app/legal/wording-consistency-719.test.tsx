/** @vitest-environment jsdom */
import { renderToStaticMarkup } from "react-dom/server";
import { expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import { PrivacyPolicy } from "@/components/privacy-policy";
import TermsPage from "@/app/terms/page";
import CommercePage from "@/app/legal/commerce/page";
import CoachPrivacyPage from "@/app/legal/coach-privacy/page";
import { GuideView } from "@/app/guide/page-view";
import { GuidelinesView } from "@/app/guidelines/page-view";
import { CoachAboutView } from "@/app/coach/about/page-view";
vi.mock("next/link", () => ({ default: ({ href, children, className }: { href: string; children: ReactNode; className?: string }) => <a href={href} className={className}>{children}</a> }));
vi.mock("next/navigation", () => ({ useRouter: () => ({ push: vi.fn(), replace: vi.fn() }), usePathname: () => "/", useSearchParams: () => new URLSearchParams() }));

// 本体 #719：規約類の小さな表記の不整合（当社・SNS欄・ひとこと・追加相談の旧称）。

function article(html: string, n: number): string {
  const start = html.indexOf(`第${n}条（`);
  const end = html.indexOf(`第${n + 1}条（`);
  expect(start).toBeGreaterThan(-1);
  return html.slice(start, end === -1 ? undefined : end);
}

it("運営者は個人なので、利用規約・特商法・プライバシーポリシーに「当社」を使わない", () => {
  for (const html of [renderToStaticMarkup(<TermsPage />), renderToStaticMarkup(<CommercePage />), renderToStaticMarkup(<PrivacyPolicy />)]) {
    expect(html).not.toContain("当社");
  }
  const terms = article(renderToStaticMarkup(<TermsPage />), 6);
  expect(terms).toContain("運営者またはコーチの責めに帰すべき事由");
  expect(terms).toContain("または運営者の都合により提供が著しく遅れた場合");
});

it("SNS欄は、アカウント名またはURLを1〜3件（各300文字以内）と、規約類で同じ書き方にする", () => {
  const terms = article(renderToStaticMarkup(<TermsPage />), 5);
  expect(terms).toContain("SNSのアカウント名またはSNS・所属チームのホームページのURLが必要です。SNS・ホームページ欄は1件以上、最大3件（各300文字以内）です。");
  expect(terms).not.toContain("URLは1件以上");
  expect(article(renderToStaticMarkup(<PrivacyPolicy />), 2)).toContain("SNSのアカウント名またはSNS・所属チームのホームページのURL（1〜3件、各300文字以内、必須）");
  expect(renderToStaticMarkup(<GuideView />)).toContain("SNSのアカウント名またはSNS・所属チームのホームページのURLが必要です。SNS・ホームページ欄は1件以上、最大3件（各300文字以内）です。");
  expect(renderToStaticMarkup(<CoachAboutView />)).toContain("SNSのアカウント名またはSNS・所属チームのホームページのURLを入力します。SNS・ホームページ欄は1〜3件必須（各300文字以内）です。");
  expect(renderToStaticMarkup(<GuidelinesView />)).toContain("確認できるSNSのアカウント名またはSNS・所属チームのホームページのURL（1〜3件）を提出してください。");
  expect(renderToStaticMarkup(<CoachPrivacyPage />)).toContain("1 to 3 entries for your social media account names or the URLs of your social media or team website (up to 300 characters each)");
});

it("コーチ紹介に表示する「ひとこと」を、利用規約第5条と英語の Coach Privacy にも載せる", () => {
  expect(article(renderToStaticMarkup(<TermsPage />), 5)).toContain("名前・担当種目・記録・成績や指導経験・顔写真・SNS・ホームページ欄の内容・ひとことを使用します。");
  expect(article(renderToStaticMarkup(<PrivacyPolicy />), 6)).toContain("ひとこと");
  expect(renderToStaticMarkup(<CoachPrivacyPage />)).toContain("and your profile comment (the short message to athletes).");
});

it("旧商品の追加相談の経過措置と、現行の追加質問・継続プランの追加相談を文面で区別する", () => {
  const terms = renderToStaticMarkup(<TermsPage />);
  expect(article(terms, 6)).toContain("販売を終了した旧商品の「追加相談」を、その販売の終了前に購入していた場合は、購入時の条件のとおり扱います（旧商品の「追加相談」は、この項の追加質問とは別の商品です）。");
  const art8 = article(terms, 8);
  expect(art8).toContain("1ヶ月プラン（7の2に定めるもの）以外の継続プランについてコーチが料金を設定した場合の追加相談（以下この条で「継続プランの追加相談」といい、第6条の1回プランの追加質問とは別のものです。");
  // 1ヶ月プランには追加相談がない（本体 #719 運営者決定 2026-11-15）。
  expect(art8).toContain("1ヶ月プランには追加相談はありません。期間中の動画への返答や相談は、プランの料金に含まれます。");
  expect(art8).not.toContain("継続プランの契約に付随する追加相談");
  expect(art8).toContain("継続プランの追加相談の料金はコーチが決め");
  const art3 = article(renderToStaticMarkup(<PrivacyPolicy />), 3);
  expect(art3).toContain("1回プランとその追加質問（販売を終了した旧商品の「追加相談」を含みます。以下この段落で同じです）の動画のファイル");
  expect(art3).toContain("追加質問のやりとりが終わる時は");
  expect(art3).toContain("継続プランの契約に付随する追加相談（1回プランの追加質問とは別のものです）の動画のファイル");
  expect(art3).not.toContain("フォーム添削とその追加相談");
});
