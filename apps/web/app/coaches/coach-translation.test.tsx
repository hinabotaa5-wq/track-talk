/** @vitest-environment jsdom */
import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { installEnglish, setActiveLocale } from "@/lib/i18n";
import { en } from "@/lib/i18n-en";
import { getCoachDetailPreview } from "@/lib/coach-detail-preview";
import type { PublicCoach } from "@/lib/types";
import { CoachList } from "./coach-list";
import CoachDetailPage from "./[id]/page";
import { SupportPlanList } from "./support-plan-list";

const apiFetch = vi.fn();
vi.mock("@/lib/api", async (original) => ({ ...await original<typeof import("@/lib/api")>(), apiFetch: (...args: unknown[]) => apiFetch(...args) }));
vi.mock("next/navigation", () => ({ useParams: () => ({ id: "coach-1" }), useSearchParams: () => new URLSearchParams(), useRouter: () => ({ push: vi.fn() }) }));
vi.mock("next/link", () => ({ default: ({ href, children, onNavigate, ...props }: { href: string; children: ReactNode; onNavigate?: () => void }) => <a href={href} onClick={(event) => { event.preventDefault(); onNavigate?.(); }} {...props}>{children}</a> }));
vi.mock("@/lib/features", async (original) => ({ ...await original<typeof import("@/lib/features")>(), supportPlanListEnabled: true }));

const coach: PublicCoach = {
  id: "coach-1", nickname: "テストコーチ", nickname_en: "Test Coach", photo_url: "", tier: "coach", events: ["100m"],
  best_record: "100m 10秒95", best_record_en: "100m 10.95", achievements: "成績1行目\n成績2行目", achievements_en: "Result one\nResult two",
  years: 10, sns_urls: [], hitokoto: "よろしく", hitokoto_en: "Happy to help", rating_count: 0, rating_average: 0, service_types: [], plans: [], on_duty: true, country_code: "JP",
};

let root: Root;
let container: HTMLDivElement;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  container = document.createElement("div"); document.body.appendChild(container); root = createRoot(container);
  apiFetch.mockReset();
  installEnglish(en);
});
afterEach(async () => { await act(async () => root.unmount()); container.remove(); setActiveLocale("ja"); });

// 項目12: 一覧 en+訳あり→英語、en+訳なし→日本語、ja+訳あり→日本語。
it("一覧の名前とひとこと", async () => {
  setActiveLocale("en");
  apiFetch.mockResolvedValue({ coaches: [coach] });
  await act(async () => root.render(<CoachList />));
  expect(container.textContent).toContain("Test Coach");
  // 一覧カードにはひとことを出さない（本体 #932）。訳文も出ない。詳細のひとことは下の項目13で確かめる。
  expect(container.textContent).not.toContain("Happy to help");
  await act(async () => root.unmount()); root = createRoot(container);
  apiFetch.mockResolvedValue({ coaches: [{ ...coach, nickname_en: null, hitokoto_en: null }] });
  await act(async () => root.render(<CoachList />));
  expect(container.textContent).toContain("テストコーチ");
  await act(async () => root.unmount()); root = createRoot(container);
  setActiveLocale("ja");
  apiFetch.mockResolvedValue({ coaches: [coach] });
  await act(async () => root.render(<CoachList />));
  expect(container.textContent).toContain("テストコーチ");
  expect(container.textContent).not.toContain("Test Coach");
});

// 項目13: 詳細 en で4項目、改行維持、部分訳フォールバック。
it("詳細の名前・記録・成績・ひとこと", async () => {
  setActiveLocale("en");
  apiFetch.mockResolvedValue(coach);
  await act(async () => root.render(<CoachDetailPage />));
  expect(container.querySelector("h1")?.textContent).toBe("Test Coach");
  expect(container.textContent).toContain("100m 10.95");
  expect(container.textContent).toContain("Result one");
  expect(container.textContent).toContain("Result two");
  expect(container.textContent).toContain("Happy to help");
  await act(async () => root.unmount()); root = createRoot(container);
  apiFetch.mockResolvedValue({ ...coach, achievements_en: null, hitokoto_en: "" });
  await act(async () => root.render(<CoachDetailPage />));
  expect(container.querySelector("h1")?.textContent).toBe("Test Coach");
  expect(container.textContent).toContain("成績1行目");
  expect(container.textContent).toContain("よろしく");
});

// 項目14: サポート一覧の名前と、プレビュー経由の初回表示。
it("サポート一覧とプレビュー", async () => {
  setActiveLocale("en");
  const support = { id: "coach-1", nickname: "テストコーチ", nickname_en: "Test Coach", photo_url: "", events: ["100m"], kinds: ["competition_support"], competition_support: { events: ["100m"], description: "", price_min_yen: 5000, price_max_yen: 12000, reception: { status: "open", resume_on: null } } };
  apiFetch.mockResolvedValue({ coaches: [support] });
  await act(async () => root.render(<SupportPlanList />));
  expect(container.textContent).toContain("Test Coach");
  await act(async () => container.querySelector("li a")!.dispatchEvent(new MouseEvent("click", { bubbles: true, cancelable: true })));
  expect(getCoachDetailPreview("coach-1")?.nickname_en).toBe("Test Coach");
  await act(async () => root.unmount()); root = createRoot(container);
  // プレビューだけが先に描かれても英語名になる（API応答前）。
  apiFetch.mockReturnValue(new Promise(() => {}));
  await act(async () => root.render(<CoachDetailPage />));
  expect(container.querySelector("h1")?.textContent).toBe("Test Coach");
});
