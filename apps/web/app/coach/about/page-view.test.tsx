/** @vitest-environment jsdom */
import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { CoachAboutView } from "./page-view";
import { metadata } from "./page";

vi.mock("next/link", () => ({ default: ({ href, children, replace, ...props }: { href: string; children: ReactNode; replace?: boolean }) => <a href={href} data-replace={replace} {...props}>{children}</a> }));

let root: Root;
let container: HTMLDivElement;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  container = document.createElement("div"); document.body.appendChild(container); root = createRoot(container);
});
afterEach(async () => { await act(async () => root.unmount()); container.remove(); });

// 本体 #1007 B9・B6：継続プランは休眠。継続プランの article・「1ヶ月プラン」の箇条書き・「1ヶ月プランの報酬と精算」の節を出さず、
// 見出しは「関わり方」（「大きく2つ」ではない）。「継続プランは準備中です」式の提示も出さない。
it("継続プランの article・1ヶ月プランの箇条書き・報酬と精算の節を出さず、見出しは「関わり方」", async () => {
  await act(async () => root.render(<CoachAboutView />));
  const text = container.textContent ?? "";
  expect(container.querySelector("#coach-ways")?.textContent).toBe("関わり方");
  expect(text).not.toContain("大きく2つ");
  expect(container.querySelectorAll('section[aria-labelledby="coach-ways"] article')).toHaveLength(1);
  expect(container.querySelector('section[aria-labelledby="coach-ways"] h3')?.textContent).toBe("スタンダード");
  for (const word of ["継続プラン", "1ヶ月プラン", "対面プラン", "1ヶ月プランの報酬と精算", "準備中", "無料相談", "継続的な指導"]) expect(text).not.toContain(word);
  expect(text).toContain("コーチを募集しています。スタンダードは開始していて、申込みとチケットの購入ができます。");
  expect(text).toContain("動画や文章でのアドバイスを通じて、自分が対応できる形で活動できます。");
  expect(text).toContain("スタンダードは受付のオン・オフで調整できます。期限を過ぎると新規受付は自動でオフになります。届いた依頼は期限まで対応できます。");
});

// 本体 #1007 C5-3：metadata の description から継続プランへの言及を外す。
it("metadata の description はスタンダードの仕組みの紹介になっている", () => {
  expect(metadata.description).toBe("陸上の経験を、次の一歩を支えるアドバイスに。TrackTalkのコーチの役割と、スタンダードの仕組みを紹介します。");
});

it("継続プランに旧手順の文が残っていない", async () => {
  await act(async () => root.render(<CoachAboutView />));
  expect(container.textContent).not.toContain("プランを公開し");
  expect(container.textContent).not.toContain("承諾した後");
  expect(container.textContent).not.toContain("出品");
  expect(container.textContent).not.toContain("目標の大会に向けて、準備や調整をサポート");
  expect(container.textContent).not.toContain("プランに明記し");
  expect(container.textContent).not.toContain("対応できる内容と定員を自分で決めます");
});

it("スタンダードの案内は変わっていない", async () => {
  await act(async () => root.render(<CoachAboutView />));
  expect(container.textContent).toContain("受付をオンにすると、申込みが対応中の一覧に届きます");
  expect(container.textContent).toContain("届いてから24時間以内に、全コーチ共通の良い点・直す点・おすすめのドリル・コーチなら明日から何をするかと、申込みの時に決まった追加の項目のすべてに回答して返してください。");
});

// 本体 #1091 T7：回答の内容は固定4項目＋申込み時に決まった追加項目。添削項目・見本動画は申請制で、承認の後に切り替わる。
it("T7: 添削項目と見本動画は申請でき、承認の後、対応中の依頼が終わってから切り替わる", async () => {
  await act(async () => root.render(<CoachAboutView />));
  const text = container.textContent ?? "";
  expect(text).not.toContain("届いてから24時間以内に良い点・直す点・おすすめのドリルを返してください");
  expect(text).toContain("添削の追加の項目と、コーチのページに載せる見本動画は、コーチ画面から申請できます。運営の承認の後、料金と同じく対応中の依頼がすべて終わってから切り替わり、その後の新しい申込みから適用されます。");
});

// 本体 #1058：価格申請制に合わせ、料金はコーチごと・報酬はその70%。チケットの回だけ700円（一律700円とは書かない）。
it("料金はコーチごとに異なり、報酬はその依頼の料金の70%（チケットは700円）と書く", async () => {
  await act(async () => root.render(<CoachAboutView />));
  const text = container.textContent ?? "";
  expect(text).toContain("料金はコーチごとに異なります（コーチが自分で申請し、運営の承認の後に切り替わります）。コーチの報酬は、その依頼の料金の70％です。利用者がチケットで申し込んだスタンダードの報酬は700円です。");
  for (const word of ["どのコーチでも同じ", "1回1,000円", "（700円）", "報酬は同じ700円"]) expect(text).not.toContain(word);
});
