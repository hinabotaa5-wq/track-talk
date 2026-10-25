import { createElement, type ReactNode } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it, vi } from "vitest";
import CoachingPage from "./page";

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
});
