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

it("公開文面に開発中の注記や販売を終えた追加相談を表示しない", () => {
  const html = renderToStaticMarkup(<CommercePage />);
  expect(html).not.toMatch(/たたき台|下書き|未確定|要確認|TODO|確定していません|確定後|追加相談/);
  expect(html).toContain("フォーム添削 1本 500円（税込）");
  expect(html).toContain("フォーム添削のチケット 6枚 2,500円（税込）");
  // 追加質問（1回500円・都度払いのみ・チケット不可）の販売価格・支払時期・提供時期・返金。
  expect(html).toContain("フォーム添削の追加質問 1回 500円（税込）");
  expect(html).toContain("返却された添削1件につき1回まで購入でき、都度払いのみで、チケットは使えません。");
  expect(html).toContain("フォーム添削の追加質問は、申込みの時に1回分の代金を決済します。");
  expect(html).toContain("受諾は支払いから48時間以内、回答は受諾から72時間以内です。");
  expect(html).toContain("コーチの辞退・未回答の期限切れの場合は全額返金し");
  // 支払方法の欄は今使える方法だけ（PayPay は載せず、対応予定の案内も入れない）。
  expect(html).toContain("<p>クレジットカード（Apple Pay・Google Pay を含む）。1回ごとのお支払いです。</p>");
  expect(html).not.toContain("、PayPay。");
  expect(html).not.toContain("対応予定");
  // 支払時期・キャンセル・返金の欄にも PayPay の条件文を残さない（支払方法がカードのみのため）。
  expect(html).not.toContain("PayPay");
  expect(html).toContain("購入の時に6枚分の代金をまとめて決済します。");
  expect(html).toContain("コーチが添削を始める前は依頼画面からキャンセルできます。");
  expect(html).toContain("フォーム添削の契約は申込み時に成立し");
  expect(html).toContain("コーチの対応中一覧に表示された時刻から24時間以内");
  // チケットの条件（先払い・6枚一括・有効期限・原則払戻し不可と例外・本人限定。本体 #569）。
  expect(html).toContain("有効期限は決済完了の日時から6か月");
  expect(html).toContain("購入者の都合による払戻しを原則として行いません");
  expect(html).toContain("運営者がフォーム添削の提供を終了したとき");
  expect(html).toContain("購入した本人だけが使え、譲渡はできません");
  // 例外4項目の箇条書きと払戻額の計算（購入金額 ÷ 6 × 未使用枚数、四捨五入。本体 #572）。
  expect(html).toContain("（1）運営者がフォーム添削の提供を終了したとき");
  expect(html).toContain("（2）法令に基づき契約の取消し・解除ができるとき");
  expect(html).toContain("（3）当社またはコーチの責めに帰すべき事由によってフォーム添削を受けられなかったとき");
  expect(html).toContain("（4）当社またはコーチの都合により有効期限内に添削を受けられなかったチケットは、失効後であっても");
  expect(html).toContain("購入金額を6で割った額に未使用の枚数を掛けた額とし、1円未満の端数は四捨五入します");
  expect(html).toContain("動画添削1回分の価格（500円）に相当する額を返金します");
  expect(html).not.toContain("切り捨てます");
  expect(html).not.toContain("一切");
  expect(html).not.toContain("いかなる理由があっても");
  expect(html).not.toContain("添削・追加相談が返却された時に確定");
  // 内容確認のための差し戻しの例外（再提出から24時間・再提出がないときの終了と返金）。
  expect(html).toContain("再提出から24時間後が新しい返却期限です");
  expect(html).toContain("差し戻しの間は代金の確保の取消しや返金はしません");
  expect(html).toContain("差し戻しから24時間以内（夜間も含めて数えます）に再提出がなかった場合、申込みは終了し");
  expect(html).toContain("着手後は利用者都合でキャンセルできません");
  expect(html).toContain("コーチは理由を示して期限内に依頼を返却できます");
  expect(html).toContain("クレジットカードは代金の確保を取り消し、チケットは確保した1枚を戻します");
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

// 別言語の API 料金定数・チケットの商品とのずれを検出する。決済実装自体は変更しない。
it("掲載料金を API の決済用定数とチケットの商品に一致させる", () => {
  const api = path.resolve(process.cwd(), "../api");
  const review = readFileSync(path.join(api, "internal/httpx/review_payment.go"), "utf8");
  const migration = readFileSync(path.join(api, "migrations/000159_review_ticket_redesign.up.sql"), "utf8");
  const html = renderToStaticMarkup(<CommercePage />);
  const price = review.match(/const reviewPriceYen int32 = (\d+)/);
  expect(price).not.toBeNull();
  expect(html).toContain(`フォーム添削 1本 ${Number(price![1]).toLocaleString("ja-JP")}円（税込）`);
  const six = migration.match(/\('six', '[^']*', (\d+), (\d+), true\)/);
  expect(six).not.toBeNull();
  expect(html).toContain(`フォーム添削のチケット ${six![1]}枚 ${Number(six![2]).toLocaleString("ja-JP")}円（税込）`);
});
