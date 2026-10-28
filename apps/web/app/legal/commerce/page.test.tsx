/** @vitest-environment jsdom */
import { readFileSync } from "node:fs";
import path from "node:path";
import { renderToStaticMarkup } from "react-dom/server";
import { expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import { showsBackButton } from "@/components/bottom-nav";
import { coachingEnabled } from "@/lib/features";
import CommercePage from "./page";
vi.mock("next/link", () => ({ default: ({ href, children, className }: { href: string; children: ReactNode; className?: string }) => <a href={href} className={className}>{children}</a> }));

it("コーチングのフラグが閉じていても表示し、住所・電話番号は請求時の開示にする", () => {
  expect(coachingEnabled).toBe(false);
  const html = renderToStaticMarkup(<CommercePage />);
  expect(html).toContain("特定商取引法に基づく表記");
  expect(html).toContain("中窪日向");
  expect(html).toContain("請求があれば遅滞なく開示します。");
  expect(html).toContain("mailto:tracktalk.office@gmail.com");
  expect(showsBackButton("/legal/commerce")).toBe(true);
});

it("公開文面に開発中の注記や販売終了したチケットを表示しない", () => {
  const html = renderToStaticMarkup(<CommercePage />);
  expect(html).not.toMatch(/たたき台|下書き|未確定|要確認|TODO|確定していません|確定後|チケット/);
  expect(html).toContain("1,500円（税込）");
  expect(html).toContain("600円（税込）");
  expect(html).toContain("クレジットカード・PayPay ともに申込みの時に決済が済みます");
  expect(html).toContain("決済完了後48時間以内に受諾しなかった場合");
  expect(html).toContain("受諾後72時間以内に回答しなかった場合");
  expect(html).not.toContain("添削・追加相談が返却された時に確定");
});

// 別言語の API 料金定数とのずれを検出する。決済実装自体は変更しない。
it("掲載料金を API の決済用定数と一致させる", () => {
  const api = path.resolve(process.cwd(), "../api/internal/httpx");
  const review = readFileSync(path.join(api, "review_payment.go"), "utf8");
  const consultation = readFileSync(path.join(api, "coaching_ticket.go"), "utf8");
  const html = renderToStaticMarkup(<CommercePage />);
  for (const [source, name] of [[review, "reviewPriceYen"], [consultation, "ticketFaceYen"]]) {
    const value = source.match(new RegExp(`const ${name} int32 = (\\d+)`));
    expect(value, name).not.toBeNull();
    expect(html).toContain(`${Number(value![1]).toLocaleString("ja-JP")}円（税込）`);
  }
});
