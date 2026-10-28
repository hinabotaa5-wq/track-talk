/** @vitest-environment jsdom */
import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { ApiRequestError } from "@/lib/api";
import { canClarify, type Consultation, type ConsultationRoom } from "@/lib/coaching-consultations";
import { CoachingConsultations } from "./coaching-consultations";
const apiFetch = vi.fn();
const flags = vi.hoisted(() => ({ messaging: true }));
vi.mock("@/lib/features", async (original) => ({ ...await original<typeof import("@/lib/features")>(), get coachingMessagingEnabled() { return flags.messaging; } }));
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
beforeEach(() => { (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true; container = document.createElement("div"); document.body.appendChild(container); root = createRoot(container); apiFetch.mockReset(); flags.messaging = true; });
afterEach(async () => { await act(async () => root.unmount()); container.remove(); });
async function click(text: string) { const b = [...container.querySelectorAll("button")].find((x) => x.textContent === text); if (!b) throw new Error(`missing ${text}`); await act(async () => b.click()); }
async function render(data: ConsultationRoom = room, guardianToken?: string) { apiFetch.mockResolvedValue(data); await act(async () => root.render(<CoachingConsultations rootID="review-room" kind={guardianToken ? "contract" : "review"} token={guardianToken ? null : "token"} guardianToken={guardianToken} />)); }
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
it("回答への無料確認と新しい有料相談を区別する", async () => {
 const c = { ...draft, status: "answered" as const, answer: "コーチの回答", answered_at: new Date().toISOString() };
 await render({ ...room, consultations: [c] });
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
it("未成年本人は、添削のあとの追加相談を保護者の同意の範囲で自分で払える", async () => {
 await render({ ...room, funding: "cash", consultations: [{ ...draft, funding: "cash", minor: true, payer: "requester", status: "awaiting_payment" }] });
 expect([...container.querySelectorAll("button")].some((b) => b.textContent === "お支払いへ")).toBe(true);
 expect(container.textContent).not.toContain("保護者に、この契約のページから");
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
it("届出が済むまでは、新しい質問・確認・回答の欄を出さず、届いた内容だけを見せる", async () => {
 flags.messaging = false;
 const c = { ...draft, status: "answered" as const, answer: "コーチの回答", answered_at: new Date().toISOString() };
 await render({ ...room, consultations: [c] });
 expect(container.textContent).toContain("コーチの回答");
 expect(container.textContent).toContain("メッセージのやりとりは準備中です");
 expect(container.textContent).not.toContain("回答について確認する（1回無料）");
 expect(container.textContent).not.toContain("質問する（チケット1枚）");
});
