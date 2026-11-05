/** @vitest-environment jsdom */
import { renderToStaticMarkup } from "react-dom/server";
import { beforeEach, expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import { showsBackButton } from "@/components/bottom-nav";
import PlatformMeasuresPage from "./page";

const flags = { coaching: false, messaging: false };
vi.mock("@/lib/features", async (original) => ({
  ...await original<typeof import("@/lib/features")>(),
  get coachingEnabled() { return flags.coaching; },
  get coachingMessagingEnabled() { return flags.messaging; },
}));
vi.mock("next/link", () => ({ default: ({ href, children }: { href: string; children: ReactNode }) => <a href={href}>{children}</a> }));
vi.mock("next/navigation", () => ({ notFound: () => { throw new Error("NEXT_NOT_FOUND"); } }));

beforeEach(() => {
  flags.coaching = false;
  flags.messaging = false;
});

it.each([
  [false, false],
  [true, false],
  [false, true],
])("サポートプランの提供かメッセージの送信が閉じている間は 404 にする（coaching=%s, messaging=%s）", (coaching, messaging) => {
  flags.coaching = coaching;
  flags.messaging = messaging;
  expect(() => renderToStaticMarkup(<PlatformMeasuresPage />)).toThrow("NEXT_NOT_FOUND");
});

it("両方が開いたら、承認済みの文面をそのまま表示する", () => {
  flags.coaching = true;
  flags.messaging = true;
  const html = renderToStaticMarkup(<PlatformMeasuresPage />);
  expect(html).toContain("取引デジタルプラットフォーム消費者保護法に基づく措置について");
  expect(html).toContain("TrackTalkのサポートプランでは、出品者であるコーチ（販売者）と購入者が取引を行い、運営者は、取引の場の提供と、代金の代理受領を行っています。運営者は、取引デジタルプラットフォーム消費者保護法第3条に基づき、次の措置を講じています。");
  expect(html).toContain("1. 円滑な連絡手段の確保");
  expect(html).toContain("購入者とコーチは、サービス内のメッセージ機能を使って連絡を取り合うことができます。また、運営者へのお問い合わせ窓口を設けています。");
  expect(html).toContain("2. 苦情への対応");
  expect(html).toContain("購入者またはコーチから苦情や相談があった場合は、運営者が事実関係の確認など必要な調査を行い、適切に対応します。");
  expect(html).toContain("3. 販売者の特定に必要な情報の確保と開示への対応");
  expect(html).toContain("コーチには、プランの提供を始める前に、氏名・住所・電話番号の登録を求め、運営者が保管しています。購入者から、法令に基づいて販売者に関する情報の開示を求められた場合は、お問い合わせ窓口を通じて受け付け、法令に従って対応します。");
  expect(html).toContain('href="/contact"');
  expect(html).not.toMatch(/たたき台|下書き|未確定|要確認|TODO/);
  expect(showsBackButton("/legal/platform")).toBe(true);
});
