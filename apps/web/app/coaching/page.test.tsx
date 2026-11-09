import { createElement, type ReactNode } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import CoachingPage from "./page";

const flags = { coaching: false, list: false };
vi.mock("@/lib/features", async (original) => ({
  ...await original<typeof import("@/lib/features")>(),
  get coachingEnabled() { return flags.coaching; },
  get coachingCoachListEnabled() { return flags.list; },
}));
beforeEach(() => {
  flags.coaching = false;
  flags.list = false;
  vi.setSystemTime(new Date("2026-11-14T00:00:00+09:00"));
});
afterEach(() => vi.useRealTimers());

vi.mock("next/link", () => ({
  default: ({ children, href, ...props }: { children: ReactNode; href: string }) => createElement("a", { href, ...props }, children),
}));

describe("コーチタブ", () => {
  it.each([false, true])("開始直前は一覧フラグが %s でも、上部には開始案内だけを出す", (list) => {
    flags.coaching = true;
    flags.list = list;
    vi.setSystemTime(new Date("2026-11-13T23:59:59+09:00"));
    const html = renderToStaticMarkup(createElement(CoachingPage));
    expect(html.slice(0, html.indexOf("<nav"))).toContain("11/14(土)に開始します！");
    expect(html).not.toContain('id="coach-list-heading"');
    expect(html).not.toContain("登録しているコーチを見る");
  });

  it("一番上にコーチ一覧を準備中として出し、押しても先へ進まない", () => {
    const html = renderToStaticMarkup(createElement(CoachingPage));
    const listAt = html.indexOf("コーチ一覧");
    expect(listAt).toBeGreaterThan(-1);
    expect(listAt).toBeLessThan(html.indexOf("<nav"));
    const section = html.slice(html.lastIndexOf("<section", listAt), html.indexOf("</section>", listAt));
    expect(section).toContain("準備中です");
    expect(section).not.toContain("<a");
  });

  // 一番上の「コーチ一覧」の枠（準備中の section か、/coaches へのリンク）を取り出す。
  const topListBox = (html: string) => {
    const listAt = html.indexOf("コーチ一覧");
    const start = Math.max(html.lastIndexOf("<section", listAt), html.lastIndexOf("<a", listAt));
    const end = html.indexOf(html.startsWith("<a", start) ? "</a>" : "</section>", listAt);
    return html.slice(start, end);
  };
  for (const coaching of [false, true]) {
    it(`一覧を閉じている間は、コーチング${coaching ? "を開けていても" : "が閉じていても"}上の枠は準備中のままでリンクにしない`, () => {
      flags.coaching = coaching;
      const html = renderToStaticMarkup(createElement(CoachingPage));
      const box = topListBox(html);
      expect(box.startsWith("<section")).toBe(true);
      expect(box).toContain("準備中です");
      expect(box).not.toContain("<a");
      expect(box).not.toContain("登録しているコーチを見る");
    });
    it(`一覧を開けると、コーチング${coaching ? "を開けていても" : "が閉じていても"}上の枠は /coaches へのリンクになり、準備中と出さない`, () => {
      flags.coaching = coaching;
      flags.list = true;
      const html = renderToStaticMarkup(createElement(CoachingPage));
      const box = topListBox(html);
      expect(html).not.toContain("11/14(土)に開始します！");
      expect(html.indexOf("コーチ一覧")).toBeLessThan(html.indexOf("<nav"));
      expect(box.startsWith('<a href="/coaches"')).toBe(true);
      expect(box).toContain("登録しているコーチを見る");
      expect(box).not.toContain("準備中");
    });
  }

  // サポートプランの入口はコーチ一覧（/coaches）。一覧を閉じている間は、開いているように見せない。
  const supportCard = (html: string) => {
    const at = html.indexOf('href="/coaches?tab=support"', html.indexOf("<nav"));
    return html.slice(at, html.indexOf("</a>", at));
  };
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
  it("コーチングを開けると、利用中の方には残高・履歴ではなくコーチングチケットへの案内を出す", () => {
    flags.coaching = true;
    const html = renderToStaticMarkup(createElement(CoachingPage));
    expect(html).toContain("利用中の方");
    expect(html).toContain("コーチングチケット");
    expect(html).not.toContain("チケットの残高・履歴");
  });
});
