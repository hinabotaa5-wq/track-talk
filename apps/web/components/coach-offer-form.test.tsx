/** @vitest-environment jsdom */
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import type { CoachOffers } from "@/lib/coach-offer";
import { CoachOfferSection } from "./coach-offer-form";

// 添削項目・見本動画の申請欄（本体 #1108）：申請内容は token と組で持ち、切り替え・履歴復帰で前の利用者の分を出さない（#1011 の方式）。
const apiFetch = vi.hoisted(() => vi.fn());
vi.mock("@/lib/api", async (original) => ({ ...await original<typeof import("@/lib/api")>(), apiFetch: (...args: unknown[]) => apiFetch(...args) }));

const offers = (items: string[]): CoachOffers => ({ items, demo_videos: [], pending: null, approved: null, max_items: 5, max_item_chars: 30, max_demo_videos: 3 });

let container: HTMLDivElement;
let root: Root;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  apiFetch.mockReset();
  container = document.createElement("div"); document.body.appendChild(container); root = createRoot(container);
});
afterEach(async () => { await act(async () => root.unmount()); container.remove(); });

async function render(token: string) {
  await act(async () => root.render(<CoachOfferSection token={token} />));
  await act(async () => { await Promise.resolve(); });
}

it("利用者を切り替えると、前の利用者の申請内容は新しい取得が終わるまで出さない（本体 #1108 T1）", async () => {
  apiFetch.mockResolvedValue(offers(["接地の分析"]));
  await render("tok-a");
  expect(container.textContent).toContain("接地の分析");
  let resolveNext!: (value: CoachOffers) => void;
  const next = new Promise<CoachOffers>((res) => { resolveNext = res; });
  apiFetch.mockReset();
  apiFetch.mockReturnValue(next);
  await render("tok-b");
  expect(container.textContent).not.toContain("接地の分析");
  await act(async () => { resolveNext(offers(["腕振りの分析"])); });
  await act(async () => { await Promise.resolve(); });
  expect(container.textContent).toContain("腕振りの分析");
});

it("履歴キャッシュから戻ったら（pageshow persisted）申請内容を取り直す（本体 #1108 T2）", async () => {
  apiFetch.mockResolvedValue(offers(["接地の分析"]));
  await render("tok-a");
  expect(container.textContent).toContain("接地の分析");
  apiFetch.mockClear();
  apiFetch.mockResolvedValue(offers(["腕振りの分析"]));
  const restored = new Event("pageshow");
  Object.defineProperty(restored, "persisted", { value: true });
  await act(async () => { window.dispatchEvent(restored); });
  await act(async () => { await Promise.resolve(); });
  expect(apiFetch).toHaveBeenCalledWith("/v1/coach/me/offers", { token: "tok-a" });
  expect(container.textContent).toContain("腕振りの分析");
});
