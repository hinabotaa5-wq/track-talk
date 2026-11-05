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
  expect(text).toContain("前の月の末日までに確定した報酬をまとめて、その月の15日ごろ");
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
  expect(text).toContain("手数料20%を差し引いた残り");
  expect(text).toContain("前の月の末日までに確定した分をまとめて、その月の15日ごろ");
  expect(text).toContain("費用と事務の負担を抑えて");
  expect(text).toContain("まだ支払っていない額を記録して管理");
  expect(text).toContain("返金とキャンセル");
  expect(text).toContain("支払いの保留");
  expect(text).toContain("コーチをやめる場合の精算");
  // 現行の運用で分けて管理していないので、分別管理は約束しない。
  expect(text).not.toContain("分別");
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
