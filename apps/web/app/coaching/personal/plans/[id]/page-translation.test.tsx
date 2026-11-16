/** @vitest-environment jsdom */
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { installEnglish, setActiveLocale } from "@/lib/i18n";
import { en } from "@/lib/i18n-en";
import ApplyCoachingPlanPage from "./page";

const apiFetch = vi.fn();
vi.mock("@/lib/api", async (original) => ({ ...await original<typeof import("@/lib/api")>(), apiFetch: (...args: unknown[]) => apiFetch(...args) }));
vi.mock("next/navigation", () => ({ useParams: () => ({ id: "plan-1" }), useRouter: () => ({ push: vi.fn(), replace: vi.fn() }) }));
vi.mock("@/components/auth-provider", () => ({ useAuth: () => ({ session: { user: { id: "u1" } }, token: "tok", loading: false }) }));

let root: Root;
let container: HTMLDivElement;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  container = document.createElement("div"); document.body.appendChild(container); root = createRoot(container);
  apiFetch.mockReset();
  installEnglish(en);
  apiFetch.mockImplementation((path: string) => {
    if (path.startsWith("/v1/coaching/plans/")) return Promise.resolve({
      payments: true,
      coach: { id: "coach-1", nickname: "テストコーチ", nickname_en: "Test Coach", photo_url: "", tier: "coach", events: ["100m"] },
      plan: { id: "plan-1", coach_id: "coach-1", title: "プラン", kind: "period", description: "説明", duration_days: 30, price_yen: 10000, capacity: 3, published: true, service_type: "competition_support", video_reviews: 1, menu_updates: null, reply_formats: [], start_on_first_video: false },
    });
    return Promise.resolve({ birth_month: "2000-01", is_minor: false, guardian: null, push_devices: 0, demo: true });
  });
});
afterEach(async () => { await act(async () => root.unmount()); container.remove(); setActiveLocale("ja"); });

// 項目14: プラン詳細でも en では訳名になる。
it("プラン詳細のコーチ名", async () => {
  setActiveLocale("en");
  await act(async () => root.render(<ApplyCoachingPlanPage />));
  expect(container.textContent).toContain("Test Coach");
  expect(container.textContent).not.toContain("テストコーチ");
});
