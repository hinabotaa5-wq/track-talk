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
  expect(text).toContain("Coach Japanese sprinters online — monthly packages and video reviews");
  expect(text).toContain("Package: athletes pay $139 (¥22,000) per month — you receive $104 (¥16,500) of that, per athlete per month.");
  expect(text).toContain("Reviews: around $12 per review.");
  expect(text).toContain("in Japan a single review costs the athlete ¥500 and the coach receives ¥350");
  expect(text).toContain("up to about 28 videos");
  expect(text).toContain("aiming for about 48 hours per reply");
  expect(text).toContain("revise it each week");
  expect(text).toContain("paid by the 15th of the following month (during the pilot we aim to send it around the 7th)");
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
