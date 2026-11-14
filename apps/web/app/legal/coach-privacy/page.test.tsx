/** @vitest-environment jsdom */
import { renderToStaticMarkup } from "react-dom/server";
import { expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import { showsBackButton } from "@/components/bottom-nav";
import { PrivacyPolicy } from "@/components/privacy-policy";
import CoachPrivacyPage from "./page";
vi.mock("next/link", () => ({ default: ({ href, children, className }: { href: string; children: ReactNode; className?: string }) => <a href={href} className={className}>{children}</a> }));

it("海外コーチ向けの英語の通知：Wise と翻訳の委託、学習に使わないことと運営者の改善利用を別の文で書く", () => {
  const html = renderToStaticMarkup(<CoachPrivacyPage />);
  expect(html).toContain('lang="en"');
  expect(html).toContain("Privacy Notice for Overseas Coaches");
  expect(html).toContain("Wise Payments Japan (Japan): sending your fees.");
  expect(html).toContain("<p>Anthropic does not use the text we send to train its AI models.</p>");
  expect(html).toContain("we never use information about injuries or physical condition");
  expect(html).toContain("do not collect conversations automatically");
  expect(html).toContain('href="/privacy"');
  expect(html).not.toMatch(/[ぁ-んァ-ン一-龯]/);
  expect(showsBackButton("/legal/coach-privacy")).toBe(true);
});

it("プライバシーポリシー：翻訳と Wise を委託先に加え、改善利用は個人を特定できない形・要配慮情報なし・手動で、英語の通知へリンクする", () => {
  const html = renderToStaticMarkup(<PrivacyPolicy />);
  expect(html).toContain("<li>Anthropic は、送られた文章を AI の学習に使いません。</li>");
  expect(html).toContain("個人を特定できない形にして、翻訳の用語集と指示文の改善に使います");
  expect(html).toContain("ケガや体調についての情報は使いません");
  expect(html).toContain("やりとりを自動で集めることはしません");
  expect(html).toContain("Wise Payments Japan（日本）: 海外コーチへの委託費の送金。海外コーチとのやりとりを始めるまで使いません");
  expect(html).toContain("海外コーチとのやりとりの翻訳（第3条）。送った内容は、AI の学習には使われません。海外コーチとのやりとりを始めるまで使いません");
  expect(html).toContain('href="/legal/coach-privacy"');
});

it("プライバシーポリシー：フォーム添削は提供中、継続プランは準備中、海外コーチとのやりとりは未開始と読め、Stripe に開始前の断りがない", () => {
  const html = renderToStaticMarkup(<PrivacyPolicy />);
  expect(html).toContain("オンラインコーチングのうち、フォーム添削（単発・チケット）は提供しています。継続プランは準備中です。海外コーチとのやりとり（翻訳と委託費の送金）は、まだ始めていません。");
  expect(html).not.toContain("準備中のオンラインコーチング");
  expect(html).not.toContain("購入は受け付けていません");
  expect(html).toContain("Stripe（米国。米国ほか）: オンラインコーチングの支払いの処理と、コーチへの報酬の送金</li>");
});
