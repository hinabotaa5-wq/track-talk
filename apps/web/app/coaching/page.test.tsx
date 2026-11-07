import { createElement, type ReactNode } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { beforeEach, describe, expect, it, vi } from "vitest";
import CoachingPage from "./page";

const flags = { coaching: false, list: false };
vi.mock("@/lib/features", async (original) => ({
  ...await original<typeof import("@/lib/features")>(),
  get coachingEnabled() { return flags.coaching; },
  get coachingCoachListEnabled() { return flags.list; },
}));
beforeEach(() => { flags.coaching = false; flags.list = false; });

vi.mock("next/link", () => ({
  default: ({ children, href, ...props }: { children: ReactNode; href: string }) => createElement("a", { href, ...props }, children),
}));

describe("コーチタブ", () => {
  it("一番上にコーチ一覧を準備中として出し、押しても先へ進まない", () => {
    const html = renderToStaticMarkup(createElement(CoachingPage));
    const listAt = html.indexOf("コーチ一覧");
    expect(listAt).toBeGreaterThan(-1);
    expect(listAt).toBeLessThan(html.indexOf("<nav"));
    const section = html.slice(html.lastIndexOf("<section", listAt), html.indexOf("</section>", listAt));
    expect(section).toContain("準備中です");
    expect(section).not.toContain("<a");
  });

  // サポートプランの入口はコーチ一覧（/coaches）。一覧を閉じている間は、開いているように見せない。
  const supportCard = (html: string) => html.slice(html.indexOf('href="/coaches"'), html.indexOf("</a>", html.indexOf('href="/coaches"')));
  it("コーチングを開けて一覧を閉じている間は、サポートプランを準備中と案内し、動画添削は案内どおり出す", () => {
    flags.coaching = true;
    const html = renderToStaticMarkup(createElement(CoachingPage));
    expect(supportCard(html)).toContain("準備中・公開までお待ちください");
    expect(html).toContain("コーチを選んで動画を送る");
  });
  it("コーチングと一覧の両方を開けると、サポートプランの説明を出す", () => {
    flags.coaching = true;
    flags.list = true;
    expect(supportCard(renderToStaticMarkup(createElement(CoachingPage)))).toContain("大会サポート・対面指導を選ぶ");
  });
});
