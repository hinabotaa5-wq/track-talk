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

it("フォーム添削と追加相談は運営者が提供者で、料金・返金の責任と直接取引の禁止を定める", () => {
  const text = article(6);
  for (const word of agencyWords) expect(text).not.toContain(word);
  expect(text).toContain("運営者が利用者に提供するサービス");
  expect(text).toContain("運営者がコーチに委託して行います");
  expect(text).toContain("代金を運営者に支払います");
  expect(text).toContain("料金・キャンセル・返金は運営者が責任を負い");
  expect(text).toContain("本サービスを通さずに直接取引してはいけません");
  // 指名したコーチが辞退・未応答のときは特商法の表記と同じ扱い。
  expect(text).toContain("クレジットカードは代金の確保を取り消し、PayPayは全額を返金します");
  expect(text).toContain("終了した依頼とは独立した新しい申込みとして、別のコーチに改めて申し込む");
  // 動画添削のあとの追加相談は一方通行で、1つの添削につき1回。回答がないまま終わった場合だけ1回買い直せる。
  expect(text).toContain("1つのフォーム添削について相談できるのは1回限りで、新しい質問1件とコーチの回答1件です");
  expect(text).toContain("回答への確認など、追加のやりとりはできません");
  expect(text).toContain("この場合に限り、同じフォーム添削について、もう1回だけ改めて購入できます");
  expect(text).not.toContain("回答への確認は7日以内に1回無料");
  // 内容確認のための差し戻し（1回）と、再提出期限・返却期限の数え直し・再提出がないときの終了。
  expect(text).toContain("1つの依頼につき1回だけ、質問を付けて依頼を差し戻す");
  expect(text).toContain("差し戻しの間に代金の確保の取消しや返金はしません");
  // 引受後・返却前のコーチ都合の辞退は、辞退の時点で終了する。
  expect(text).toContain("引き受けた後、返却の前の辞退を含みます。この場合は辞退の時点で終了します");
  expect(text).toContain("返却期限は再提出から24時間後に改めて設定し");
  expect(text).toContain("期限までに再提出がなかった場合、その申込みは終了し");
});

it("コーチ向けの業務委託は、報酬・支払期日・返金時の扱い・秘密保持を定め、代わりに受け取る仕組みの言葉を使わない", () => {
  const text = article(7);
  for (const word of agencyWords) expect(text).not.toContain(word);
  expect(text).toContain("業務委託の報酬");
  expect(text).toContain("1,050円");
  expect(text).toContain("420円");
  expect(text).toContain("報酬の支払期日：報酬は、毎月末日をもって締め切り、翌月15日に支払います。支払対象額（各締切日時点で確定している報酬の合計額をいい、繰り越された額を含みます）が3,000円未満の場合は、その額を翌月に1回に限り繰り越し、翌々月15日に、金額にかかわらず支払います。");
  expect(text).not.toContain("15日ごろ");
  expect(text).toContain("3,000円に満たない場合");
  expect(text).toContain("繰り越しは1回まで");
  expect(text).toContain("金額にかかわらず次の支払日に支払います");
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
  // 大会までプラン：終了時に日割りで確定、月次は仮払い、コーチ都合は返金額の75%を上限に返還（没収ではなく精算）、返金まで留保。
  expect(text).toContain("確定前の仮払いとします");
  expect(text).toContain("債務不履行に基づく精算として、支払済みの仮払いを返還します");
  expect(text).toContain("（返金額の75%）を上限とし");
  expect(text).toContain("購入者への返金が終わるまで、その契約に係るコーチへの支払いを留めます");
  expect(text).toContain("次回以降の報酬その他の支払額から控除することができます");
  expect(text).toContain("控除の対象と金額をコーチに通知します");
  expect(text).toContain("運営者に内訳の説明を求めることができます");
  // 第8条の次に置き、後ろの条は1つずつ繰り下げる。
  expect(renderToStaticMarkup(<TermsPage />)).toContain("第17条（規約の変更・準拠法・連絡）");
});

it("追加相談の購入画面は、1回限りであることと契約の相手を示す", () => {
  const review = renderToStaticMarkup(<ConsultationTermsNote kind="review" />);
  for (const word of agencyWords) expect(review).not.toContain(word);
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

it("未成年者の購入は、購入の都度の法定代理人の許可の申告と、申立てによる取消し・返金で定め、同意・上限・通知を前提にしない", () => {
  const text = article(4);
  expect(text).toContain("事前に親権者その他の法定代理人の許可を得るものとし、運営者は購入の都度、その申告を確認します");
  expect(text).toContain("当該購入を取り消し、代金を返金します");
  expect(text).toContain("保護者を契約および支払いの当事者として取り扱います");
  for (const word of ["支払い上限", "購入内容を通知", "同意を撤回"]) expect(text).not.toContain(word);
});

it("第8条：大会までプランの途中終了（7の2）と外部勧誘の禁止（10・11項）を定める", () => {
  const text = article(8);
  expect(text).toContain("返金とキャンセル（大会までプラン以外）：大会までプラン以外のサポートプランの返金は");
  expect(text).toContain("7の2. 大会までプランを途中で終える場合");
  expect(text).toContain("(1) 解約料・違約金・キャンセル手数料はいただきません。");
  expect(text).toContain("終える日は過ぎた日に数えます");
  expect(text).toContain("実際に指導が始まった日から1ヶ月（両方の日を含めた30日）");
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
