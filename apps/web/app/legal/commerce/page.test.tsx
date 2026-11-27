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
  expect(html).toContain("TrackTalk運営者（個人）");
  expect(html).not.toContain("中窪日向");
  expect(html).toContain("請求があれば遅滞なく開示します。");
  expect(html).toContain("mailto:tracktalk.office@gmail.com");
  expect(showsBackButton("/legal/commerce")).toBe(true);
});

it("公開文面に開発中の注記や販売を終えた追加相談を表示しない", () => {
  const html = renderToStaticMarkup(<CommercePage />);
  expect(html).not.toMatch(/たたき台|下書き|未確定|要確認|TODO|確定していません|確定後|追加相談/);
  // T3：価格はコーチごとに異なり、表示場所を示す。固定額・一律の断言は載せない（#1058）。
  expect(html).toContain("<p>スタンダードの価格（1回・税込）はコーチごとに異なります。各コーチのページと申込画面に表示します。</p>");
  expect(html).not.toContain("スタンダード 1回 1,000円");
  expect(html).not.toContain("どのコーチを選んでも同じ");
  // チケットは販売終了（本体 #1031）。価格は載せず、発行済みのチケットが有効期限まで使えることを書く。
  expect(html).not.toContain("スタンダードのチケット 6枚 5,000円（税込）");
  expect(html).toContain("スタンダードのチケットは販売を終了しました。発行済みのチケットは、購入時（特典チケットは付与時）の条件のとおり、有効期限まで利用できます。販売の終了前に購入したチケット（6枚）の有効期限は決済完了の日時から6か月");
  expect(html).toContain("紹介特典の¥500クーポンを使う場合は、選んだコーチのスタンダード1回の価格（表示価格）から500円を割り引いた額をお支払いいただきます。");
  expect(html).not.toContain("（1回 500円（税込））");
  // 追加質問は販売終了（本体 #984）。販売終了前に購入した分の価格・支払時期・提供時期・返金だけを残す。
  expect(html).toContain("スタンダードの追加質問は販売を終了しました。");
  expect(html).not.toContain("スタンダードの追加質問 1回 500円（税込）");
  expect(html).not.toContain("1回まで購入でき");
  expect(html).toContain("販売の終了前に購入したスタンダードの追加質問は、申込みの時に1回分の代金を決済済みです。");
  expect(html).toContain("受諾は支払いから24時間以内、回答は受諾から12時間以内です。");
  expect(html).toContain("コーチの辞退・未回答の期限切れの場合は全額返金し");
  // 支払方法の欄は今使える方法だけ（PayPay は載せず、対応予定の案内も入れない）。
  expect(html).toContain("<p>クレジットカード（Apple Pay・Google Pay を含む）。1回ごとのお支払いです。</p>");
  expect(html).not.toContain("、PayPay。");
  expect(html).not.toContain("対応予定");
  // 支払時期・キャンセル・返金の欄にも PayPay の条件文を残さない（支払方法がカードのみのため）。
  expect(html).not.toContain("PayPay");
  expect(html).not.toContain("購入の時に6枚分の代金をまとめて決済します。");
  expect(html).toContain("販売の終了前に購入したスタンダードのチケットは、購入の時に代金を決済済みです。");
  expect(html).toContain("コーチが対応を始める前は依頼画面からキャンセルできます。");
  expect(html).toContain("スタンダードの契約は申込み時に成立し");
  expect(html).toContain("コーチの対応中一覧に表示された時刻から24時間以内");
  // チケットの条件（先払い・6枚一括・有効期限・原則払戻し不可と例外・本人限定。本体 #569）。
  expect(html).toContain("有効期限は決済完了の日時から6か月");
  expect(html).toContain("購入者の都合による払戻しを原則として行いません");
  expect(html).toContain("運営者がスタンダードの提供を終了したとき");
  expect(html).toContain("購入した本人だけが使え、譲渡はできません");
  // 例外4項目の箇条書きと払戻額の計算（購入金額 ÷ 6 × 未使用枚数、四捨五入。本体 #572）。
  expect(html).toContain("（1）運営者がスタンダードの提供を終了したとき");
  expect(html).toContain("（2）法令に基づき契約の取消し・解除ができるとき");
  expect(html).toContain("（3）運営者またはコーチの責めに帰すべき事由によってスタンダードを受けられなかったとき");
  expect(html).toContain("（4）運営者またはコーチの都合により有効期限内にスタンダードを受けられなかったチケットは、失効後であっても");
  // 計算式は（1）の提供終了にだけかかり、（3）（4）の運営・コーチ都合は1,000円（利用規約第6条第5項。本体 #715）。
  expect(html).toContain("終了の時点で未使用のチケット（有効期限にかかわらず）について、購入金額を6で割った額に未使用の枚数を掛けた額（1円未満の端数は四捨五入します）を払い戻します");
  // T4：チケット払戻しの1,000円は不変（言い方だけ「チケット1枚分の額」に。#1058）。
  expect(html).toContain("チケット1枚分の額（1,000円）に相当する額を返金します");
  expect(html).not.toContain("スタンダードの1回分の価格（1,000円）");
  expect(html).toContain("購入金額を6で割って計算するのは（1）の場合だけで、（3）と（4）の返金はこの計算によらず、受けられなかったスタンダード1回（チケット1枚）につき1,000円です");
  expect(html).toContain("返金は購入時の支払方法への返金を原則としますが、返金の合計額が購入時の決済額を超えるときは、その決済で返金できる額までを購入時の支払方法で返金し、超える分は運営者が別途指定する方法（銀行振込など）で支払います（お問い合わせで案内します）。");
  expect(html).not.toContain("払戻しの額は購入金額を6で割った額");
  // 「チケットを戻す」は依頼終了時の1枚返還の場面に限り、購入チケットの払戻し額とは混ぜない。
  expect(html).not.toContain("チケットを戻すか");
  expect(html).toContain("申し込んだ依頼が終了したときに確保した1枚を戻す扱いは、この欄の上の段落のとおりで、ここでいう払戻しとは別のものです");
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
  expect(html).toContain("クレジットカードは代金の確保を取り消し、チケットは確保した1枚を、¥500クーポンは使ったクーポンを戻します");
  expect(html).toContain("チケットの有効期限を過ぎてから依頼が終了したときは、戻るチケットは失効します");
  expect(html).toContain("有効期限内にスタンダードを受けられなかった場合の失効後の返金は、利用規約第6条第5項（4）のとおりです");
});

it("海外コーチ プレミアム（パイロット）の分を表示する（#539、#1007 で月額から置き換え）", () => {
  const html = renderToStaticMarkup(<CommercePage />);
  expect(html).toContain("運営者が販売する「海外コーチ プレミアム（パイロット）」についての表記です。");
  // 海外プレミアムの新方針（#1007 運営者決定 2026-11-24）：1件 20,000円、動画最大10本・総括1回。C3。
  expect(html).toContain("海外コーチ プレミアム（パイロット）は 1件 20,000円（税込）。動画最大10本までをまとめて送り、コーチが総括1回で回答します。");
  expect(html).toContain("動画最大10本");
  expect(html).toContain("総括1回");
  for (const old of ["60,000円", "月20本", "1ヶ月コーチング", "22,000円", "その1ヶ月分の料金の返金はなく", "案内した開始日から1ヶ月間"]) expect(html).not.toContain(old);
  expect(html).toContain("海外コーチ プレミアム（パイロット）もインターネットの通信料は利用者の負担です。");
  expect(html).toContain("海外コーチ プレミアム（パイロット）はクレジットカード（決済リンク経由。1回ごとのお支払いです）。");
  expect(html).toContain("海外コーチ プレミアム（パイロット）はお申込み（決済リンクでのお支払い）の時に決済が完了します。");
  // 提供時期・返金は仮仕様（#1007 §8-2。デプロイ前に運営者が確定させる）。
  expect(html).toContain("海外コーチ プレミアム（パイロット）は、お支払いの確認後、運営者があなたの動画（最大10本）と相談内容を担当コーチへ送り、総括1回の回答を提供します。回答までの期間はお申込みの前に案内します。");
  expect(html).toContain("海外コーチ プレミアム（パイロット）は、回答が提供される前であれば、お問い合わせから取り消すことができ、その場合は全額を返金します。コーチ側の事情で回答が提供できなかった場合も、全額を返金します。回答が提供された後は、利用者の都合による返金はできません。");
  // 海外は運営者が販売者・契約当事者で、利用規約第8条（国内コーチの継続プラン）の当事者・返金の定めは適用しない（#718）。
  expect(html).toContain("日本国外に住むコーチ（海外コーチ）が指導し、運営者が販売者・契約の当事者になります");
  expect(html).toContain("利用規約第8条（継続プランの契約と代金の受け取り）の契約の当事者・返金・途中で終える場合の定めは適用せず");
  expect(html).toContain("日本国内に住むコーチが出す継続プラン（1ヶ月の継続サポート）はコーチが販売者になる");
});

it("海外コーチ スタンダード（パイロット）の分を表示する（追加質問・チケット・PayPay は載せない）", () => {
  const html = renderToStaticMarkup(<CommercePage />);
  expect(html).toContain("運営者が販売する「海外コーチ スタンダード（パイロット）」についての表記です。");
  expect(html).not.toContain("単発動画添削（パイロット）");
  expect(html).toContain("利用規約第6条冒頭の定めにより、海外コーチによる単発の価格その他の条件は、このページと購入時の個別の案内で示します。追加質問は付きません。お申込みは決済リンク経由で受け付け、チケットは使えません。");
  expect(html).toContain("海外コーチ スタンダード（パイロット）は 1回 10,000円（税込）。動画1本への回答1回分です。");
  expect(html).not.toContain("海外コーチ スタンダード（パイロット）は 1回 2,000円");
  expect(html).toContain("海外コーチ スタンダード（パイロット）もインターネットの通信料は利用者の負担です。");
  expect(html).toContain("海外コーチ スタンダード（パイロット）はクレジットカード（決済リンク経由。1回ごとのお支払いです）。");
  expect(html).toContain("海外コーチ スタンダード（パイロット）はお申込み（決済リンクでのお支払い）の時に決済が完了します。");
  expect(html).toContain("コーチが受け取ってから約24時間以内の回答を目安として提供します");
  expect(html).toContain("海外コーチ スタンダード（パイロット）は、回答が提供される前であれば、お問い合わせから取り消すことができ、その場合は全額を返金します。コーチ側の事情で回答が提供できなかった場合も、全額を返金します。回答が提供された後は、利用者の都合による返金はできません。");
  // スタンダード（単発）分の段落は、各 Row で海外プレミアム分の段落の直後に置く。
  expect(html.indexOf("海外コーチ スタンダード（パイロット）は 1回 10,000円")).toBeGreaterThan(html.indexOf("1件 20,000円（税込）"));
  expect(html.indexOf("1件 20,000円（税込）")).toBeGreaterThan(-1);
  expect(html.indexOf("運営者が販売する「海外コーチ スタンダード（パイロット）」")).toBeGreaterThan(html.indexOf("運営者が販売する「海外コーチ プレミアム（パイロット）」"));
  const single = html.slice(html.indexOf("運営者が販売する「海外コーチ スタンダード（パイロット）」"));
  expect(single).not.toContain("PayPay");
});

// 別言語の API 定数とのずれを検出する。スタンダードはコーチごとの価格なので固定額を載せず（本体 #1058）、
// クーポンの額面だけ API と突き合わせる。チケットは販売終了のため価格を載せない（本体 #1031）。
it("クーポンの額面を API に一致させ、スタンダードの固定額は載せない", () => {
  const api = path.resolve(process.cwd(), "../api");
  const review = readFileSync(path.join(api, "internal/httpx/review_payment.go"), "utf8");
  const coupon = readFileSync(path.join(api, "internal/httpx/coaching_coupon.go"), "utf8");
  const html = renderToStaticMarkup(<CommercePage />);
  const price = review.match(/const reviewPriceYen int32 = (\d+)/);
  expect(price).not.toBeNull();
  expect(html).not.toContain(`スタンダード 1回 ${Number(price![1]).toLocaleString("ja-JP")}円`);
  const amount = coupon.match(/const referralCouponYen int32 = (\d+)/);
  expect(amount).not.toBeNull();
  expect(html).toContain(`価格（表示価格）から${Number(amount![1]).toLocaleString("ja-JP")}円を割り引いた額をお支払いいただきます。`);
  expect(html).not.toContain(`（1回 ${(Number(price![1]) - Number(amount![1])).toLocaleString("ja-JP")}円（税込））`);
  expect(html).not.toMatch(/チケット\s*6枚\s*[\d,]+円/);
});
