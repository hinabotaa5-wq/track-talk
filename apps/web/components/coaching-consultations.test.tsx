/** @vitest-environment jsdom */
import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { ApiRequestError } from "@/lib/api";
import { canClarify, type Consultation, type ConsultationRoom } from "@/lib/coaching-consultations";
import { CoachingConsultations } from "./coaching-consultations";
const apiFetch = vi.fn();
const flags = vi.hoisted(() => ({ messaging: true, consultation: true }));
vi.mock("@/lib/features", async (original) => ({ ...await original<typeof import("@/lib/features")>(), get coachingMessagingEnabled() { return flags.messaging; }, get coachingConsultationEnabled() { return flags.consultation; } }));
vi.mock("next/link", () => ({ default: ({ href, children }: { href: string; children: ReactNode }) => <a href={href}>{children}</a> }));
vi.mock("@/components/confirm-dialog", () => ({ useConfirm: () => vi.fn(async () => true), reportConfirmOptions: (x: unknown) => x }));
vi.mock("@/lib/api", async (original) => ({ ...await original<typeof import("@/lib/api")>(), apiFetch: (...args: unknown[]) => apiFetch(...args) }));
const draft: Consultation = {
 id: "consultation", request_key: "saved-key", status: "draft", funding: "ticket", price_yen: 500, payment_state: "unpaid", minor: false, payer: "requester",
 question: "購入後に戻っても残る質問", answer: "", video_public_id: null, video_url: null, answer_video_url: null,
 clarify_body: null, clarification: null, clarify_requested_at: null, clarified_at: null, answered_at: null, respond_by: null, answer_by: null,
 expires_at: "2026-12-01T00:00:00Z", created_at: "2026-10-23T00:00:00Z", purged: false, hidden: false,
};
const room: ConsultationRoom = { consultations: [draft], role: "requester", funding: "ticket", price_yen: 500, can_purchase: true, block_reason: "", requires_video: false };
let root: Root; let container: HTMLDivElement;
beforeEach(() => { (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true; container = document.createElement("div"); document.body.appendChild(container); root = createRoot(container); apiFetch.mockReset(); flags.messaging = true; flags.consultation = true; });
afterEach(async () => { await act(async () => root.unmount()); container.remove(); });
async function click(text: string) { const b = [...container.querySelectorAll("button")].find((x) => x.textContent === text); if (!b) throw new Error(`missing ${text}`); await act(async () => b.click()); }
async function render(data: ConsultationRoom = room, guardianToken?: string, kind: "review" | "contract" = guardianToken ? "contract" : "review") { apiFetch.mockResolvedValue(data); await act(async () => root.render(<CoachingConsultations rootID="review-room" kind={kind} token={guardianToken ? null : "token"} guardianToken={guardianToken} />)); }
it("購入前に下書きを保存し、不足時には元の部屋に戻る購入リンクを表示する", async () => {
 await render();
 apiFetch.mockImplementation(async (path: string, options?: { method: string }) => { if (!options?.method) return room; if (path.endsWith("/submit")) throw new ApiRequestError(422, { error: { code: "insufficient_tickets", message: "チケットが不足しています" } }); return { consultation: draft }; });
 await click("質問する（チケット1枚）");
 const writes = apiFetch.mock.calls.filter(([, opt]) => opt?.method === "POST");
 expect(writes[0][0]).toBe("/v1/coaching/requests/review-room/consultations");
 expect(JSON.parse(writes[0][1].body)).toMatchObject({ request_key: "saved-key", body: draft.question });
 expect(writes[1][0]).toBe("/v1/coaching/consultations/consultation/submit");
 const link = container.querySelector<HTMLAnchorElement>('a[href*="tickets"]');
 expect(link?.getAttribute("href")).toBe("/me/coaching/tickets?return_to=%2Fcoaching%2Frequests%2Freview-room");
 expect(container.querySelector("textarea")?.value).toBe(draft.question);
});
it("購入から戻ると保存済みの下書きと動画を復元する", async () => {
 await render({ ...room, consultations: [{ ...draft, video_public_id: "coaching/saved" }] });
 expect(container.querySelector("textarea")?.value).toBe(draft.question);
 expect(container.textContent).toContain("保存済みの動画を添付します");
});
it("サポートプランの相談は、回答への無料確認と新しい有料相談を区別する", async () => {
 const c = { ...draft, status: "answered" as const, answer: "コーチの回答", answered_at: new Date().toISOString() };
 await render({ ...room, consultations: [c] }, undefined, "contract");
 expect(container.textContent).toContain("回答について確認する（1回無料）");
 expect(container.textContent).toContain("質問する（チケット1枚）");
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
it("未成年本人は、添削のあとの追加相談の支払いを開くたびに、保護者の許可のチェックが要る", async () => {
 await render({ ...room, funding: "cash", consultations: [{ ...draft, funding: "cash", minor: true, payer: "requester", status: "awaiting_payment" }] });
 expect(container.textContent).toContain("保護者に許可をもらいました");
 expect(container.textContent).toContain("未成年の方は、購入する前に保護者の許可を得てください。");
 expect(container.textContent).not.toContain("保護者に、この契約のページから");
 expect(button("お支払いへ")?.disabled).toBe(true);
 await check();
 expect(button("お支払いへ")?.disabled).toBe(false);
 apiFetch.mockImplementation(async (_path: string, options?: { method: string }) => options?.method ? {} : room);
 await click("お支払いへ");
 const call = apiFetch.mock.calls.find(([path]) => path === "/v1/coaching/consultations/consultation/checkout");
 expect(JSON.parse(call?.[1].body)).toEqual({ guardian_permission: true, guardian_permission_shown_at: expect.any(String), guardian_permission_checked_at: expect.any(String) });
});
it("未成年本人が新しい追加相談をその場で払うときは、チェックしないと送れない", async () => {
 const cashDraft = { ...draft, funding: "cash" as const, minor: true, price_yen: 600 };
 await render({ ...room, funding: "cash", minor: true, consultations: [cashDraft] });
 expect([...container.querySelectorAll("button")].find((b) => b.textContent?.startsWith("質問する（"))?.disabled).toBe(true);
 await check();
 apiFetch.mockImplementation(async (path: string, options?: { method: string }) => { if (!options?.method) return room; if (path.endsWith("/submit")) return {}; return { consultation: cashDraft }; });
 await act(async () => [...container.querySelectorAll("button")].find((b) => b.textContent?.startsWith("質問する（"))?.click());
 const submit = apiFetch.mock.calls.find(([path]) => path === "/v1/coaching/consultations/consultation/submit");
 expect(JSON.parse(submit?.[1].body)).toEqual({ guardian_permission: true, guardian_permission_shown_at: expect.any(String), guardian_permission_checked_at: expect.any(String) });
});
it("18歳以上には、保護者の許可のチェックを出さない", async () => {
 await render({ ...room, funding: "cash", consultations: [{ ...draft, funding: "cash", status: "awaiting_payment" }, { ...draft, id: "d2", funding: "cash" }] });
 expect(container.querySelector('input[type="checkbox"]')).toBeNull();
 expect(button("お支払いへ")?.disabled).toBe(false);
});
const payNotice = "現在、お支払いはクレジットカードのみとなります。PayPayでのお支払いにも対応予定です（時期は未定です）。対応が整い次第、この画面でご案内します。";
it("動画添削のあとの追加相談をその場で払うときは、カードのみの案内を出す", async () => {
 await render({ ...room, funding: "cash", consultations: [{ ...draft, funding: "cash", status: "awaiting_payment" }, { ...draft, id: "d2", funding: "cash" }] });
 expect(container.textContent?.split(payNotice).length).toBe(3);
});
it("チケットで払う追加相談とサポートプランの追加相談には、カードのみの案内を出さない", async () => {
 await render();
 expect(container.textContent).not.toContain(payNotice);
 await act(async () => root.unmount()); root = createRoot(container);
 await render({ ...room, funding: "cash", consultations: [{ ...draft, funding: "cash", status: "awaiting_payment" }, { ...draft, id: "d2", funding: "cash" }] }, undefined, "contract");
 expect(container.textContent).not.toContain(payNotice);
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
it("届出が済むまでは、サポートプランの新しい質問・確認・回答の欄を出さず、届いた内容だけを見せる", async () => {
 flags.messaging = false;
 const c = { ...draft, status: "answered" as const, answer: "コーチの回答", answered_at: new Date().toISOString() };
 await render({ ...room, consultations: [c] }, undefined, "contract");
 expect(container.textContent).toContain("コーチの回答");
 expect(container.textContent).toContain("メッセージのやりとりは準備中です");
 expect(container.textContent).not.toContain("回答について確認する（1回無料）");
 expect(container.textContent).not.toContain("質問する（チケット1枚）");
});
it("動画添削のあとの相談は一方通行で、回答への確認・補足の欄を出さず、過去の確認・補足は表示だけする", async () => {
 const c = { ...draft, status: "answered" as const, answer: "コーチの回答", answered_at: new Date().toISOString(), clarify_body: "以前の確認", clarification: "以前の補足" };
 await render({ ...room, can_purchase: false, block_reason: "この添削の追加相談は回答済みです。", consultations: [c, { ...draft, id: "c2", status: "answered" as const, answer: "別の回答", answered_at: new Date().toISOString() }] });
 expect(container.textContent).toContain("この添削について質問を1回送ると、担当コーチが1回回答します");
 expect(container.textContent).not.toContain("会話はこの部屋に残ります");
 expect(container.textContent).toContain("回答への確認：以前の確認");
 expect(container.textContent).toContain("補足：以前の補足");
 expect(container.textContent).not.toContain("回答について確認する（1回無料）");
 expect(container.querySelector("textarea")).toBeNull();
 expect(container.textContent).toContain("この添削の追加相談は回答済みです。");
});
it("動画添削のあとの相談には、コーチの補足の欄も出さない", async () => {
 await render({ ...room, role: "coach", can_purchase: false, consultations: [{ ...draft, status: "answered", answer: "回答", answered_at: new Date().toISOString(), clarify_body: "確認" }] });
 expect(container.querySelector("textarea")).toBeNull();
 expect(container.textContent).not.toContain("補足を送る");
});
it("動画添削のあとの相談は専用のゲートで出し分け、届出待ちのゲートには左右されない", async () => {
 flags.messaging = false;
 await render({ ...room, consultations: [] });
 expect(container.textContent).toContain("質問する（チケット1枚）");
 expect(container.textContent).not.toContain("メッセージのやりとりは準備中です");
 await act(async () => root.unmount()); root = createRoot(container);
 flags.messaging = true; flags.consultation = false;
 await render({ ...room, role: "coach", can_purchase: false, consultations: [{ ...draft, status: "in_review" }] });
 expect(container.textContent).toContain("追加相談の受付は準備中です");
 expect(container.textContent).not.toContain("回答を送る");
});
