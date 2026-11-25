/** @vitest-environment jsdom */
import { renderToStaticMarkup } from "react-dom/server";
import { expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import { ConsultationTermsNote } from "@/components/coaching-consultations";
import TermsPage from "./page";
vi.mock("next/link", () => ({ default: ({ href, children, className }: { href: string; children: ReactNode; className?: string }) => <a href={href} className={className}>{children}</a> }));

// 条ごとの本文。見出し「第n条（…）」から次の条の見出しまで。
function article(n: number): string {
  const html = renderToStaticMarkup(<TermsPage />);
  const start = html.indexOf(`第${n}条（`);
  const end = html.indexOf(`第${n + 1}条（`);
  expect(start).toBeGreaterThan(-1);
  return html.slice(start, end === -1 ? undefined : end);
}

// 運営者が自ら提供するフォーム添削などには、代わりに受け取る仕組みの言葉を使わない。
const agencyWords = ["収納代行", "代理受領", "預り金", "預かり金"];

it("フォーム添削とチケットは運営者が提供者で、料金・返金の責任と直接取引の禁止を定める", () => {
  const text = article(6);
  for (const word of agencyWords) expect(text).not.toContain(word);
  expect(text).toContain("運営者が利用者に提供するサービス");
  expect(text).toContain("運営者がコーチに委託して行います");
  expect(text).toContain("代金を運営者に支払います");
  expect(text).toContain("料金・キャンセル・返金は運営者が責任を負い");
  expect(text).toContain("本サービスを通さずに直接取引してはいけません");
  // 指名したコーチが辞退・未応答のときは特商法の表記と同じ扱い。
  // PayPay は未提供のため、現在の返金方法として書かない（本体 #714）。PayPay 追加時に戻す。
  expect(text).toContain("これらの理由で終了した依頼は、クレジットカードは代金の確保を取り消し、チケットは確保した1枚を戻します");
  expect(renderToStaticMarkup(<TermsPage />)).not.toContain("PayPay");
  expect(text).toContain("スタンダードの契約は申込み時に成立し");
  expect(text).toContain("担当のコーチは、相談内容が分かりにくい場合に、1つの依頼につき1回だけ質問を付けて差し戻せます。");
  expect(text).toContain("スタンダードと同じ内容の指導を、本サービスを通さずに直接取引してはいけません。");
  expect(text).toContain("スタンダードのやりとりで、SNS・LINEなど本サービス外の連絡手段に誘ったり、SNSのアカウント、LINEのID、メールアドレス、電話番号などの外部連絡先を教えたり求めたりしてはいけません。");
  // 料金は1本1,000円（#983）。チケットは6枚5,000円の先払いで、有効期限・失効・原則払戻し不可と例外・本人限定を定める（本体 #569）。
  expect(text).toContain("スタンダードは1回1,000円（税込）");
  expect(text).toContain("6枚を1組として5,000円（税込）で販売し、6枚をまとめて先に支払います");
  expect(text).toContain("チケット6枚商品の有効期限は、決済が完了した日時から6か月です。以前に購入した1枚商品・5枚商品のチケットには、購入時の条件が適用され、有効期限はありません");
  expect(text).toContain("有効期限は延長しません");
  expect(text).toContain("チケットは購入した本人だけが使えます");
  // チケット条項 v3（本体 #572）。第1項：購入者都合は原則払戻し不可＋第3項〜第5項・法令の例外への相互参照。
  expect(text).toContain("購入者の都合による払戻しは、原則として行いません");
  expect(text).toContain("（2）から（4）までおよび法令に基づく場合は、この限りではありません");
  // 第2項：失効。第3項〜第5項への留保＋運営・コーチ都合の失効後返金の例外。
  expect(text).toContain("次項（2）から（4）までに定める払戻しは、有効期限を過ぎたチケットについても行います");
  expect(text).toContain("運営者またはコーチの都合により有効期限内にスタンダードを受けることができなかったチケットについては、失効後であっても返金します");
  // 第3項：サービス終了時の払戻し（購入金額 ÷ 6 × 未使用枚数、四捨五入）。
  expect(text).toContain("運営者がスタンダードの提供を終了した場合");
  expect(text).toContain("購入金額を6で割った額に未使用の枚数を掛けた額（1円未満の端数は四捨五入します）");
  // 第4項：法令上の取消し・解除（債務不履行・未成年者の取消し＋解除・損賠請求の非妨害）。
  expect(text).toContain("運営者の債務不履行に基づく解除および未成年者の取消しを含みます");
  expect(text).toContain("この場合の契約の解除および損害賠償の請求は妨げられません");
  // 第5項：提供不能時（著しい遅れを追加、単発価格1,000円、解除・損賠請求の非妨害）。
  expect(text).toContain("または運営者の都合により提供が著しく遅れた場合");
  expect(text).toContain("単発価格（1,000円）に相当する額を返金します");
  expect(text).toContain("返金は購入時の支払方法への返金を原則としますが、返金の合計額が購入時の決済額を超えるときは、その決済で返金できる額までを購入時の支払方法で返金し、超える分は運営者が別途指定する方法（銀行振込など）で支払います（支払方法はお問い合わせで案内します）。");
  // 変更条項：購入済みチケットに不利益な変更を及ぼさない。
  expect(text).toContain("購入済みのチケットについて、有効期限の短縮その他の不利益な変更を及ぼす規約の変更はしません");
  // 絶対表現は使わない（本体 #572 項目9）。
  expect(text).not.toContain("一切");
  expect(text).not.toContain("いかなる理由があっても");
  // 追加質問は販売終了（本体 #984）。販売終了前に購入した分は購入時の条件のとおり。旧・追加相談も購入時の条件を残す。
  expect(text).toContain("スタンダードの追加質問は、販売を終了しました。新たに購入することはできません。");
  expect(text).toContain("販売の終了前に購入した追加質問は、購入時の条件のとおり扱います（返却されたスタンダード1件につき1回、料金は1回500円（税込、都度払いのみ）。");
  expect(text).not.toContain("もう1回だけ改めて購入できます");
  expect(text).toContain("販売を終了した旧商品の「追加相談」を、その販売の終了前に購入していた場合は、購入時の条件のとおり扱います（旧商品の「追加相談」は、この項の追加質問とは別の商品です）");
  expect(text).not.toContain("追加相談の販売は終了しました");
  expect(text).not.toContain("600円");
  expect(text).not.toContain("1,500円");
  // 内容確認のための差し戻し（1回）と、再提出期限・返却期限の数え直し・再提出がないときの終了。
  expect(text).toContain("1つの依頼につき1回だけ質問を付けて差し戻せます");
  expect(text).toContain("差し戻し中は代金の確保の取消し・返金・チケットの返還は行いません");
  expect(text).toContain("コーチは理由を示して期限内に依頼を返却できます");
  expect(text).toContain("返却期限は再提出から24時間後になります");
  expect(text).toContain("期限までに再提出がない場合、依頼は終了し、第6項と同じ方法で代金の確保の取消し・チケットの返還を行います");
});

it("コーチ向けの業務委託は、報酬・支払期日・返金時の扱い・秘密保持を定め、代わりに受け取る仕組みの言葉を使わない", () => {
  const text = article(7);
  for (const word of agencyWords) expect(text).not.toContain(word);
  expect(text).toContain("業務委託の報酬");
  expect(text).toContain("料金1,000円の70%にあたる700円（税込）");
  expect(text).toContain("利用者がチケットで申し込んだスタンダードも、1件につき700円です");
  expect(text).toContain("スタンダードの追加質問への回答の報酬は、1件につき350円（税込、料金500円の70%）です");
  expect(text).not.toContain("1,050円");
  expect(text).not.toContain("420円");
  expect(text).toContain("報酬の支払期日：毎月末日を締切とし、翌月15日に支払います。ただし、各締切時点の支払額（税込）が金3,000円未満の場合は翌月以降に繰り越し、累積した支払額が金3,000円以上となった締切月の翌月15日に支払います。なお、コーチとの取引が終了した場合は、金額にかかわらず、終了月の翌月15日に未払額の全額を支払います。");
  expect(text).not.toContain("15日ごろ");
  expect(text).toContain("3,000円に満たない場合");
  expect(text).not.toContain("繰り越しは1回まで");
  expect(text).not.toContain("翌々月15日");
  expect(text).toContain("累積が3,000円以上となるまで、翌月以降に繰り越します");
  expect(text).toContain("まだ支払っていない報酬の全額を、金額にかかわらず終了月の翌月15日に支払います");
  expect(text).toContain("その後に支払う報酬から差し引きます");
  expect(text).toContain("第三者に開示せず");
});

it("継続プランは、当事者・債権の帰属・代理受領・債務の消滅・二重請求の禁止・支払期日・返金等を定める", () => {
  const text = article(8);
  expect(text).toContain("利用者とプランを出しているコーチとの間で成立");
  expect(text).toContain("代金債権）はコーチに帰属");
  expect(text).toContain("代理受領権）を与えます");
  expect(text).toContain("支払義務は消滅します");
  expect(text).toContain("重ねて請求することはできません");
  expect(text).toContain("手数料25%を差し引いた残り");
  expect(text).toContain("前の月の末日までに確定した分をまとめて、その月の15日");
  expect(text).toContain("費用と事務の負担を抑えて");
  expect(text).toContain("まだ支払っていない額を記録して管理");
  expect(text).toContain("返金とキャンセル");
  expect(text).toContain("支払いの保留");
  expect(text).toContain("コーチをやめる場合の精算");
  // 現行の運用で分けて管理していないので、分別管理は約束しない。
  expect(text).not.toContain("分別");
});

it("継続プランの返金に伴う精算は、支払済みの報酬相当額の返還・次回以降の支払額からの控除と通知・内訳の説明を定める", () => {
  const text = article(9);
  expect(text).toContain("購入者への返金に伴う精算");
  expect(text).toContain("その返金に対応する報酬相当額を返還します");
  // 1ヶ月プラン：終了時に日割りで確定（経過分＝代金−返金額）、月次は仮払い、コーチ都合は返金額の75%を上限に返還（没収ではなく精算）、返金まで留保。
  expect(text).toContain("1ヶ月プランのコーチへの支払額は、契約が終わった時に");
  expect(text).toContain("経過分の代金は、代金から第8条第7の2項(2)の返金額を差し引いた額とし、1円未満は切り捨てます");
  expect(text).toContain("確定前の仮払いとし、第8条第5項の支払日に支払います");
  expect(text).not.toContain("大会までプラン");
  expect(text).toContain("債務不履行に基づく精算として、支払済みの仮払いを返還します");
  expect(text).toContain("（返金額の75%）を上限とし");
  expect(text).toContain("購入者への返金が終わるまで、その契約に係るコーチへの支払いを留めます");
  // 4の2：やむを得ない事情は通常の精算、書面で通知、異議は1回・期限なし、判断が変わっても全額返金は取り消さない。
  expect(text).toContain("4の2. コーチ本人の入院・死亡、災害による業務不能、法令による業務停止など");
  expect(text).toContain("第3項は適用せず、第1項のとおり精算します");
  expect(text).toContain("1回に限り、期限なく異議を申し立てて再審査を求めることができます");
  expect(text).toContain("購入者への全額の返金は取り消さず、コーチには終わった日までの経過日数の日割りで第1項の支払額を支払います");
  expect(text).toContain("次回以降の報酬その他の支払額から控除することができます");
  expect(text).toContain("控除の対象と金額をコーチに通知します");
  expect(text).toContain("運営者に内訳の説明を求めることができます");
  expect(text).toContain("前7項の規定にかかわらず、返金が運営者の責めに帰すべき事由による場合、その事由に対応する部分については、コーチは第2項、第3項及び第6項の返還義務を負わず、第5項の控除の対象にもなりません");
  // 第8条の次に置き、後ろの条は1つずつ繰り下げる。
  expect(renderToStaticMarkup(<TermsPage />)).toContain("第17条（規約の変更・準拠法・連絡）");
});

it("追加質問の購入画面は、1回限りであることと契約の相手を示す", () => {
  const review = renderToStaticMarkup(<ConsultationTermsNote kind="review" />);
  for (const word of agencyWords) expect(review).not.toContain(word);
  expect(review).toContain("回答がないまま終わった場合に限り、もう1回だけ購入できます。提供・お支払いは TrackTalk の運営者です（回答はコーチに委託します）。");
  expect(review).not.toContain("7日以内に1回無料");
  const contract = renderToStaticMarkup(<ConsultationTermsNote kind="contract" />);
  expect(contract).toContain("契約の相手はコーチ");
  expect(contract).toContain("代理受領の権限");
  expect(contract).toContain("新しい質問1件とコーチの回答1件の1回限り");
  // 継続プランの追加相談は変えない。
  expect(contract).toContain("回答への確認は7日以内に1回無料");
  expect(contract).toContain("改めて購入してください");
});

it("未成年者の購入は、全員への購入前チェックと、申立てによる取消し・返金で定め、同意・上限・通知を前提にしない", () => {
  const text = article(4);
  expect(text).toContain("事前に親権者その他の法定代理人の許可を得るものとします。スタンダード（単発・チケット）とその追加質問の購入では、年齢にかかわらず、購入の都度、決済に進む前に「未成年の方は保護者に許可を取ってください」の注意事項へのチェックを求めます");
  expect(text).toContain("当該購入を取り消し、代金を返金します");
  expect(text).toContain("保護者を契約および支払いの当事者として取り扱います");
  for (const word of ["支払い上限", "購入内容を通知", "同意を撤回"]) expect(text).not.toContain(word);
});

it("国内継続プランの販売者案内から第8条の見出しへリンクできる", () => {
  const container = document.createElement("div");
  container.innerHTML = renderToStaticMarkup(<TermsPage />);
  expect(container.querySelectorAll("#article-8")).toHaveLength(1);
  expect(container.querySelector("h2#article-8")?.textContent).toBe("第8条（継続プランの契約と代金の受け取り）");
});

it("第8条：1ヶ月プランの途中終了（7の2）と外部勧誘の禁止（10・11項）を定める", () => {
  const text = article(8);
  expect(text).toContain("返金とキャンセル（1ヶ月プラン以外）：1ヶ月プラン以外の継続プランの返金は");
  expect(text).toContain("7の2. 1ヶ月プランを途中で終える場合");
  expect(text).toContain("提案に示した開始日から1ヶ月（両方の日を含めた30日）とし、終了日は契約に記録された日とします");
  expect(text).toContain("(1) 解約料・違約金・キャンセル手数料はいただきません。利用者は、いつでも途中で終えることができます。");
  expect(text).toContain("契約に記録された開始日から終了日までの日数（両方の日を含めた30日）とし、提案と最終確認の画面に示した日数と同じです");
  expect(text).toContain("終えることが決まった日は、過ぎた日に数えます");
  expect(text).toContain("開始日の前に終える場合は全額を返金し、終了日以降に終える場合は返金はありません");
  expect(text).toContain("動画への返答や通話の回数は、返金の額に関係しません");
  expect(text).toContain("利用者に全額を返金した後は、その返金を取り消しません");
  expect(text).toContain("(4) 目標にしていた大会が中止・延期になった場合も、1ヶ月プランは期間どおり続きます");
  expect(text).toContain("(6) 返金の求めは、お問い合わせで受け付けます。");
  expect(text).not.toContain("大会までプラン");
  expect(text).not.toContain("主催者が正式に発表した日をもって契約を終えます");
  expect(text).toContain("SNS・LINE などの本サービスの外の連絡手段へ誘うこと");
  expect(text).toContain("確認は通報があった場合などに限ります");
  const html = renderToStaticMarkup(<TermsPage />);
  expect(html).toContain('value="10"');
  expect(html).toContain('value="11"');
});

it("動画のファイルの保存期間：フォーム添削はやりとりの終わりから7日、継続プランは終了時点で見られなくなり動画は30日で削除", () => {
  const review = article(6);
  expect(review).toContain("スタンダードなどで送られた動画と、コーチの見本動画のファイルは、そのやりとりが終わってから7日で削除します");
  expect(review).toContain("回答の文章は、動画のファイルを削除した後も見られます");
  const plan = article(8);
  expect(plan).toContain("契約が終わった時点（期間の満了または途中での終了）で、利用者・コーチ・保護者のいずれからも見られなくなります");
  expect(plan).toContain("契約が終わるまでのあいだに、利用者とコーチは文章とメニューをダウンロードできます");
  expect(plan).toContain("やりとりの動画のファイルは、契約が終わってから30日で削除します");
  expect(plan).toContain("対応が終わるまで保全します");
});

it("第6条に紹介特典の本人利用・付与日起算の期限・払戻し条件を追加する", () => {
  const text = article(6);
  expect(text).toContain("紹介特典チケットについて");
  expect(text).toContain("特典チケットは、付与された本人だけが使えます");
  expect(text).toContain("特典チケットの有効期限は、付与した日時から6か月です");
  expect(text).toContain("サービス提供の終了時に行う未使用チケットの払戻しの計算にも、特典チケットは含めません");
  expect(text).toContain("使用した特典チケット1枚を戻します");
});

// #717：支払いに含めた後は本サービス上で返金できないが、法令に基づく返金と運営者の個別の返金は除き、その精算は第9条第6項でつなぐ。
it("1ヶ月プラン以外の継続プランの支払済み後の返金は、例外を明記して第9条第6項の返還につなげる", () => {
  const eight = article(8);
  expect(eight).toContain("コーチへの支払いに含めた後は、本サービス上で返金の手続きはできません。ただし、法令に基づいて返金する場合や、運営者が個別に判断して返金する場合は除きます。この場合のコーチとの精算は、第9条第6項のとおりとします。");
  const nine = article(9);
  expect(nine).toContain("運営者が購入者に返金を行った場合（第8条第7項のただし書きの場合を含みます）において、その返金に係る報酬を運営者がコーチに支払済みのときは");
  // 退会の前に対応を終える必要がある（実装は退会を止める）。
  expect(article(15)).toContain("先に対応を終える必要があります");
});

// #717 残件2：第15条の「未解決の通報」は、自分の指導のやりとりについて受けたものに絞る。列挙の他項目は不変。
it("第15条の退会の前提になる通報は、自分の指導のやりとりについて受けた未解決の通報に限る", () => {
  const text = article(15);
  expect(text).toContain("未解決の通報（あなたの指導のやりとりについて受けたもの）");
  expect(text).not.toContain("未解決の通報、まだ支払っていない");
  // 列挙の他の項目は変えない。
  expect(text).toContain("進行中の契約、未使用・確保中のチケット、返金、");
  expect(text).toContain("まだ支払っていないコーチの報酬などがある場合は、先に対応を終える必要があります");
});

it("第7条・第8条は国内コーチに適用し、海外コーチは Coach Terms・work order と特商法表記に接続する（#718）", () => {
  const seven = article(7);
  expect(seven).toContain("この条は、日本国内に住むコーチ（以下「国内コーチ」）に適用します");
  expect(seven).toContain("Terms of Engagement for Overseas Coaches");
  expect(seven).toContain('href="/legal/coach-terms"');
  expect(seven).toContain("依頼ごとの個別の発注書（work order）によります");
  const eight = article(8);
  expect(eight).toContain("この条の継続プランは、国内コーチが出すものに限ります");
  expect(eight).toContain("運営者が販売者となり利用者との契約の当事者となる別の取引であり、この条の第1項から第9項まで");
  expect(eight).toContain('href="/legal/commerce"');
});

// 本体 #1007 C5-1：第8条第2項の海外商品名は、特商法の表示名「海外コーチ プレミアム（パイロット）」に揃える。
it("第8条の海外商品名は「海外コーチ プレミアム（パイロット）」で、旧「海外コーチ 1ヶ月コーチング」を含まない", () => {
  const eight = article(8);
  expect(eight).toContain("海外コーチが指導する「海外コーチ プレミアム（パイロット）」は、運営者が販売者となり利用者との契約の当事者となる別の取引であり");
  expect(eight).not.toContain("海外コーチ 1ヶ月コーチング");
  expect(renderToStaticMarkup(<TermsPage />)).not.toContain("1ヶ月コーチング");
});

// 本体 #1007 B6：第2条は「スタンダードの提供」だけを示し、継続プランを「準備中」と提示しない。
it("第2条は「スタンダードの提供」で、継続プランの準備中の提示が無い", () => {
  const two = article(2);
  expect(two).toContain("第2条（スタンダードの提供）");
  expect(two).toContain("本サービスでは、スタンダード（フォーム添削や競技に関する悩み）を提供しています。");
  expect(two).not.toContain("継続プラン");
  expect(two).not.toContain("準備中");
  expect(renderToStaticMarkup(<TermsPage />)).not.toContain("1回プラン");
});

it("第6条の料金と追加質問は国内コーチに適用し、海外単発は個別表示・追加質問なしとする（#764）", () => {
  const six = article(6);
  expect(six).toContain("この条に定めるスタンダードの料金（第2項の一律1,000円）および追加質問（第10項）は、日本国内に住むコーチ（国内コーチ）が担当する場合についての定めです。");
  expect(six).toContain("日本国外に住むコーチ（海外コーチ）によるスタンダードの単発の価格その他の条件は、購入画面および個別の表示で示し、追加質問は付きません。");
  expect(six).toContain("海外コーチによるスタンダードの単発には追加質問は付きません。");
  // 従来の文が実質そのまま読めること
  expect(six).toContain("スタンダードは1回1,000円（税込）");
  expect(six).toContain("スタンダードの追加質問は、販売を終了しました。");
});

it("第11条：感覚メモの「本人だけに」に継続プラン共有の例外を加え、競技会の記録は本人だけのまま（#878）", () => {
  const text = article(11);
  // （a）従来文はそのまま。
  expect(text).toContain("感覚メモと競技会の記録の内容は本人だけに、ケガのグループの投稿はその参加者に表示します。");
  // （b）固定の例外文。
  const exception = "ただし、継続プランの契約期間中は、選手が共有をやめていない感覚メモを担当コーチが見ることができます。";
  expect(text).toContain(exception);
  // （c）例外文は感覚メモだけを対象にし、競技会の記録に触れない。
  expect(exception).not.toContain("記録");
  expect(exception).not.toContain("競技会");
});

it("第7条第7項：単発の返金時の報酬の扱いに、運営者の帰責事由による場合の除外を明記する（#903）", () => {
  const html = renderToStaticMarkup(<TermsPage />);
  // 第7条の ol の7番目の li。
  const items = [...article(7).matchAll(/<li>(.*?)<\/li>/g)].map((m) => m[1]);
  const item7 = items[6];
  // 細目2：従来文（不払い・保留・差引き・翌月以降への繰越）はそのまま残る。
  const original = "報酬の支払い前に返金が決まった場合、その依頼の報酬は支払いません。返金の手続き中は、その依頼の報酬の支払いを保留します。報酬を支払った後に返金した場合は、返金した分に応じた報酬の額を、その後に支払う報酬から差し引きます。その月に差し引ききれない分は、翌月以降の報酬から差し引きます。";
  // 細目1：指定の追加文が1文そのまま、従来文の直後に入る。
  const added = "ただし、返金が運営者の責めに帰すべき事由による場合、その事由に対応する部分については、報酬の不払いおよび差引きの対象としません。";
  expect(item7).toBe(original + added);
  expect(html.split(added).length - 1).toBe(1);
  // 細目3：第9条第8項は変更しない。
  expect(article(9)).toContain("<li>前7項の規定にかかわらず、返金が運営者の責めに帰すべき事由による場合、その事由に対応する部分については、コーチは第2項、第3項及び第6項の返還義務を負わず、第5項の控除の対象にもなりません。</li>");
});

it("最終更新日が実装日（2026年11月23日）に更新されている（#903・#983）", () => {
  const html = renderToStaticMarkup(<TermsPage />);
  expect(html).toContain("最終更新日: 2026年11月23日");
  expect(html).not.toContain("2026年11月17日");
});
