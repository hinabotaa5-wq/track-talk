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
  plan_title: "1ヶ月プラン", coach: { nickname: "コーチA" }, requester: { nickname: "選手" }, video_reviews: 0, reply_formats: [], menu_updates: null, start_on_first_video: false, clock_started_at: null,
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
  await act(async () => root.render(<CoachingMessages contract={c} token="t" guardianToken={guardianToken} />));
  await act(async () => { await Promise.resolve(); });
};
const coachMsg = { id: "m1", sender_role: "coach", mine: false, sender_active: true, body: "よろしく", video_url: null, thumbnail_url: null, menu: { title: "1週目", period: "12/1〜12/7", note: "スタートを固める", days: [{ day: "月", content: "30m×6" }] }, created_at: "2026-12-01T00:00:00Z" };

it("期間中は24時間目安と、終わると見られなくなることを案内し、ダウンロードでき、メニューを期間と一言つきで見せる", async () => {
  apiFetch.mockResolvedValue({ messages: [coachMsg], view_closed: false, view_until: "2026-12-20T15:00:00Z" });
  await render({ ...base, messages_view_until: "2026-12-20T15:00:00Z" } as CoachingContract);
  const text = container.textContent!;
  expect(text).toContain("コーチからの返答は24時間以内を目安としています");
  expect(text).toContain("に契約が終わると、このやりとりは見られなくなります（選手・コーチ・保護者とも）");
  expect(text).toContain("終わる前に文章とメニューをダウンロードしてください");
  expect(text).toContain("やりとり（文章とメニュー）をダウンロード");
  expect(text).not.toContain("90日");
  expect(text).toContain("12/1〜12/7");
  expect(text).toContain("スタートを固める");
  expect(container.querySelector('textarea[aria-label="メッセージ"]')).not.toBeNull();
});

it("保護者のページは見るだけで、入力欄を出さない", async () => {
  apiFetch.mockResolvedValue({ messages: [coachMsg], view_closed: false, view_until: null });
  await render({ ...base, role: "guardian" } as CoachingContract, "g-token");
  expect(container.textContent).toContain("見ることだけができます（送信はできません）");
  expect(container.textContent).toContain("このやりとりは見られなくなります");
  expect(container.textContent).not.toContain("ダウンロード");
  expect(container.querySelector('textarea[aria-label="メッセージ"]')).toBeNull();
});

it("コーチにはメニュー作成のボタンと、返答の目安が出る", async () => {
  apiFetch.mockResolvedValue({ messages: [], view_closed: false, view_until: null });
  await render({ ...base, role: "coach", minor: false } as CoachingContract);
  expect(container.textContent).toContain("選手への返答は24時間以内を目安にしてください");
  expect([...container.querySelectorAll("button")].some((b) => b.textContent === "メニュー作成")).toBe(true);
});

it("終わったら見られず、ダウンロードもできない。通報・紛争の対応中は読むだけで残る", async () => {
  apiFetch.mockResolvedValue({ messages: [], view_closed: true, view_until: "2026-12-20T15:00:00Z" });
  await render({ ...base, status: "ended", messages_view_until: "2026-12-20T15:00:00Z" } as CoachingContract);
  expect(container.textContent).toContain("契約が終わったので、このやりとりは見られません");
  expect(container.textContent).not.toContain("ダウンロード");
  expect(container.querySelector('textarea[aria-label="メッセージ"]')).toBeNull();

  // 期間中の契約でも、終わる時刻を過ぎたら（終わった印が付く前でも）閉じて、入力欄を出さない。
  await act(async () => root.unmount());
  root = createRoot(container);
  await render(base);
  expect(container.textContent).toContain("契約が終わったので、このやりとりは見られません");
  expect(container.querySelector('textarea[aria-label="メッセージ"]')).toBeNull();

  apiFetch.mockResolvedValue({ messages: [{ ...coachMsg, video_purged: true }], view_closed: false, view_until: "2026-12-20T15:00:00Z" });
  await act(async () => root.unmount());
  root = createRoot(container);
  await render({ ...base, status: "refunded", messages_view_until: "2026-12-20T15:00:00Z" } as CoachingContract);
  expect(container.textContent).toContain("通報・紛争の対応中のため、対応が終わるまで読むことだけができます");
  expect(container.textContent).toContain("動画は保存期間（契約の終わりから30日）を過ぎたため削除しました");
  expect(container.querySelector('textarea[aria-label="メッセージ"]')).toBeNull();
});

it("終わった契約では評価フォームを出さない", async () => {
  apiFetch.mockResolvedValue({ messages: [coachMsg], view_closed: false, view_until: null });
  await render({ ...base, status: "ended", messages_view_until: "2026-12-20T15:00:00Z" } as CoachingContract);
  expect(container.textContent).toContain("期間が終わりました");
  expect(container.textContent).not.toContain("このパーソナル指導の評価");
  expect(container.querySelector('[role="radiogroup"]')).toBeNull();
});

it("消去済みのやりとりでは評価フォームを出さない", async () => {
  apiFetch.mockResolvedValue({ messages: [], view_closed: true, view_until: null });
  await render({ ...base, status: "ended", messages_purged_at: "2027-12-20T15:00:00Z" } as CoachingContract);
  expect(container.textContent).toContain("やりとり（文章と動画）は消しました");
  expect(container.textContent).not.toContain("このパーソナル指導の評価");
  expect(container.querySelector('[role="radiogroup"]')).toBeNull();
});
