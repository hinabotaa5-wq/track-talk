/** @vitest-environment jsdom */
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { CoachingMessages } from "./coaching-messages";
import type { CoachingContract } from "@/lib/types";

const apiFetch = vi.fn();
vi.mock("@/lib/features", async (original) => ({ ...await original<typeof import("@/lib/features")>(), coachingMessagingEnabled: true }));
vi.mock("@/lib/api", async (original) => ({ ...await original<typeof import("@/lib/api")>(), apiFetch: (...args: unknown[]) => apiFetch(...args) }));
vi.mock("@/components/confirm-dialog", () => ({ useConfirm: () => async () => true, reportConfirmOptions: (o: unknown) => o }));

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

it("終わっても評価フォームは出ない（星評価は廃止）", async () => {
  apiFetch.mockResolvedValue({ messages: [coachMsg], view_closed: false, view_until: null });
  await render({ ...base, status: "ended", rating: null } as CoachingContract);
  expect(container.querySelector('[role="radiogroup"]')).toBeNull();
  expect(container.querySelectorAll('button[role="radio"]')).toHaveLength(0);
  expect(container.textContent).not.toContain("このパーソナル指導の評価");
  expect(container.textContent).toContain("よろしく");
});

const reqMsg = { id: "m2", sender_role: "requester", mine: false, sender_active: true, body: "お願いします", video_url: null, thumbnail_url: null, menu: null, created_at: "2026-12-01T01:00:00Z" };
const mineMsg = { id: "m3", sender_role: "requester", mine: true, sender_active: true, body: "自分の文", video_url: null, thumbnail_url: null, menu: null, created_at: "2026-12-01T02:00:00Z" };

it("細目1: コーチには通報ボタンが一切出ない", async () => {
  apiFetch.mockResolvedValue({ messages: [reqMsg], view_closed: false, view_until: null });
  await render({ ...base, role: "coach", minor: false } as CoachingContract);
  expect(container.textContent).not.toContain("通報する");
  expect(container.textContent).not.toContain("通報しました");
  expect([...container.querySelectorAll("button")].some((b) => b.textContent?.includes("通報"))).toBe(false);
});

it("細目2: 選手・保護者には通報が出て、押すと従来どおり送って完了表示になる", async () => {
  apiFetch.mockImplementation(async (url: string) => {
    if (url === "/v1/reports") return {};
    return { messages: [coachMsg], view_closed: false, view_until: null };
  });
  await render(base);
  const btn = [...container.querySelectorAll("button")].find((b) => b.textContent === "通報する");
  expect(btn).toBeTruthy();
  await act(async () => { btn!.click(); await Promise.resolve(); await Promise.resolve(); });
  expect(apiFetch).toHaveBeenCalledWith("/v1/reports", expect.objectContaining({ method: "POST" }));
  expect(container.textContent).toContain("通報しました。運営が確認します。");

  await act(async () => root.unmount());
  root = createRoot(container);
  apiFetch.mockResolvedValue({ messages: [coachMsg], view_closed: false, view_until: null });
  await render({ ...base, role: "guardian" } as CoachingContract, "g-token");
  expect([...container.querySelectorAll("button")].some((b) => b.textContent === "通報する")).toBe(true);
});

it("細目3・4・5: 名前日時なし、日付区切り、吹き出し色、脇アバターなし、連続はグループ", async () => {
  const day2 = { ...coachMsg, id: "m4", body: "次の日", created_at: "2026-12-02T00:00:00Z" };
  const coach2 = { ...coachMsg, id: "m5", body: "連続2通目", created_at: "2026-12-01T01:30:00Z" };
  apiFetch.mockResolvedValue({ messages: [coachMsg, coach2, mineMsg, day2], view_closed: false, view_until: null });
  await render(base);
  const text = container.textContent!;
  expect(text).not.toContain("コーチA ·");
  expect(text).not.toMatch(/コーチA ·|選手 ·|あなた ·/);
  // 区切り: 12/1 と 12/2 に1つずつ。同日2通でも増えない。
  const seps = [...container.querySelectorAll('section[aria-label="やりとり"] p.text-center')].map((p) => p.textContent);
  const { formatDateTime } = await import("@/lib/format");
  expect(seps).toEqual([formatDateTime(coachMsg.created_at), formatDateTime(day2.created_at)]);
  // 色: 自分は紫グラデーション、相手は無地グレー。
  const mineBubble = container.querySelector("#message-m3 > div");
  const otherBubble = container.querySelector("#message-m1 > div");
  expect(mineBubble?.className).toContain("bg-gradient-to-r");
  expect(mineBubble?.className).toContain("from-violet-600");
  expect(mineBubble?.className).toContain("to-fuchsia-600");
  expect(otherBubble?.className).toContain("bg-neutral-800");
  expect(otherBubble?.className).not.toContain("border");
  // 脇アバターなし（一覧内に画像なし）。
  const section = container.querySelector('section[aria-label="やりとり"]');
  expect(section?.querySelectorAll("img")).toHaveLength(0);
  // グループ: 連続するコーチ2通は1グループ (space-y-0.5)、順序どおり。
  const groups = [...container.querySelectorAll("ul[data-message-group]")];
  expect(groups.map((g) => g.querySelectorAll("li").length)).toEqual([2, 1, 1]);
  expect(groups[0].className).toContain("space-y-0.5");
  expect(groups[0].textContent).toContain("よろしく");
  expect(groups[0].textContent).toContain("連続2通目");
  expect(text.indexOf("よろしく")).toBeLessThan(text.indexOf("連続2通目"));
});

it("細目7: 入力バーから送信でき、空では送れない。動画添付の入口がある", async () => {
  apiFetch.mockImplementation(async (url: string, opts?: { method?: string }) => {
    if (opts?.method === "POST") return { message: { ...mineMsg, id: "m9", body: "送った文" } };
    return { messages: [], view_closed: false, view_until: null };
  });
  await render({ ...base, role: "coach", minor: false } as CoachingContract);
  const submit = [...container.querySelectorAll("button")].find((b) => b.textContent === "送る") as HTMLButtonElement;
  expect(submit.disabled).toBe(true);
  expect(container.querySelector('[aria-label^="動画をつける"]')).not.toBeNull();
  expect(container.querySelector('form[class*="sticky"]')).not.toBeNull();
  const ta = container.querySelector('textarea[aria-label="メッセージ"]') as HTMLTextAreaElement;
  await act(async () => {
    Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, "value")!.set!.call(ta, "送った文");
    ta.dispatchEvent(new Event("input", { bubbles: true }));
  });
  expect(submit.disabled).toBe(false);
  await act(async () => { submit.click(); await Promise.resolve(); await Promise.resolve(); });
  expect(apiFetch).toHaveBeenCalledWith(expect.stringContaining("/messages"), expect.objectContaining({ method: "POST" }));
  expect(container.textContent).toContain("送った文");
});

it("細目11: 英語では浮動ボタンの aria-label が英語になる", async () => {
  const { installEnglish, setActiveLocale } = await import("@/lib/i18n");
  const { en } = await import("@/lib/i18n-en");
  installEnglish(en);
  setActiveLocale("en");
  (globalThis as Record<string, unknown>).IntersectionObserver = class {
    constructor(cb: (e: { isIntersecting: boolean }[]) => void) { cb([{ isIntersecting: false }]); }
    observe() {}
    disconnect() {}
    unobserve() {}
  };
  try {
    apiFetch.mockResolvedValue({ messages: [coachMsg], view_closed: false, view_until: null });
    await render(base);
    const btn = container.querySelector('button[aria-label="Go to the latest message"]');
    expect(btn).not.toBeNull();
    expect(container.querySelector('button[aria-label="最新のメッセージへ"]')).toBeNull();
  } finally {
    setActiveLocale("ja");
    delete (globalThis as Record<string, unknown>).IntersectionObserver;
  }
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
