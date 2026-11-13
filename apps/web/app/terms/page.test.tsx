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
  expect(text).toContain("クレジットカードは代金の確保を取り消し、PayPayは全額を返金し、チケットは確保した1枚を戻します");
  expect(text).toContain("フォーム添削の契約は申込み時に成立し");
  // 料金は1本500円。チケットは6枚2,500円の先払いで、有効期限・失効・原則払戻し不可と例外・本人限定を定める（本体 #569）。
  expect(text).toContain("フォーム添削は1本500円（税込）");
  expect(text).toContain("6枚を1組として2,500円（税込）で販売し、6枚をまとめて先に支払います");
  expect(text).toContain("チケットの有効期限は、決済が完了した日時から6か月です");
  expect(text).toContain("有効期限は延長しません");
  expect(text).toContain("チケットは購入した本人だけが使えます");
  // チケット条項 v3（本体 #572）。第1項：購入者都合は原則払戻し不可＋第3項〜第5項・法令の例外への相互参照。
  expect(text).toContain("購入者の都合による払戻しは、原則として行いません");
  expect(text).toContain("（2）から（4）までおよび法令に基づく場合は、この限りではありません");
  // 第2項：失効。第3項〜第5項への留保＋運営・コーチ都合の失効後返金の例外。
  expect(text).toContain("次項（2）から（4）までに定める払戻しは、有効期限を過ぎたチケットについても行います");
  expect(text).toContain("当社またはコーチの都合により有効期限内に添削を受けることができなかったチケットについては、失効後であっても返金します");
  // 第3項：サービス終了時の払戻し（購入金額 ÷ 6 × 未使用枚数、四捨五入）。
  expect(text).toContain("運営者がフォーム添削の提供を終了した場合");
  expect(text).toContain("購入金額を6で割った額に未使用の枚数を掛けた額（1円未満の端数は四捨五入します）");
  // 第4項：法令上の取消し・解除（債務不履行・未成年者の取消し＋解除・損賠請求の非妨害）。
  expect(text).toContain("当社の債務不履行に基づく解除および未成年者の取消しを含みます");
  expect(text).toContain("この場合の契約の解除および損害賠償の請求は妨げられません");
  // 第5項：提供不能時（著しい遅れを追加、単発価格500円、解除・損賠請求の非妨害）。
  expect(text).toContain("または当社の都合により提供が著しく遅れた場合");
  expect(text).toContain("単発価格（500円）に相当する額を返金します");
  // 変更条項：購入済みチケットに不利益な変更を及ぼさない。
  expect(text).toContain("購入済みのチケットについて、有効期限の短縮その他の不利益な変更を及ぼす規約の変更はしません");
  // 絶対表現は使わない（本体 #572 項目9）。
  expect(text).not.toContain("一切");
  expect(text).not.toContain("いかなる理由があっても");
  // 追加質問の販売（1回500円・都度払いのみ・動画1本可）。購入済みの追加相談は購入時の条件を残す。
  expect(text).toContain("フォーム添削の追加質問は、返却された添削1件につき1回まで購入でき、料金は1回500円（税込、都度払いのみ）です");
  expect(text).toContain("チケットは使えません");
  expect(text).toContain("質問には動画を1本まで添えることができ、動画がなくても購入できます");
  expect(text).toContain("コーチの受諾は48時間以内、受諾後の回答は72時間以内");
  expect(text).toContain("辞退・回答のない期限切れは全額返金します");
  expect(text).toContain("回答がないまま終わった場合に限り、もう1回だけ改めて購入できます");
  expect(text).toContain("販売の終了前に購入した追加相談は、購入時の条件のとおり扱います");
  expect(text).not.toContain("追加相談の販売は終了しました");
  expect(text).not.toContain("600円");
  expect(text).not.toContain("1,500円");
  // 内容確認のための差し戻し（1回）と、再提出期限・返却期限の数え直し・再提出がないときの終了。
  expect(text).toContain("1つの依頼につき1回だけ質問を付けて差し戻せます");
  expect(text).toContain("差し戻し中は代金の確保の取消し・返金・チケットの返還は行いません");
  expect(text).toContain("コーチは理由を示して期限内に依頼を返却できます");
  expect(text).toContain("返却期限は再提出から24時間後になります");
  expect(text).toContain("期限までに再提出がない場合、依頼は終了し");
});

it("コーチ向けの業務委託は、報酬・支払期日・返金時の扱い・秘密保持を定め、代わりに受け取る仕組みの言葉を使わない", () => {
  const text = article(7);
  for (const word of agencyWords) expect(text).not.toContain(word);
  expect(text).toContain("業務委託の報酬");
  expect(text).toContain("料金500円の70%にあたる350円（税込）");
  expect(text).toContain("利用者がチケットで申し込んだフォーム添削も、1本につき350円です");
  expect(text).toContain("フォーム添削の追加質問への回答の報酬は、1件につき350円（税込、料金500円の70%）です");
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

it("サポートプランは、当事者・債権の帰属・代理受領・債務の消滅・二重請求の禁止・支払期日・返金等を定める", () => {
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

it("サポートプランの返金に伴う精算は、支払済みの報酬相当額の返還・次回以降の支払額からの控除と通知・内訳の説明を定める", () => {
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
  expect(review).toContain("追加質問は、この添削を担当したコーチへの");
  expect(review).toContain("この添削について相談できるのは1回限り");
  expect(review).toContain("追加のやりとりはできません");
  expect(review).toContain("回答がないまま終わった場合に限り、もう1回だけ改めて購入できます");
  expect(review).not.toContain("7日以内に1回無料");
  expect(review).toContain("提供者とお支払い先は TrackTalk の運営者");
  expect(review).toContain("料金・キャンセル・返金は運営者が責任を持って対応");
  const contract = renderToStaticMarkup(<ConsultationTermsNote kind="contract" />);
  expect(contract).toContain("契約の相手はコーチ");
  expect(contract).toContain("代理受領の権限");
  expect(contract).toContain("新しい質問1件とコーチの回答1件の1回限り");
  // サポートプランの追加相談は変えない。
  expect(contract).toContain("回答への確認は7日以内に1回無料");
  expect(contract).toContain("改めて購入してください");
});

it("未成年者の購入は、全員への購入前チェックと、申立てによる取消し・返金で定め、同意・上限・通知を前提にしない", () => {
  const text = article(4);
  expect(text).toContain("事前に親権者その他の法定代理人の許可を得るものとします。動画添削（単発・チケット）の購入では、年齢にかかわらず、購入の都度、決済に進む前に「未成年の方は保護者に許可を取ってください」の注意事項へのチェックを求めます");
  expect(text).toContain("当該購入を取り消し、代金を返金します");
  expect(text).toContain("保護者を契約および支払いの当事者として取り扱います");
  for (const word of ["支払い上限", "購入内容を通知", "同意を撤回"]) expect(text).not.toContain(word);
});

it("第8条：1ヶ月プランの途中終了（7の2）と外部勧誘の禁止（10・11項）を定める", () => {
  const text = article(8);
  expect(text).toContain("返金とキャンセル（1ヶ月プラン以外）：1ヶ月プラン以外のサポートプランの返金は");
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

it("動画のファイルの保存期間：フォーム添削はやりとりの終わりから7日、サポートプランは終了時点で見られなくなり動画は30日で削除", () => {
  const review = article(6);
  expect(review).toContain("フォーム添削などで送られた動画と、コーチの見本動画のファイルは、そのやりとりが終わってから7日で削除します");
  expect(review).toContain("添削と回答の文章は、動画のファイルを削除した後も見られます");
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
