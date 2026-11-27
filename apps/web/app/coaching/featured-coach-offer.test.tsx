/** @vitest-environment jsdom */
import { act, createElement, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";

// 本体 #1086 補足 S5・W9：特集ページには「このコーチの添削で得られるもの」と見本動画の節を出さない
// （API がコーチ詳細に offer_items・demo_videos を返していても）。
const apiFetch = vi.fn();
vi.mock("@/lib/api", async (original) => ({ ...await original<typeof import("@/lib/api")>(), apiFetch: (...args: unknown[]) => apiFetch(...args) }));
vi.mock("@/components/auth-provider", () => ({ useAuth: () => ({ token: null, loading: false }) }));
vi.mock("next/navigation", () => ({ useSearchParams: () => new URLSearchParams(""), usePathname: () => "/coaching" }));
vi.mock("next/link", () => ({
  default: ({ children, href, ...props }: { children: ReactNode; href: string; onNavigate?: () => void }) => {
    delete (props as { onNavigate?: unknown }).onNavigate;
    return createElement("a", { href, ...props }, children);
  },
}));
const { FeaturedCoachView } = await import("./featured-coach-view");

const id = "2f6d0a8e-1111-4c2b-9a7e-3c332627b25f";
const coach = {
  id, nickname: "テスト太郎", nickname_en: null, photo_url: "https://img.example/c.jpg", tier: "coach",
  events: ["100m"], best_record: "100m 10.76", achievements: "", years: 5, sns_urls: [], hitokoto: "", on_duty: true, country_code: "JP",
  review_price_yen: 1000, plans: [], service_types: [], rating_count: 0, rating_average: 0,
  offer_items: ["スタート局面の詳しい分析"], demo_videos: [{ url: "https://example.com/demo.mp4" }],
};

let root: Root;
let container: HTMLDivElement;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  container = document.createElement("div"); document.body.appendChild(container); root = createRoot(container);
  apiFetch.mockReset();
  apiFetch.mockImplementation(async (path: string) => {
    if (path === `/v1/coaches/${id}`) return { ...coach };
    throw new Error(`unexpected ${path}`);
  });
});
afterEach(async () => { await act(async () => root.unmount()); container.remove(); });

it("W9: 特集ページには「得られるもの」節と見本動画節が出ない", async () => {
  await act(async () => root.render(<FeaturedCoachView coachIDs={[id]} />));
  await act(async () => { await Promise.resolve(); });
  expect(container.textContent).toContain("テスト太郎");
  expect(container.textContent).not.toContain("このコーチの添削で得られるもの");
  expect(container.textContent).not.toContain("スタート局面の詳しい分析");
  expect(container.querySelector('[data-testid="coach-offer-items"]')).toBeNull();
  expect(container.querySelector('video[src="https://example.com/demo.mp4"]')).toBeNull();
});
