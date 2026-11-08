/** @vitest-environment jsdom */
import type { ReactNode } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { expect, it, vi } from "vitest";
import CoachPayoutGuidePage from "./page";

vi.mock("next/link", () => ({ default: ({ href, children, className }: { href: string; children: ReactNode; className?: string }) => <a href={href} className={className}>{children}</a> }));

it("考えずに入力できる形で業種・URL・商品説明を載せる", () => {
  const html = renderToStaticMarkup(<CoachPayoutGuidePage />);
  expect(html).toContain("個人事業主");
  expect(html).toContain("教育 ＞ その他の教育サービス");
  expect(html).toContain("https://track-talk.netlify.app");
  expect(html).toContain("陸上競技の動画添削サービスです。");
});

it("完了確認と真っ白時の対処・単体URLでの案内を含む", () => {
  const html = renderToStaticMarkup(<CoachPayoutGuidePage />);
  expect(html).toContain("登録済み");
  expect(html).toContain("アプリを完全に閉じて");
  expect(html).toContain('href="/coach"');
  expect(html).toContain('href="/contact"');
  expect(html).not.toMatch(/TODO|要確認|未確定/);
});
