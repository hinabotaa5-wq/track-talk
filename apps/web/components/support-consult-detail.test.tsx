/** @vitest-environment jsdom */
import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { SupportConsultDetail } from "./support-consult-detail";
import { emptyAnswers, type SupportConsult } from "@/lib/support-consult";

const apiFetch = vi.fn();
vi.mock("next/link", () => ({ default: ({ href, children }: { href: string; children: ReactNode }) => <a href={href}>{children}</a> }));
vi.mock("@/lib/api", async (original) => ({ ...await original<typeof import("@/lib/api")>(), apiFetch: (...args: unknown[]) => apiFetch(...args) }));

const proposal = {
  competition_name: "秋季記録会", support_start_date: "2026-11-20", competition_date: "2026-12-20", duration_days: 31,
  call_count: 1, call_minutes: 30, price_yen: 9000, fee_percent: 20,
  coach_receives_yen: 7200, content_note: "週1回の返答", answer_to_question: "週1回で十分です", valid_until: "2026-11-30T15:00:00Z", valid_until_date: "2026-11-30",
};
const base: SupportConsult = {
  id: "s1", status: "proposed", minor: false, event: "100m", answer_by: null, contract_id: null, created_at: "2026-11-09T00:00:00Z", updated_at: "2026-11-09T00:00:00Z",
  answers: { ...emptyAnswers(), goal_decided: true, competition_name: "秋季記録会", competition_date: "2026-12-20", main_worry: "スタートが遅い", question_to_coach: "週に何回送る？" },
  questions: [], proposal,
};
let root: Root;
let container: HTMLDivElement;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  container = document.createElement("div"); document.body.appendChild(container); root = createRoot(container);
  apiFetch.mockReset();
});
afterEach(async () => { await act(async () => root.unmount()); container.remove(); });
const render = (c: SupportConsult, role: "athlete" | "coach", onChange = vi.fn()) =>
  act(async () => root.render(<SupportConsultDetail consult={c} role={role} token="t" onChange={onChange} />));
const button = (text: string) => [...container.querySelectorAll("button")].find((b) => b.textContent?.includes(text)) as HTMLButtonElement | undefined;

it("選手は提案を1画面で確認できる（期間・内容・価格・開始日・有効期限）", async () => {
  await render(base, "athlete");
  const text = container.textContent!;
  expect(text).toContain("2026-11-20 〜 2026-12-20（目標大会の日まで・31日間）");
  expect(text).toContain("9,000円");
  expect(text).toContain("2026-11-30 の終わりまで");
  expect(text).toContain("週1回で十分です");
  expect(text).toContain("相談と提案は契約ではありません");
  expect(text).not.toContain("運営手数料");
});

it("同意にチェックを入れるまで申し込めず、申し込むと契約の状況へつながる", async () => {
  apiFetch.mockImplementation(async (path: string) => path.endsWith("/accept") ? { contract: { id: "k1" } } : { consult: { ...base, status: "accepted", contract_id: "k1" } });
  const onChange = vi.fn();
  await render(base, "athlete", onChange);
  const submit = container.querySelector('button[type="submit"]') as HTMLButtonElement;
  expect(submit.disabled).toBe(true);
  for (const box of container.querySelectorAll('input[type="checkbox"]')) await act(async () => (box as HTMLInputElement).click());
  expect(submit.disabled).toBe(false);
  await act(async () => container.querySelector("form")!.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true })));
  expect(apiFetch.mock.calls[0][0]).toBe("/v1/support-consults/s1/accept");
  expect(onChange).toHaveBeenCalledWith(expect.objectContaining({ status: "accepted", contract_id: "k1" }));
});

it("提案には『何が返ってくるか』の標準形（動画への返答は無制限）が出て、回数や形式の選択はない", async () => {
  await render(base, "athlete");
  expect(container.textContent).toContain("回数の制限なく受けられます");
  expect(container.textContent).not.toContain("動画への返答の回数");
});

it("未成年は契約を作らず、「保護者に提案を送る」だけができる", async () => {
  await render({ ...base, minor: true }, "athlete");
  expect(container.textContent).toContain("保護者に提案を送ります");
  expect(container.textContent).toContain("あなたが送っただけでは、契約もお支払いも起きません");
  const button = [...container.querySelectorAll("button")].find((b) => b.textContent === "保護者に提案を送る")!;
  expect(container.querySelector('button[type="submit"]')).toBeNull();
  await act(async () => button.click());
  expect(apiFetch.mock.calls[0][0]).toBe("/v1/support-consults/s1/send-to-guardian");
});

it("有効期限が過ぎた提案は申し込めない", async () => {
  await render({ ...base, proposal: { ...proposal, expired: true } }, "athlete");
  for (const box of container.querySelectorAll('input[type="checkbox"]')) await act(async () => (box as HTMLInputElement).click());
  expect((container.querySelector('button[type="submit"]') as HTMLButtonElement).disabled).toBe(true);
  expect(container.textContent).toContain("有効期限が過ぎています");
});

it("コーチの聞き直し中は、選手が答えを送れる", async () => {
  apiFetch.mockResolvedValue({ consult: { ...base, status: "requested" } });
  const c: SupportConsult = { ...base, status: "clarifying", proposal: null, questions: [{ id: "q1", question: "週末は練習できますか", answer: null, asked_at: "2026-11-09T01:00:00Z", answered_at: null }] };
  await render(c, "athlete");
  expect(container.textContent).toContain("週末は練習できますか");
  const ta = container.querySelector('textarea[aria-label="聞き直しへの答え"]') as HTMLTextAreaElement;
  expect(ta).not.toBeNull();
});

it("コーチには相談の回答が区分ごとに見え、手数料20%を引いた受け取りが提案の入力中に見える", async () => {
  await render({ ...base, status: "requested", proposal: null }, "coach");
  expect(container.textContent).toContain("相談フォームの回答");
  expect(container.textContent).toContain("提案を出す");
  expect(container.textContent).toContain("運営手数料は20%です");
  const price = container.querySelector('input[aria-label="価格"]') as HTMLInputElement;
  const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value")!.set!;
  await act(async () => { setter.call(price, "10000"); price.dispatchEvent(new Event("input", { bubbles: true })); });
  expect(container.textContent).toContain("あなたの受け取り: 8,000円");
  expect(container.textContent).toContain("選手の質問への回答「週に何回送る？」");
  expect(button("辞退する")).toBeDefined();
  expect(button("選手に聞き直す")).toBeDefined();
});

it("選手は提案前なら相談を取り下げられる", async () => {
  await render({ ...base, status: "requested", proposal: null }, "athlete");
  expect(button("相談を取り下げる")).toBeDefined();
  await render({ ...base, status: "declined", proposal: null }, "athlete");
  expect(button("相談を取り下げる")).toBeUndefined();
});

it("提案画面に、日割りの算定例・返金の時期と方法・中止延期の扱い・終了後のチャット閲覧期間が出る", async () => {
  await render(base, "athlete");
  const text = container.textContent!;
  expect(text).toContain("9,000円・31日間の契約を10日目に終えると、残り21日分の6,096円を返金します");
  expect(text).toContain("お支払いに使ったカードへ返金");
  expect(text).toContain("主催者が中止・延期を正式に発表した日で契約を終えます");
  expect(text).toContain("契約が終わった時点（期間の満了・途中での終了とも）で、選手・コーチ・保護者のいずれからも見られなくなります");
  expect(text).toContain("動画のファイルは契約の終わりから30日で削除します");
});
