/** @vitest-environment jsdom */
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { CoachingMessages } from "./coaching-messages";
import type { CoachingContract } from "@/lib/types";

const apiFetch = vi.fn();
vi.mock("@/lib/features", async (original) => ({ ...await original<typeof import("@/lib/features")>(), coachingMessagingEnabled: true }));
vi.mock("@/lib/api", async (original) => ({ ...await original<typeof import("@/lib/api")>(), apiFetch: (...args: unknown[]) => apiFetch(...args) }));
vi.mock("@/components/confirm-dialog", () => ({ useConfirm: () => async () => true, reportConfirmOptions: {} }));

const base = {
  id: "c1", status: "active", role: "requester", minor: true, ends_at: "2026-12-20T15:00:00Z", messages_purged_at: null, messages_view_until: null,
  plan_title: "大会までプラン", coach: { nickname: "コーチA" }, requester: { nickname: "選手" }, video_reviews: 0, reply_formats: [], menu_updates: null, start_on_first_video: false, clock_started_at: null,
} as unknown as CoachingContract;
let root: Root;
let container: HTMLDivElement;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  container = document.createElement("div"); document.body.appendChild(container); root = createRoot(container);
  apiFetch.mockReset();
});
afterEach(async () => { await act(async () => root.unmount()); container.remove(); });
const render = async (c: CoachingContract, guardianToken?: string) => {
  await act(async () => root.render(<CoachingMessages contract={c} token="t" guardianToken={guardianToken} onContractChange={vi.fn()} />));
  await act(async () => { await Promise.resolve(); });
};
const coachMsg = { id: "m1", sender_role: "coach", mine: false, sender_active: true, body: "よろしく", video_url: null, thumbnail_url: null, menu: { title: "1週目", period: "12/1〜12/7", note: "スタートを固める", days: [{ day: "月", content: "30m×6" }] }, created_at: "2026-12-01T00:00:00Z" };

it("期間中は24時間目安と、終了後90日の閲覧期間を案内し、メニューを期間と一言つきで見せる", async () => {
  apiFetch.mockResolvedValue({ messages: [coachMsg], view_closed: false, view_until: null });
  await render(base);
  const text = container.textContent!;
  expect(text).toContain("コーチからの返答は24時間以内を目安としています");
  expect(text).toContain("契約が終わった後も90日間は読むことができ");
  expect(text).toContain("12/1〜12/7");
  expect(text).toContain("スタートを固める");
  expect(container.querySelector('textarea[aria-label="メッセージ"]')).not.toBeNull();
});

it("保護者のページは見るだけで、入力欄を出さない", async () => {
  apiFetch.mockResolvedValue({ messages: [coachMsg], view_closed: false, view_until: null });
  await render({ ...base, role: "guardian" } as CoachingContract, "g-token");
  expect(container.textContent).toContain("見ることだけができます（送信はできません）");
  expect(container.querySelector('textarea[aria-label="メッセージ"]')).toBeNull();
});

it("コーチにはメニュー作成のボタンと、返答の目安が出る", async () => {
  apiFetch.mockResolvedValue({ messages: [], view_closed: false, view_until: null });
  await render({ ...base, role: "coach", minor: false } as CoachingContract);
  expect(container.textContent).toContain("選手への返答は24時間以内を目安にしてください");
  expect([...container.querySelectorAll("button")].some((b) => b.textContent === "メニュー作成")).toBe(true);
});

it("終わったあとは期限までの閲覧のみで、ダウンロードできる。90日を過ぎたら見られない", async () => {
  apiFetch.mockResolvedValue({ messages: [coachMsg], view_closed: false, view_until: "2027-03-20T15:00:00Z" });
  await render({ ...base, status: "ended", messages_view_until: "2027-03-20T15:00:00Z" } as CoachingContract);
  expect(container.textContent).toContain("まで読むことだけができ（送信はできません）");
  expect(container.textContent).toContain("やりとり（文章とメニュー）をダウンロード");
  expect(container.querySelector('textarea[aria-label="メッセージ"]')).toBeNull();

  apiFetch.mockResolvedValue({ messages: [], view_closed: true, view_until: "2027-03-20T15:00:00Z" });
  await act(async () => root.unmount());
  root = createRoot(container);
  await render({ ...base, status: "ended", messages_view_until: "2027-03-20T15:00:00Z" } as CoachingContract);
  expect(container.textContent).toContain("90日を過ぎたので、このやりとりは見られません");
  expect(container.textContent).not.toContain("ダウンロード");
});
