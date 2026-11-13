/** @vitest-environment jsdom */
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { installEnglish, setActiveLocale } from "@/lib/i18n";
import { en } from "@/lib/i18n-en";
import { rememberCoachDetailPreview } from "@/lib/coach-detail-preview";
import type { PublicCoach } from "@/lib/types";
import CoachDetailPage from "./page";

const mocks = vi.hoisted(() => ({
  api: vi.fn(),
  auth: { session: null, token: null as string | null, loading: false },
}));
vi.mock("@/lib/api", async (original) => ({ ...await original<typeof import("@/lib/api")>(), apiFetch: (...args: unknown[]) => mocks.api(...args) }));
vi.mock("@/components/auth-provider", () => ({ useAuth: () => mocks.auth }));
vi.mock("next/navigation", () => ({ useParams: () => ({ id: "c1" }) }));
vi.mock("next/link", () => ({ default: ({ href, children }: { href: string; children: React.ReactNode }) => <a href={href}>{children}</a> }));

const coach: PublicCoach = {
  id: "c1", nickname: "テストコーチ", photo_url: "", tier: "coach", events: ["100m", "200m"],
  best_record: "10秒00", achievements: "全国大会優勝\n県記録保持者", years: 10,
  sns_urls: ["https://www.instagram.com/testcoach/?utm_source=qr", "note-test"],
  hitokoto: "よろしくお願いします", rating_count: 0, rating_average: 0,
  service_types: ["video_review"], plans: [], support_consult: null,
};

let container: HTMLDivElement;
let root: Root;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  container = document.createElement("div"); document.body.appendChild(container); root = createRoot(container);
  mocks.api.mockReset();
  mocks.auth.session = null;
  mocks.auth.token = null;
  rememberCoachDetailPreview(coach, 0);
  mocks.api.mockImplementation(async (path: string) => {
    if (path === "/v1/coaches/c1") return coach;
    throw new Error(`unexpected ${path}`);
  });
});
afterEach(async () => { await act(async () => root.unmount()); container.remove(); setActiveLocale("ja"); });

async function render() {
  await act(async () => root.render(<CoachDetailPage />));
  await act(async () => {});
}

it("未ログインでは動画添削の節と依頼ボタンを有効なまま出し、受付状態を取りに行かない", async () => {
  await render();
  expect(container.textContent).toContain("動画添削");
  const link = container.querySelector('a[href="/coaching/new?coach=c1"]');
  expect(link?.textContent).toBe("動画添削を依頼する");
  expect(container.textContent).toContain("500円");
  expect(container.textContent).toContain("チケットも使えます");
  expect(container.textContent).not.toContain("現在は受付していません");
  expect(mocks.api.mock.calls).toEqual([["/v1/coaches/c1"]]);
});

it("ログイン済みで受付中ならボタンを有効にし、受付中でない場合は押せない見た目と注記にする", async () => {
  mocks.auth.token = "token";
  mocks.api.mockImplementation(async (path: string, init?: { token?: string | null }) => {
    if (path === "/v1/coaches/c1") return coach;
    if (path.startsWith("/v1/coaching/nominees")) {
      expect(init?.token).toBe("token");
      return { coaches: [] };
    }
    throw new Error(`unexpected ${path}`);
  });
  await render();
  expect(container.querySelector('a[href="/coaching/new?coach=c1"]')).toBeNull();
  expect(container.querySelector('span[aria-disabled="true"]')?.textContent).toBe("動画添削を依頼する");
  expect(container.textContent).toContain("現在は受付していません");
});

it("ログイン済みで受付中ならボタンを有効にする", async () => {
  mocks.auth.token = "token";
  mocks.api.mockImplementation(async (path: string) => {
    if (path === "/v1/coaches/c1") return coach;
    if (path.startsWith("/v1/coaching/nominees")) {
      return { coaches: [{ id: "c1", nickname: "テストコーチ", photo_url: "", events: ["100m"], years: 10, best_record: "", achievements: "" }] };
    }
    throw new Error(`unexpected ${path}`);
  });
  await render();
  expect(container.querySelector('a[href="/coaching/new?coach=c1"]')?.textContent).toBe("動画添削を依頼する");
  expect(container.textContent).not.toContain("現在は受付していません");
});

it("受付状態の取得に失敗してもボタンを有効なまま出す", async () => {
  mocks.auth.token = "token";
  mocks.api.mockImplementation(async (path: string) => {
    if (path === "/v1/coaches/c1") return coach;
    throw new Error("network down");
  });
  await render();
  expect(container.querySelector('a[href="/coaching/new?coach=c1"]')?.textContent).toBe("動画添削を依頼する");
});

it("プロフィールは種目チップ・目立つ記録・箇条書きの実績・短いSNSラベルで出す", async () => {
  await render();
  const chips = [...container.querySelectorAll("li")].filter((li) => ["100m", "200m"].includes(li.textContent ?? ""));
  expect(chips).toHaveLength(2);
  expect(container.textContent).toContain("全国大会優勝");
  expect(container.textContent).toContain("県記録保持者");
  const items = [...container.querySelectorAll("li")].filter((li) => ["全国大会優勝", "県記録保持者"].includes(li.textContent ?? ""));
  expect(items).toHaveLength(2);
  const sns = container.querySelector('a[href="https://www.instagram.com/testcoach/?utm_source=qr"]');
  expect(sns?.getAttribute("target")).toBe("_blank");
  expect(sns?.textContent).toContain("Instagram");
  expect(sns?.textContent).toContain("@testcoach");
  expect(sns?.textContent).not.toContain("utm_source");
  expect(container.textContent).toContain("note-test");
});

it("英語では依頼ボタンと注記を英語で出す", async () => {
  installEnglish(en);
  setActiveLocale("en");
  mocks.auth.token = "token";
  mocks.api.mockImplementation(async (path: string) => {
    if (path === "/v1/coaches/c1") return coach;
    if (path.startsWith("/v1/coaching/nominees")) return { coaches: [] };
    throw new Error(`unexpected ${path}`);
  });
  await render();
  expect(container.textContent).toContain("Request a video review");
  expect(container.textContent).toContain("Not accepting requests right now");
});
