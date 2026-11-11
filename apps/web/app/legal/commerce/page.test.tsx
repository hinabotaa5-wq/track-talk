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
  // 支払方法の欄は今使える方法だけ（PayPay は載せず、対応予定の案内も入れない）。
  expect(html).toContain("<p>クレジットカード（Apple Pay・Google Pay を含む）。1回ごとのお支払いです。</p>");
  expect(html).not.toContain("、PayPay。");
  expect(html).not.toContain("対応予定");
  // 支払時期・キャンセル・返金の欄にも PayPay の条件文を残さない（支払方法がカードのみのため）。
  expect(html).not.toContain("PayPay");
  expect(html).toContain("フォーム添削の追加相談は、申込みの時に決済が済みます。");
  expect(html).toContain("クレジットカードは代金の確保を取り消します。");
  expect(html).toContain("申込みを終了し、全額を返金します。");
  expect(html).toContain("決済完了後48時間以内に受諾しなかった場合");
  expect(html).toContain("受諾後72時間以内に回答しなかった場合");
  expect(html).not.toContain("添削・追加相談が返却された時に確定");
  // 内容確認のための差し戻しの例外（再提出から24時間・再提出がないときの終了と返金）。
  expect(html).toContain("再提出から24時間後が新しい返却期限です");
  expect(html).toContain("差し戻しの間は代金の確保の取消しや返金はしません");
  expect(html).toContain("差し戻しから24時間以内（夜間も含めて数えます）に再提出がなかった場合、申込みは終了し");
  // 引受後・返却前のコーチ都合の辞退も、辞退と同じく終了・取消し・返金。
  expect(html).toContain("選んだコーチが辞退した場合（引き受けた後、返却の前の辞退を含みます）");
});

it("海外コーチ 1ヶ月コーチング（パイロット）の分を表示し、単発の動画添削は入れない（#539）", () => {
  const html = renderToStaticMarkup(<CommercePage />);
  expect(html).toContain("運営者が販売する「海外コーチ 1ヶ月コーチング（パイロット）」についての表記です。");
  expect(html).toContain("1ヶ月コーチング 22,000円（税込）");
  expect(html).toContain("決済リンク経由。1回ごとのお支払いです");
  expect(html).toContain("お申込み（決済リンクでのお支払い）の時に決済が完了します");
  expect(html).toContain("お支払いの確認後、案内した期間（1ヶ月間）について指導を提供します");
  expect(html).toContain("その月の料金の返金はなく、期間は月末までとなります");
  expect(html).toContain("未提供分を返金します");
  // 単発の海外動画添削は選手価格が未確定のため入れない。
  expect(html).not.toContain("単発");
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
