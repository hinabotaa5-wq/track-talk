/** @vitest-environment jsdom */
import { renderToStaticMarkup } from "react-dom/server";
import { expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import { SellerNote } from "./seller-note";
vi.mock("next/link", () => ({ default: ({ href, children, className }: { href: string; children: ReactNode; className?: string }) => <a href={href} className={className}>{children}</a> }));

it("フォーム添削・チケットは運営者が販売者で、特商法の表記へ案内する", () => {
  const html = renderToStaticMarkup(<SellerNote />);
  expect(html).toContain("販売者: TrackTalk の運営者");
  expect(html).toContain('href="/legal/commerce"');
});

it("サポートプランはコーチが販売者で、運営者の表記へは案内しない", () => {
  const html = renderToStaticMarkup(<SellerNote coachName="山田" />);
  expect(html).toContain("このプランを出しているコーチ（山田）");
  expect(html).not.toContain("/legal/commerce");
});
