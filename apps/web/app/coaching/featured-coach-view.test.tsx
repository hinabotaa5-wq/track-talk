/** @vitest-environment jsdom */
import { act, createElement, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { installEnglish, setActiveLocale } from "@/lib/i18n";
import { en } from "@/lib/i18n-en";

// コーチタブの特集ページ（本体 #985）。特集コーチ1人の詳細だけを取り、一覧・種目チップ・切り替えは出さない。
const apiFetch = vi.fn();
const auth = { token: null as string | null };
const flags = { supportList: false };
vi.mock("@/lib/api", async (original) => ({ ...await original<typeof import("@/lib/api")>(), apiFetch: (...args: unknown[]) => apiFetch(...args) }));
vi.mock("@/components/auth-provider", () => ({ useAuth: () => ({ token: auth.token, loading: false }) }));
vi.mock("@/lib/features", async (original) => ({ ...await original<typeof import("@/lib/features")>(), get supportPlanListEnabled() { return flags.supportList; } }));
vi.mock("next/link", () => ({
  default: ({ children, href, ...props }: { children: ReactNode; href: string }) => createElement("a", { href, ...props }, children),
}));
const { FeaturedCoachView, recordChips } = await import("./featured-coach-view");

const coach = {
  id: "2f6d0a8e-1111-4c2b-9a7e-3c332627b25f", nickname: "テスト太郎", nickname_en: null, photo_url: "https://img.example/c.jpg", tier: "coach",
  events: ["100m", "200m", "400m"], best_record: "100m 10.76／400m 47.54", achievements: "福岡のスプリントコーチ\n全国大会 優勝",
  years: 5, sns_urls: ["https://www.instagram.com/example_account/"], hitokoto: "一緒に速くなりましょう。", on_duty: true, country_code: "JP",
  plans: [], service_types: [], rating_count: 0, rating_average: 0,
};

let root: Root;
let container: HTMLDivElement;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  container = document.createElement("div"); document.body.appendChild(container); root = createRoot(container);
  auth.token = null; flags.supportList = false;
  apiFetch.mockReset();
  apiFetch.mockImplementation(async (path: string) => {
    if (path === "/v1/coaches/2f6d0a8e-1111-4c2b-9a7e-3c332627b25f") return { ...coach };
    throw new Error(`unexpected ${path}`);
  });
});
afterEach(async () => { await act(async () => root.unmount()); container.remove(); setActiveLocale("ja"); });
const render = async () => {
  await act(async () => root.render(<FeaturedCoachView coachID="2f6d0a8e-1111-4c2b-9a7e-3c332627b25f" />));
  await act(async () => { await Promise.resolve(); });
};
const link = (text: string) => Array.from(container.querySelectorAll("a")).find((a) => a.textContent?.includes(text));

it("ヒーロー・記録チップ・受付ピル・固定 CTA（¥1,000、特集コーチ指名で依頼へ）を出す", async () => {
  await render();
  expect(apiFetch.mock.calls.map(([path]) => path)).toEqual(["/v1/coaches/2f6d0a8e-1111-4c2b-9a7e-3c332627b25f"]);
  expect(container.querySelector("img")?.getAttribute("src")).toBe(coach.photo_url);
  expect(container.querySelector("h1")?.textContent).toBe("テスト太郎コーチ");
  expect(container.textContent).toContain("福岡のスプリントコーチ");
  const chips = Array.from(container.querySelectorAll('ul[aria-label="記録と対応種目"] li')).map((li) => li.textContent);
  expect(chips).toEqual(["100m 10.76", "400m 47.54", "対応種目 100m・200m・400m"]);
  expect(container.textContent).toContain("受付中");
  expect(link("動画添削を依頼する ¥1,000")?.getAttribute("href")).toBe("/coaching/new?coach=2f6d0a8e-1111-4c2b-9a7e-3c332627b25f&pay=card");
  expect(link("チケットを買う")?.getAttribute("href")).toBe("/me/coaching/tickets?return_to=%2Fcoaches%2F2f6d0a8e-1111-4c2b-9a7e-3c332627b25f");
  // 紹介バナーはヒーローの直後に新価格の文言で出す。
  const hero = container.querySelector('section[aria-label="コーチの紹介"]');
  expect(hero?.nextElementSibling?.getAttribute("href")).toBe("/referral");
  expect(hero?.nextElementSibling?.textContent).toContain("1回(1000円)無料！");
  // 回答の見本は実物の回答と同じ項目。
  const sample = Array.from(container.querySelectorAll('article[aria-label="回答の見本"] h3')).map((h) => h.textContent);
  expect(sample).toEqual(["良い点", "直す点", "おすすめのドリル", "ひとこと", "見本動画"]);
  expect(container.textContent).toContain("24時間以内に返却");
  expect(container.textContent).toContain("一緒に速くなりましょう。");
  expect(container.querySelector('a[href="https://www.instagram.com/example_account/"]')).not.toBeNull();
  expect(container.textContent).toContain("継続プランは準備中です");
  expect(container.textContent).toContain("よくある質問");
  // 一覧の部品（種目チップの絞り込み・切り替え）は出さない。
  expect(container.textContent).not.toContain("1回プラン");
  expect(container.querySelector('[role="tablist"], [role="tabpanel"]')).toBeNull();
});

it("受付停止中は、ピルと CTA を停止表示にし、依頼へ進めない", async () => {
  apiFetch.mockImplementation(async () => ({ ...coach, on_duty: false }));
  await render();
  expect(container.textContent).toContain("受付停止中");
  expect(container.textContent).toContain("現在は受付を停止しています");
  expect(link("動画添削を依頼する")).toBeUndefined();
  expect(container.querySelector('[aria-disabled="true"]')?.textContent).toBe("動画添削を依頼する ¥1,000");
});

it("チケットの新規販売を止めている間は「チケットを買う」を出さない", async () => {
  auth.token = "tok";
  apiFetch.mockImplementation(async (path: string) => (path === "/v1/me/coaching/tickets" ? { can_purchase: false, balances: [] } : { ...coach }));
  await render();
  expect(link("チケットを買う")).toBeUndefined();
});

it("特集コーチが非公開なら、ほかのコーチを出さずに非公開と表示する", async () => {
  const { ApiRequestError } = await import("@/lib/api");
  apiFetch.mockImplementation(async () => { throw new ApiRequestError(404, { error: { code: "coach_not_public", message: "x" } }); });
  await render();
  expect(container.textContent).toContain("このコーチは現在公開されていません。");
});

it("英語表示では固定文を英語で出す", async () => {
  installEnglish(en);
  setActiveLocale("en");
  await render();
  expect(container.textContent).toContain("One running video shows you what to fix next.");
  expect(container.textContent).toContain("Request video feedback ¥1,000");
  expect(container.textContent).toContain("What feedback you get");
  expect(container.textContent).toContain("FAQ");
  expect(container.textContent).not.toContain("よくある質問");
});

it("記録の欄を区切りでチップに分ける", () => {
  expect(recordChips("100m 10.76、400m 47.54\n200m 21.9")).toEqual(["100m 10.76", "400m 47.54", "200m 21.9"]);
  expect(recordChips(null)).toEqual([]);
});
