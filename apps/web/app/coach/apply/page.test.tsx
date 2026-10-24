/** @vitest-environment jsdom */
import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import CoachApplyPage from "./page";
const apiFetch = vi.fn();
const session = {};
vi.mock("@/components/auth-provider", () => ({ useAuth: () => ({ session, profile: { nickname: "普段の名前" }, token: "test", loading: false }) }));
vi.mock("next/navigation", () => ({ useRouter: () => ({ replace: vi.fn() }) }));
vi.mock("next/link", () => ({ default: ({ href, children, ...props }: { href: string; children: ReactNode }) => <a href={href} {...props}>{children}</a> }));
vi.mock("@/lib/api", async (original) => ({ ...await original<typeof import("@/lib/api")>(), apiFetch: (...args: unknown[]) => apiFetch(...args) }));
vi.mock("@/components/coach-photo", () => ({ CoachPhotoForm: () => null, uploadCoachPhoto: vi.fn() }));
const application = { name: "コーチの名前", tier: "coach", events: ["100m"], best_record: "10秒95", achievements: "全国大会", sns_urls: ["https://example-club.jp/"], status: "pending", applied_at: "2026-10-24T00:00:00Z", photo_url: "photo", years: 8 };
let root: Root;
let container: HTMLDivElement;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  apiFetch.mockReset();
  container = document.createElement("div"); document.body.appendChild(container); root = createRoot(container);
});
afterEach(async () => { await act(async () => root.unmount()); container.remove(); });
it("名前を保存し、年数と公開説明文を外して専用の機能紹介へ案内する", async () => {
  apiFetch.mockResolvedValue({ application });
  await act(async () => root.render(<CoachApplyPage />));
  expect(container.querySelector<HTMLInputElement>('input[autocomplete="name"]')?.value).toBe("コーチの名前");
  expect(container.querySelector('input[type="number"]')).toBeNull();
  expect(container.textContent).not.toContain("競技年数");
  expect(container.textContent).not.toContain("承認されると、ニックネーム");
  expect(container.textContent).toContain("成績や指導経験");
  expect(container.querySelector('a[href="/coach/about"]')).not.toBeNull();
  await act(async () => container.querySelector("form")!.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true })));
  const submitted = apiFetch.mock.calls.find(([, options]) => options?.method === "POST");
  expect(JSON.parse(submitted![1].body)).toMatchObject({ name: "コーチの名前", sns_urls: ["https://example-club.jp/"] });
  expect(JSON.parse(submitted![1].body)).not.toHaveProperty("years");
});

it("SNSやホームページが空なら応募を送らず、入力を案内する", async () => {
  apiFetch.mockResolvedValue({ application: { ...application, sns_urls: [] } });
  await act(async () => root.render(<CoachApplyPage />));
  expect(container.querySelector('textarea[required]')).not.toBeNull();
  await act(async () => container.querySelector("form")!.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true })));
  expect(container.textContent).toContain("URLを1件以上入力してください");
  expect(apiFetch.mock.calls.some(([, options]) => options?.method === "POST")).toBe(false);
});
