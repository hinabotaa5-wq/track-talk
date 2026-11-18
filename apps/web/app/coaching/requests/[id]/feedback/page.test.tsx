/** @vitest-environment jsdom */
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { ApiRequestError } from "@/lib/api";
import { loadEnglish, setActiveLocale } from "@/lib/i18n";
import { en } from "@/lib/i18n-en";
import { Header } from "@/components/header";
import { LeaveGuardProvider } from "@/components/leave-guard";
import FeedbackPage from "./page";

const { router, auth, apiFetch } = vi.hoisted(() => ({
  router: { replace: vi.fn(), push: vi.fn(), back: vi.fn() },
  auth: { session: { user: { id: "athlete" } }, token: "token", loading: false },
  apiFetch: vi.fn(),
}));
vi.mock("next/navigation", () => ({
  useParams: () => ({ id: "review-1" }),
  usePathname: () => "/coaching/requests/review-1/feedback",
  useRouter: () => router,
}));
vi.mock("@/components/auth-provider", () => ({ useAuth: () => auth }));
vi.mock("@/lib/api", async (original) => ({ ...await original<typeof import("@/lib/api")>(), apiFetch }));
const label = "この添削はどうでしたか（任意）";
const notice = "感想は匿名で、7日後にコーチへ届きます。どの添削への感想かはコーチには分かりません。";
const request = { id: "review-1", status: "returned", response: {}, feedback_submitted: false };
let container: HTMLDivElement;
let root: Root;
beforeEach(() => {
  vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
  vi.clearAllMocks();
  setActiveLocale("ja");
  apiFetch.mockResolvedValue(request);
  auth.loading = false;
  container = document.createElement("div"); document.body.append(container); root = createRoot(container);
});
afterEach(async () => { await act(async () => root.unmount()); container.remove(); setActiveLocale("ja"); vi.unstubAllGlobals(); });
async function render() { await act(async () => root.render(<LeaveGuardProvider><Header /><FeedbackPage /></LeaveGuardProvider>)); }
function textarea() { return container.querySelector<HTMLTextAreaElement>("textarea"); }
function send() { return [...container.querySelectorAll("button")].find(b => b.textContent === "感想を送る")!; }
async function input(value: string) {
  await act(async () => {
    Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, "value")!.set!.call(textarea(), value);
    textarea()!.dispatchEvent(new Event("input", { bubbles: true }));
  });
}
async function back() { await act(async () => container.querySelector<HTMLButtonElement>('button[aria-label="戻る"]')!.click()); }

it.each(["returned", "clarifying"])("B1: %s は既存フォームを表示する", async status => {
  apiFetch.mockResolvedValue({ ...request, status });
  await render();
  expect(apiFetch).toHaveBeenCalledWith("/v1/coaching/requests/review-1", { token: "token" });
  expect(container.textContent).toContain(label);
  expect(container.textContent).toContain(notice);
  expect(textarea()?.maxLength).toBe(500);
  expect(send().disabled).toBe(true);
  expect(router.replace).not.toHaveBeenCalled();
});
it("B1: 既存の英訳で表示する", async () => {
  await loadEnglish(); setActiveLocale("en"); await render();
  for (const key of [label, notice, "感想を送る"]) expect(container.textContent).toContain(en[key]);
  expect(textarea()?.getAttribute("aria-label")).toBe(en[label]);
  expect(container.textContent).not.toContain(label);
});
it("B2/D2: 空白は送信不可、既存APIに送信後は完了文だけになり、戻る先は一覧", async () => {
  await render(); await input("   \n  "); expect(send().disabled).toBe(true);
  await input("  分かりやすかったです  ");
  await act(async () => send().click());
  expect(apiFetch).toHaveBeenCalledWith("/v1/coaching/requests/review-1/feedback", { method: "POST", token: "token", body: JSON.stringify({ body: "分かりやすかったです" }) });
  expect(textarea()).toBeNull();
  expect(container.textContent).toBe("感想を送りました");
  await back(); expect(router.replace).toHaveBeenLastCalledWith("/coaching"); expect(router.back).not.toHaveBeenCalled();
});
it.each([new Error("network"), new ApiRequestError(500, "送信エラー")])("B2: 送信失敗はフォームを残す %s", async error => {
  await render(); await input("感想"); apiFetch.mockRejectedValueOnce(error);
  await act(async () => send().click());
  expect(textarea()?.value).toBe("感想");
  expect(container.textContent).toContain(error instanceof ApiRequestError ? error.message : "送信に失敗しました");
  expect(send().disabled).toBe(false);
});
it.each([
  { feedback_submitted: true }, { response: null },
  ...["awaiting_payment", "queued", "offered", "in_review", "needs_info", "expired"].map(status => ({ status })),
])("B3: 条件外ではフォームなしで一覧へ replace %j", async extra => {
  apiFetch.mockResolvedValue({ ...request, ...extra }); await render();
  expect(textarea()).toBeNull(); expect(router.replace).toHaveBeenCalledWith("/coaching");
});
it.each([403, 404])("B3: 他人または存在しない依頼は一覧へ %i", async status => {
  apiFetch.mockRejectedValue(new ApiRequestError(status, "依頼が見つかりません")); await render();
  expect(textarea()).toBeNull(); expect(router.replace).toHaveBeenCalledWith("/coaching");
});
it("読み込み中は既存 PageLoading を表示し、フォームを出さない", async () => {
  apiFetch.mockReturnValue(new Promise(() => {})); await render();
  expect(textarea()).toBeNull(); expect(container.querySelector('[role="status"]')).not.toBeNull();
});
it("D1: チャットから push で来ても送信前の戻る先はコーチ一覧", async () => {
  window.history.pushState({}, "", "/coaching/requests/review-1");
  window.history.pushState({}, "", "/coaching/requests/review-1/feedback");
  await render(); await back();
  expect(router.replace).toHaveBeenCalledWith("/coaching"); expect(router.back).not.toHaveBeenCalled(); expect(router.push).not.toHaveBeenCalled();
});
