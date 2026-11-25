/** @vitest-environment jsdom */
import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { canClarify, type Consultation, type ConsultationRoom } from "@/lib/coaching-consultations";
import { formatChatDateTime } from "@/lib/format";
import { CoachingConsultations } from "./coaching-consultations";
import { installEnglish, setActiveLocale } from "@/lib/i18n";
import { en } from "@/lib/i18n-en";
const apiFetch = vi.fn();
const flags = vi.hoisted(() => ({ messaging: true, consultation: true }));
vi.mock("@/lib/features", async (original) => ({ ...await original<typeof import("@/lib/features")>(), get coachingMessagingEnabled() { return flags.messaging; }, get coachingConsultationEnabled() { return flags.consultation; } }));
vi.mock("next/link", () => ({ default: ({ href, children }: { href: string; children: ReactNode }) => <a href={href}>{children}</a> }));
vi.mock("@/components/confirm-dialog", () => ({ useConfirm: () => vi.fn(async () => true), reportConfirmOptions: (x: unknown) => x }));
vi.mock("@/lib/api", async (original) => ({ ...await original<typeof import("@/lib/api")>(), apiFetch: (...args: unknown[]) => apiFetch(...args) }));
const draft: Consultation = {
 id: "consultation", request_key: "saved-key", status: "draft", funding: "cash", price_yen: 500, payment_state: "unpaid", minor: false, payer: "requester",
 question: "購入後に戻っても残る質問", answer: "", video_public_id: null, video_url: null, answer_video_url: null,
 clarify_body: null, clarification: null, clarify_requested_at: null, clarified_at: null, answered_at: null, respond_by: null, answer_by: null,
 expires_at: "2026-12-01T00:00:00Z", created_at: "2026-10-23T00:00:00Z", purged: false, hidden: false,
};
const room: ConsultationRoom = { consultations: [draft], role: "requester", funding: "cash", price_yen: 500, can_purchase: true, block_reason: "", requires_video: false };
let root: Root; let container: HTMLDivElement;
beforeEach(() => { (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true; container = document.createElement("div"); document.body.appendChild(container); root = createRoot(container); apiFetch.mockReset(); flags.messaging = true; flags.consultation = true; });
afterEach(async () => { await act(async () => root.unmount()); container.remove(); });
async function click(text: string) { const b = [...container.querySelectorAll("button")].find((x) => x.textContent === text); if (!b) throw new Error(`missing ${text}`); await act(async () => b.click()); }
async function render(data: ConsultationRoom = room, guardianToken?: string, kind: "review" | "contract" = guardianToken ? "contract" : "review") { apiFetch.mockResolvedValue(data); await act(async () => root.render(<CoachingConsultations rootID="review-room" kind={kind} token={guardianToken ? null : "token"} guardianToken={guardianToken} />)); }
it("継続プランの追加相談は、購入前に下書きを保存し、送信後は同じ部屋で再開できる", async () => {
 await render(room, undefined, "contract");
 apiFetch.mockImplementation(async (path: string, options?: { method: string }) => { if (!options?.method) return { ...room, consultations: [{ ...draft, question: draft.question }] }; return { consultation: draft }; });
 await click("質問する（500円）");
 const writes = apiFetch.mock.calls.filter(([, opt]) => opt?.method === "POST");
 expect(writes[0][0]).toBe("/v1/coaching/contracts/review-room/consultations");
 expect(JSON.parse(writes[0][1].body)).toMatchObject({ request_key: "saved-key", body: draft.question });
 expect(writes[1][0]).toBe("/v1/coaching/consultations/consultation/submit");
 expect(container.querySelector("textarea")?.value).toBe(draft.question);
});
it("継続プランの追加相談は、購入から戻ると保存済みの下書きと動画を復元する", async () => {
 await render({ ...room, consultations: [{ ...draft, video_public_id: "coaching/saved" }] }, undefined, "contract");
 expect(container.querySelector("textarea")?.value).toBe(draft.question);
 expect(container.textContent).toContain("保存済みの動画を添付します");
});
it("継続プランの相談は、回答への無料確認と新しい有料相談を区別する", async () => {
 const c = { ...draft, funding: "cash" as const, status: "answered" as const, answer: "コーチの回答", answered_at: new Date().toISOString() };
 await render({ ...room, funding: "cash", consultations: [c] }, undefined, "contract");
 expect(container.textContent).toContain("回答について確認する（1回無料）");
 expect(container.textContent).toContain("質問する（500円）");
 expect(canClarify(c, new Date(c.answered_at).getTime() + 7 * 86400000)).toBe(false);
 expect(canClarify({ ...c, clarify_body: "一度確認済み" })).toBe(false);
});
it("未成年本人には、保護者が払うパーソナル指導の個別決済を開くボタンを表示しない", async () => {
 await render({ ...room, can_purchase: false, funding: "cash", consultations: [{ ...draft, funding: "cash", minor: true, payer: "guardian", status: "awaiting_payment" }] });
 expect(container.textContent).toContain("保護者に");
 expect([...container.querySelectorAll("button")].some((b) => b.textContent === "お支払いへ")).toBe(false);
});
function button(text: string) { return [...container.querySelectorAll("button")].find((b) => b.textContent === text); }
async function check() { const box = container.querySelector<HTMLInputElement>('input[type="checkbox"]'); if (!box) throw new Error("missing checkbox"); await act(async () => box.click()); }
// 本体 #984: 1回プランの追加質問は販売終了。支払い前のものは支払いに進めず、取り下げだけができる。
it("販売終了前に作られた支払い前の追加質問は、支払いへ進めず取り下げだけができる", async () => {
 await render({ ...room, funding: "cash", consultations: [{ ...draft, funding: "cash", minor: true, payer: "requester", status: "awaiting_payment" }] });
 expect(button("お支払いへ")).toBeUndefined();
 expect(container.querySelector('input[type="checkbox"]')).toBeNull();
 expect(button("支払い前の質問を取り下げる")).toBeTruthy();
});
it("スタンダードの選手画面には、新しい追加質問の入力欄・価格・購入前チェック・支払い案内を出さない", async () => {
 await render({ ...room, funding: "cash", minor: true, consultations: [{ ...draft, id: "a1", status: "answered", answer: "回答", answered_at: "2026-11-14T02:00:00Z" }] });
 expect(container.querySelector("textarea")).toBeNull();
 expect(container.querySelector('input[type="checkbox"]')).toBeNull();
 expect(container.textContent).not.toContain("質問する（");
 expect(container.textContent).not.toContain("現在、お支払いはクレジットカードのみとなります。");
 expect(container.textContent).toContain("追加質問の販売は終了しました。");
});
it("保護者は閲覧と支払いができ、新しい有料質問は作れない", async () => {
 await render({ ...room, role: "guardian", funding: "cash", can_purchase: false, consultations: [{ ...draft, funding: "cash", minor: true, payer: "guardian", status: "awaiting_payment" }] }, "guardian-secret");
 expect(container.querySelector("textarea")).toBeNull();
 expect(container.textContent).toContain("お支払いへ");
 apiFetch.mockImplementation(async (path: string, options?: { method: string }) => options?.method ? {} : { ...room, role: "guardian", consultations: [] }); await click("お支払いへ");
 expect(apiFetch.mock.calls.some(([path]) => path === "/v1/guardian/guardian-secret/consultations/consultation/checkout")).toBe(true);
});
it("担当コーチには受諾と辞退を表示する", async () => {
 await render({ ...room, role: "coach", can_purchase: false, consultations: [{ ...draft, status: "offered" }] });
 expect(container.textContent).toContain("引き受ける");
 expect(container.textContent).toContain("辞退する");
 expect(container.querySelector("textarea")).toBeNull();
});
it("届出が済むまでは、継続プランの新しい質問・確認・回答の欄を出さず、届いた内容だけを見せる", async () => {
 flags.messaging = false;
 const c = { ...draft, status: "answered" as const, answer: "コーチの回答", answered_at: new Date().toISOString() };
 await render({ ...room, consultations: [c] }, undefined, "contract");
 expect(container.textContent).toContain("コーチの回答");
 expect(container.textContent).toContain("メッセージのやりとりは準備中です");
 expect(container.textContent).not.toContain("回答について確認する（1回無料）");
 expect(container.textContent).not.toContain("質問する（500円）");
});
it("フォーム添削のあとの相談は一方通行で、回答への確認・補足の欄を出さず、過去の確認・補足は表示だけする", async () => {
 const c = { ...draft, status: "answered" as const, answer: "コーチの回答", answered_at: new Date().toISOString(), clarify_body: "以前の確認", clarification: "以前の補足" };
 await render({ ...room, can_purchase: false, block_reason: "この添削の追加質問は回答済みです。", consultations: [c, { ...draft, id: "c2", status: "answered" as const, answer: "別の回答", answered_at: new Date().toISOString() }] });
 expect(container.textContent).not.toContain("この添削について質問を1回送ると、担当コーチが1回回答します");
 expect(container.textContent).not.toContain("会話はこの部屋に残ります");
 expect(container.textContent).toContain("回答への確認：以前の確認");
 expect(container.textContent).toContain("補足：以前の補足");
 expect(container.textContent).not.toContain("回答について確認する（1回無料）");
 expect(container.querySelector("textarea")).toBeNull();
 expect(container.textContent).not.toContain("この添削の追加質問は回答済みです。");
});
it("回答済みの追加質問はチャット風の吹き出しと終了状態を示し、入力欄を出さない", async () => {
 const c = {
  ...draft, status: "answered" as const, question: "質問文", answer: "コーチの回答",
  video_public_id: null, video_url: "https://example.com/q.mp4", answer_video_url: "https://example.com/a.mp4",
  answered_at: "2026-11-14T02:00:00Z",
 };
 await render({ ...room, can_purchase: false, block_reason: "この添削の追加質問は回答済みです。追加質問は1つの添削につき1回までです", consultations: [c] });
 expect(container.textContent).toContain("回答済み・この追加質問は終了しました");
 expect(container.textContent).not.toContain("あなたの質問");
 expect(container.textContent).toContain("コーチの回答");
 expect(container.querySelector("video")).not.toBeNull();
 expect(container.querySelector("textarea")).toBeNull();
 expect(container.textContent).not.toContain("この添削の追加質問は回答済みです。");
});
it("回答待ちの追加質問は質問の吹き出しを示し、追撃の入力欄を出さない", async () => {
 const c = { ...draft, status: "in_review" as const, question: "質問文", answer_by: "2026-11-17T02:00:00Z" };
 await render({ ...room, can_purchase: true, consultations: [c] });
 expect(container.textContent).toContain("回答待ち");
 expect(container.textContent).not.toContain("あなたの質問");
 expect(container.querySelector("textarea")).toBeNull();
});
it("フォーム添削のあとの相談には、コーチの補足の欄も出さない", async () => {
 await render({ ...room, role: "coach", can_purchase: false, consultations: [{ ...draft, status: "answered", answer: "回答", answered_at: new Date().toISOString(), clarify_body: "確認" }] });
 expect(container.querySelector("textarea")).toBeNull();
 expect(container.textContent).not.toContain("補足を送る");
});
it("フォーム添削のあとの相談は専用のゲートで出し分け、届出待ちのゲートには左右されない", async () => {
 flags.messaging = false;
 await render({ ...room, consultations: [] });
 // 販売終了（本体 #984）なので、購入済みが無い添削では枠ごと出さない。
 expect(container.textContent).toBe("");
 await act(async () => root.unmount()); root = createRoot(container);
 flags.messaging = true; flags.consultation = false;
 await render({ ...room, role: "coach", can_purchase: false, consultations: [{ ...draft, status: "in_review" }] });
 expect(container.textContent).toContain("追加質問の受付は準備中です");
 expect(container.textContent).not.toContain("回答を送る");
});
it("ゲートが閉じている間は、過去の相談がない添削に追加質問の欄を出さない", async () => {
 flags.consultation = false;
 await render({ ...room, consultations: [draft] });
 expect(container.textContent).not.toContain("追加質問");
 expect(container.textContent).not.toContain("追加相談");
 expect(container.querySelector("textarea")).toBeNull();
});
const contractThread = {
 ...draft, status: "answered" as const, question: "契約の質問", answer: "契約の回答", clarify_body: "契約の確認", clarification: "契約の補足",
 created_at: "2026-11-01T01:00:00Z", answered_at: "2026-11-02T02:00:00Z", clarify_requested_at: "2026-11-03T03:00:00Z", clarified_at: "2026-11-04T04:00:00Z",
};
function bubble(text: string) {
 const p = [...container.querySelectorAll("p")].find((x) => x.textContent === text);
 const row = p?.closest("div.flex");
 if (!row) throw new Error(`missing bubble ${text}`);
 return { side: row.classList.contains("justify-end") ? "right" : row.classList.contains("justify-start") ? "left" : "none", head: row.querySelector("time")?.textContent ?? "" };
}
it("継続プランの相談は、質問・確認を右、回答・補足を左の吹き出しで、ラベルなし・年なし日時で見せる", async () => {
 await render({ ...room, funding: "cash", consultations: [contractThread] }, undefined, "contract");
 expect(bubble("契約の質問")).toEqual({ side: "right", head: formatChatDateTime(contractThread.created_at) });
 expect(bubble("契約の回答")).toEqual({ side: "left", head: formatChatDateTime(contractThread.answered_at) });
 expect(bubble("契約の確認")).toEqual({ side: "right", head: formatChatDateTime(contractThread.clarify_requested_at) });
 expect(bubble("契約の補足")).toEqual({ side: "left", head: formatChatDateTime(contractThread.clarified_at) });
 expect(container.textContent).not.toContain("回答への確認：");
 expect(container.textContent).not.toContain("補足：契約の補足");
});
it("継続プランの相談を保護者が見るときは、話者ラベルなしで吹き出しを見せる", async () => {
 await render({ ...room, role: "guardian", funding: "cash", can_purchase: false, consultations: [contractThread] }, "guardian-secret");
 expect(bubble("契約の質問")).toMatchObject({ side: "right", head: formatChatDateTime(contractThread.created_at) });
 expect(bubble("契約の回答")).toMatchObject({ side: "left", head: formatChatDateTime(contractThread.answered_at) });
 expect(bubble("契約の確認")).toMatchObject({ side: "right", head: formatChatDateTime(contractThread.clarify_requested_at) });
 expect(bubble("契約の補足")).toMatchObject({ side: "left", head: formatChatDateTime(contractThread.clarified_at) });
 expect(container.textContent).not.toContain("あなたの");
});
it("継続プランの相談をコーチが見るときは、自分の回答・補足をラベルなしで右、選手の質問・確認を左に見せる", async () => {
 await render({ ...room, role: "coach", funding: "cash", can_purchase: false, consultations: [contractThread] }, undefined, "contract");
 expect(bubble("契約の質問")).toMatchObject({ side: "left", head: formatChatDateTime(contractThread.created_at) });
 expect(bubble("契約の回答")).toMatchObject({ side: "right", head: formatChatDateTime(contractThread.answered_at) });
 expect(bubble("契約の確認")).toMatchObject({ side: "left", head: formatChatDateTime(contractThread.clarify_requested_at) });
 expect(bubble("契約の補足")).toMatchObject({ side: "right", head: formatChatDateTime(contractThread.clarified_at) });
});
const reviewQuestion = { ...draft, question: "添削の質問", created_at: "2026-11-01T01:00:00Z" };
const reviewAnswered = { ...reviewQuestion, status: "answered" as const, answer: "添削の回答", answered_at: "2026-11-02T02:00:00Z" };
it("受諾待ちの追加質問をコーチが見るときは、選手の質問を左の吹き出しで見せる", async () => {
 await render({ ...room, role: "coach", can_purchase: false, consultations: [{ ...reviewQuestion, status: "offered" as const }] });
 expect(bubble("添削の質問")).toEqual({ side: "left", head: formatChatDateTime(reviewQuestion.created_at) });
});
it("回答済みの追加質問をコーチが見るときは、選手の質問を左、自分の回答を右の吹き出しで見せる", async () => {
 await render({ ...room, role: "coach", can_purchase: false, consultations: [reviewAnswered] });
 expect(bubble("添削の質問")).toMatchObject({ side: "left", head: formatChatDateTime(reviewQuestion.created_at) });
 expect(bubble("添削の回答")).toEqual({ side: "right", head: formatChatDateTime(reviewAnswered.answered_at) });
});
it("回答済みの追加質問を選手が見るときは、自分の質問を右、コーチの回答を左の吹き出しのまま見せる", async () => {
 await render({ ...room, can_purchase: false, consultations: [reviewAnswered] });
 expect(bubble("添削の質問")).toMatchObject({ side: "right", head: formatChatDateTime(reviewQuestion.created_at) });
 expect(bubble("添削の回答")).toMatchObject({ side: "left", head: formatChatDateTime(reviewAnswered.answered_at) });
});
// #918 コーチ画面（動画添削のあと）では、追加質問の選手向け説明を出さない。質問が届くまでは枠ごと出さない。
const reviewIntro = "この添削について質問を1回送ると、担当コーチが1回回答します。";
const reviewTerms = "コーチの受諾は24時間以内、受諾後の回答は12時間以内。辞退・未回答の期限切れは全額返金します。";
it("追加質問・追加相談の期限は日本語と英語の両方で受諾24時間・回答12時間", async () => {
 installEnglish(en);
 try {
  // 追加質問（review）は販売終了で選手向けの説明を出さないので、継続プランの追加相談だけを確かめる（本体 #984）。
  for (const kind of ["contract"] as const) {
   setActiveLocale("ja");
   await render(room, undefined, kind);
   expect(container.textContent).toContain("受諾は24時間以内、受諾後の回答は12時間以内");
   expect(container.textContent).not.toMatch(/48時間|72時間/);
   setActiveLocale("en");
   await render(room, undefined, kind);
   expect(container.textContent).toContain("within 24 hours");
   expect(container.textContent).toContain("within 12 hours");
   expect(container.textContent).not.toMatch(/48 hours|72 hours/);
  }
 } finally { setActiveLocale("ja"); }
});
const systemIntro = "この添削について、担当コーチに1回だけ質問できます。受諾は24時間以内、回答は受諾後12時間以内。辞退・期限切れは全額返金します。";
const coachRoom: ConsultationRoom = { ...room, role: "coach", can_purchase: false, consultations: [] };
it("A1: コーチの添削画面で追加質問がまだ無いときは、追加質問の枠を出さない", async () => {
 await render(coachRoom);
 expect(container.querySelector("section")).toBeNull();
 expect(container.textContent).not.toContain("追加質問");
 expect(container.textContent).not.toContain(reviewIntro);
});
it("A2: コーチの添削画面で選手の下書きしか無いときも、追加質問の枠を出さない", async () => {
 await render({ ...coachRoom, consultations: [draft] });
 expect(container.querySelector("section")).toBeNull();
 expect(container.textContent).toBe("");
});
it("A3: 届いた追加質問は、コーチが引き受け・辞退できる（説明は出さない）", async () => {
 await render({ ...coachRoom, consultations: [{ ...draft, status: "offered", question: "届いた質問", respond_by: "2026-11-20T00:00:00Z" }] });
 expect(container.textContent).toContain("届いた質問");
 expect(button("引き受ける")).toBeTruthy();
 expect(button("辞退する")).toBeTruthy();
 expect(container.textContent).not.toContain(reviewIntro);
 expect(container.textContent).not.toContain(reviewTerms);
});
it("A4: 引き受けた追加質問は、コーチが回答を送れる（説明は出さない）", async () => {
 await render({ ...coachRoom, consultations: [{ ...draft, status: "in_review", answer_by: "2026-11-20T00:00:00Z" }] });
 expect(button("回答を送る")).toBeTruthy();
 expect(container.querySelector("textarea")).not.toBeNull();
 expect(container.textContent).not.toContain(reviewIntro);
 expect(container.textContent).not.toContain(reviewTerms);
});
it("A5: 回答済みの追加質問は、質問と回答の吹き出しを見せる（説明は出さない）", async () => {
 await render({ ...coachRoom, consultations: [{ ...draft, status: "answered", question: "質問A5", answer: "回答A5", answered_at: "2026-11-14T02:00:00Z" }] });
 expect(bubble("質問A5").head).toBe(formatChatDateTime(draft.created_at));
 expect(bubble("回答A5").head).toBe("11/14 11:00");
 expect(container.textContent).not.toContain(reviewIntro);
 expect(container.textContent).not.toContain(reviewTerms);
});
it("B1: 選手の添削画面は、購入済みの追加質問が無ければ枠ごと出さない（販売終了・本体 #984）", async () => {
 await render();
 expect(container.querySelector("section")).toBeNull();
 expect(container.textContent).not.toContain(systemIntro);
 expect(container.textContent).not.toContain("質問する（500円）");
});
it("B2: 保護者の添削画面は、見出しと販売終了の案内を出し、購入時の説明は出さない", async () => {
 await render({ ...room, role: "guardian", can_purchase: false, consultations: [{ ...draft, status: "answered", answer: "回答", answered_at: "2026-11-14T02:00:00Z" }] }, "guardian-secret", "review");
 expect(container.querySelector("h2")?.textContent).toBe("追加質問");
 expect(container.textContent).toContain("追加質問の販売は終了しました。");
 expect(container.textContent).not.toContain(reviewIntro);
 expect(container.textContent).not.toContain(reviewTerms);
});
it("B3: 継続プランの相談をコーチが見るときは、説明を従来どおり出す", async () => {
 await render({ ...coachRoom, consultations: [] }, undefined, "contract");
 expect(container.querySelector("h2")?.textContent).toBe("追加相談");
 expect(container.textContent).toContain("新しい質問1件に、コーチが1回答えます。");
 expect(container.textContent).toContain("辞退・未回答の期限切れはチケット返却または全額返金します。");
});
it("B4: ゲートが閉じている間も、コーチの添削画面では過去の相談だけを出し、説明と回答欄は出さない", async () => {
 flags.consultation = false;
 await render({ ...coachRoom, consultations: [{ ...draft, status: "in_review" }] });
 expect(container.textContent).toContain("追加質問の受付は準備中です");
 expect(container.textContent).not.toContain("回答を送る");
 expect(container.textContent).not.toContain(reviewIntro);
});

it.each([{ consultations: [] }, { consultations: [draft] }])("送信前の選手には説明・入力欄を出さない（%j）", async ({ consultations }) => {
 await render({ ...room, consultations });
 expect(container.textContent).toBe("");
 expect(container.querySelector("textarea")).toBeNull();
});
it.each(["awaiting_payment", "offered", "in_review", "answered", "declined", "expired", "cancelled"] as const)("送信後（%s）はシステム説明を出さない", async (status) => {
 await render({ ...room, consultations: [{ ...draft, status }] });
 expect(container.textContent).not.toContain(systemIntro);
 expect(container.textContent).not.toContain(reviewIntro);
 expect([...container.querySelectorAll("p")].some((p) => p.textContent === "TrackTalk")).toBe(false);
});

it("購入不可で下書きもない場合は、購入できない理由も出さない", async () => {
 await render({ ...room, consultations: [], can_purchase: false, block_reason: "購入できません" });
 expect(container.textContent).not.toContain(systemIntro);
 expect(container.textContent).not.toContain("購入できません");
});
it.each(["review", "contract"] as const)("%s の両フォームに短い注意書き・枠とボタン付きのファイル入力を出す", async (kind) => {
 const assertFile = () => {
  const file = container.querySelector('input[type="file"]')!;
  expect(file.getAttribute("accept")).toBe("video/*");
  expect(file.closest("label")?.classList).toContain("border");
  expect(file.closest("label")?.classList).toContain("border-white/25");
  expect(file.classList).toContain("file:rounded-full");
  expect(container.textContent).toContain("外部の連絡先（URL・メールアドレス・電話番号・SNSのID）は送れません。");
  expect(container.textContent).not.toContain("規約で禁止されています");
 };
 // 選手の新しい質問の入力欄は継続プランだけ（追加質問は販売終了・本体 #984）。
 await render(room, undefined, kind);
 if (kind === "contract") assertFile();
 else expect(container.querySelector('input[type="file"]')).toBeNull();
 await act(async () => root.unmount()); root = createRoot(container);
 await render({ ...room, role: "coach", can_purchase: false, consultations: [{ ...draft, status: "in_review" }] }, undefined, kind);
 assertFile();
 expect(container.textContent).not.toContain(systemIntro);
});

it.each(["requester", "coach", "guardian"] as const)("%s の両kindの吹き出しは話者ラベルなし・年なし日時で、通報を維持する", async (role) => {
 for (const kind of ["review", "contract"] as const) {
  await render({ ...room, role, can_purchase: false, consultations: [contractThread] }, role === "guardian" ? "guardian-secret" : undefined, kind);
  const times = [...container.querySelectorAll("article time")];
  expect(times).toHaveLength(kind === "contract" ? 4 : 2);
  expect(times.map((t) => t.textContent)).toEqual(kind === "contract" ? ["11/1 10:00", "11/2 11:00", "11/3 12:00", "11/4 13:00"] : ["11/1 10:00", "11/2 11:00"]);
  expect(container.querySelector("article")?.textContent).not.toContain("2026");
  for (const label of ["あなたの質問", "選手の質問", "あなたの回答", "あなたの確認", "選手の確認", "あなたの補足"]) expect(container.textContent).not.toContain(label);
  expect([...container.querySelectorAll("button")].some((b) => b.textContent === (kind === "review" ? "この追加質問を通報する" : "この追加相談を通報する"))).toBe(true);
  expect(container.textContent).not.toContain(systemIntro);
  await act(async () => root.unmount()); root = createRoot(container);
 }
});

// 本体 #916 B2: 追加質問は、相手の文だけ訳文メイン＋原文併記、自分の文は原文だけ。
const overseas: Consultation = {
 ...draft, id: "overseas", status: "answered", question: "流しは何本がいいですか", question_en: "How many strides should I do?",
 answer: "Do 3 strides.", answer_ja: "流しを3本やりましょう。", answered_at: new Date().toISOString(),
 clarify_body: "距離も教えてください", clarify_body_en: "Please tell me the distance too", clarify_requested_at: new Date().toISOString(),
 clarification: "About 100m each.", clarification_ja: "1本100mくらいです。", clarified_at: new Date().toISOString(),
};
it("B2: 選手（ja）には、コーチの回答・補足が日本語メイン＋英語原文の併記で出て、自分の質問・確認は原文だけ", async () => {
 await render({ ...room, consultations: [overseas] }, undefined, "contract");
 const text = container.textContent!;
 expect(text).toContain("流しを3本やりましょう。");
 expect(text).toContain("1本100mくらいです。");
 expect([...container.querySelectorAll("p[lang='en']")].map((p) => p.textContent)).toEqual(["Do 3 strides.", "About 100m each."]);
 expect(text).toContain("流しは何本がいいですか");
 expect(text).not.toContain("How many strides should I do?");
 expect(container.querySelectorAll("p[lang='ja']")).toHaveLength(0);
});
it("B2: コーチ（en）には、選手の質問・確認が英語メイン＋日本語原文の併記で出て、自分の回答・補足は原文だけ", async () => {
 setActiveLocale("en");
 try {
  await render({ ...room, role: "coach", consultations: [overseas] }, undefined, "contract");
  const text = container.textContent!;
  expect(text).toContain("How many strides should I do?");
  expect(text).toContain("Please tell me the distance too");
  expect([...container.querySelectorAll("p[lang='ja']")].map((p) => p.textContent)).toEqual(["流しは何本がいいですか", "距離も教えてください"]);
  expect(text).toContain("Do 3 strides.");
  expect(text).not.toContain("流しを3本やりましょう。");
 } finally { setActiveLocale("ja"); }
});
it("B4: 訳の項目が null（国内コーチ）のときは、ja／en どちらでも原文だけ", async () => {
 const domestic = { ...overseas, question_en: null, answer_ja: null, clarify_body_en: null, clarification_ja: null };
 for (const locale of ["ja", "en"] as const) {
  setActiveLocale(locale);
  try {
   await render({ ...room, role: "coach", consultations: [domestic] }, undefined, "contract");
   expect(container.querySelectorAll("p[lang]")).toHaveLength(0);
   expect(container.textContent).toContain("流しは何本がいいですか");
  } finally { setActiveLocale("ja"); }
 }
});

it("#960: 追加相談の契約説明と下書き注意は全文一致のまま", async () => {
 await render(room, undefined, "contract");
 const paragraphs = [...container.querySelectorAll("p")];
 const first = "このプランの追加相談の契約の相手はコーチで、料金もコーチが決めます。1回の購入で相談できるのは、新しい質問1件とコーチの回答1件の1回限りです（回答への確認は7日以内に1回無料）。さらに相談するときは、改めて購入してください。";
 const terms = paragraphs.find((p) => p.textContent === first)!;
 expect(terms).toBeTruthy();
 expect([...terms.parentElement!.querySelectorAll("p")].map((p) => p.textContent)).toEqual([first, "代金を請求する権利はコーチにあり、TrackTalk の運営者はコーチから代理受領の権限を受けて代金を受け取ります。運営者へ支払った時点でコーチへの支払いは済み、コーチから同じ代金を重ねて請求されることはありません。"]);
 expect(paragraphs.some((p) => p.textContent === "購入前に下書きを7日間保存します。送信後は内容を変更できません。18歳未満の方のパーソナル指導の相談は、保護者が支払います。")).toBe(true);
});
it.each(["review", "contract"] as const)("%s の質問・回答の動画は幅260px・高さ300pxを上限にする", async (kind) => {
 const c = { ...draft, status: "answered" as const, answer: "回答", video_url: "https://example.com/q.mp4", answer_video_url: "https://example.com/a.mp4", answered_at: "2026-11-14T02:00:00Z" };
 await render({ ...room, can_purchase: false, consultations: [c] }, undefined, kind);
 const videos = [...container.querySelectorAll("video")];
 expect(videos.map((video) => video.getAttribute("src"))).toEqual([c.video_url, c.answer_video_url]);
 for (const video of videos) {
  expect(video.classList).toContain("max-w-[260px]");
  expect(video.classList).toContain("max-h-[300px]");
  expect(video.controls).toBe(true);
  expect(video.playsInline).toBe(true);
  expect(video.preload).toBe("metadata");
 }
});
it("動画のない相談では動画要素を表示しない", async () => {
 await render({ ...room, can_purchase: false, consultations: [{ ...draft, status: "answered", answer: "回答" }] });
 expect(container.querySelector("video")).toBeNull();
});
