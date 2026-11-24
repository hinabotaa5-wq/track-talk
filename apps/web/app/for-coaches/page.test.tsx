/** @vitest-environment jsdom */
import { renderToStaticMarkup } from "react-dom/server";
import { expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import ForCoachesPage, { metadata } from "./page";
vi.mock("next/link", () => ({ default: ({ href, children, className }: { href: string; children: ReactNode; className?: string }) => <a href={href} className={className}>{children}</a> }));

it("海外コーチ向けの英語の紹介ページ：確定原稿の金額・条件を示し、Coach Terms とコーチ向けプライバシー通知へリンクする", () => {
  const html = renderToStaticMarkup(<ForCoachesPage />);
  const text = html.replace(/<[^>]*>/g, "").replace(/&#x27;|&apos;/g, "'");
  expect(html).toContain('lang="en"');
  expect(text).toContain("Coach Japanese sprinters online — video reviews and premium assessments");
  // 海外コーチの新方針（#1007 運営者決定 2026-11-24）：スタンダード（単発）¥10,000／$50 とプレミアム ¥20,000／$100。コーチ受取はドル固定。
  expect(text).toContain("Premium: athletes pay $127 (¥20,000) per package — you receive $100 of that.");
  expect(text).toContain("Reviews: athletes pay $65 (¥10,000) per review — you receive $50 of that.");
  expect(text).toContain("in Japan a single review costs the athlete ¥1,000 and the coach receives ¥700");
  expect(text).toContain("in two formats — a single video review, or a premium assessment across up to 10 videos.");
  expect(text).toContain("2. Premium package (the deeper option)");
  expect(text).toContain("The athlete sends up to 10 videos at once, numbered so you can refer to them easily. You watch all of them and reply with one overall assessment — not a separate review of each video.");
  expect(text).toContain("a cue and drills for each, and the direction for the athlete's next training block.");
  // 月額パッケージの旧文言は残さない（C1）。
  for (const old of ["$380", "¥60,000", "$300", "20 videos", "one-month", "monthly packages", "36 hours", "revise it each week", "across a full month"]) expect(text).not.toContain(old);
  expect(text).not.toContain("no set cap");
  expect(text).not.toContain("28 videos");
  // 旧フォローアップ $12 は出さない（プレミアムの $127 は別物なので除く）。
  expect(text).not.toMatch(/\$12(?!7)/);
  // 海外の単発には有料フォローアップを付けない（#718 運営者決定 2026-11-14）。Coach Terms 第1条・第2条と一致させる。
  expect(text).toContain("A single review has no paid follow-up add-on; an athlete who wants a deeper look across several videos books the premium package.");
  expect(text.indexOf("1. Single video review (the entry point)")).toBeLessThan(text.indexOf("2. Premium package (the deeper option)"));
  expect(text).not.toContain("is available as an add-on");
  expect(text).toContain("paid by the 15th of the following month (during the pilot we aim to send it around the 7th)");
  expect(text).toContain("Both fees are set in US dollars, so the amount you receive does not move with the exchange rate.");
  expect(text).not.toContain("The package fee is set in yen");
  expect(text).toContain("There is no minimum payout — everything you earn is paid out each month, and we cover the Wise transfer fees.");
  expect(text).toContain("Paid pilot");
  expect(text).toContain("We start small with a paid pilot, run manually with a few athletes.");
  expect(text).not.toMatch(/November|December/i);
  expect(html).toContain('href="/legal/coach-terms"');
  expect(html).toContain('href="/legal/coach-privacy"');
  expect(html).toContain('href="mailto:tracktalk.office@gmail.com"');
  expect(text).toContain("— TrackTalk (operated by Hinata, Japan)");
  // 英語固定のページ（日本語の文は出さない）。
  expect(text).not.toMatch(/[ぁ-んァ-ン一-龯]/);
  expect(metadata.robots).toEqual({ index: false });
});
