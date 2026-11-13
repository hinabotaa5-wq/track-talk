/** @vitest-environment jsdom */
import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import CoachProfilePage from "./page";
const apiFetch = vi.fn();
const session = {};
vi.mock("@/components/auth-provider", () => ({ useAuth: () => ({ session, profile: { nickname: "普段の名前" }, token: "test", loading: false }) }));
vi.mock("next/navigation", () => ({ useRouter: () => ({ replace: vi.fn() }) }));
vi.mock("next/link", () => ({ default: ({ href, children, ...props }: { href: string; children: ReactNode }) => <a href={href} {...props}>{children}</a> }));
vi.mock("@/lib/api", async (original) => ({ ...await original<typeof import("@/lib/api")>(), apiFetch: (...args: unknown[]) => apiFetch(...args) }));
vi.mock("@/lib/features", () => ({ coachingEnabled: true }));
vi.mock("@/components/coach-photo", () => ({ CoachPhotoForm: () => null }));
const application = { name: "コーチの名前", tier: "coach", events: ["100m"], best_record: "10秒95", achievements: "全国大会", sns_urls: ["@tracktalk.office"], hitokoto: "よろしくお願いします", status: "approved", applied_at: "2026-10-24T00:00:00Z", photo_url: "photo", years: 8 };
let root: Root;
let container: HTMLDivElement;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  apiFetch.mockReset();
  container = document.createElement("div"); document.body.appendChild(container); root = createRoot(container);
});
afterEach(async () => { await act(async () => root.unmount()); container.remove(); });

it("承認済みでは現在値入りの編集フォームが出て保存できる", async () => {
  apiFetch.mockImplementation((path: string, options?: { method?: string }) => {
    if (options?.method === "PUT") return Promise.resolve({ application: { ...application, best_record: "10秒94" } });
    return Promise.resolve({ application });
  });
  await act(async () => root.render(<CoachProfilePage />));
  expect(container.querySelector<HTMLInputElement>('input[autocomplete="name"]')?.value).toBe("コーチの名前");
  expect(container.querySelector<HTMLInputElement>('input[maxlength="100"]')?.value).toBe("よろしくお願いします");
  expect(container.textContent).toContain("選手へのひとことを入力できます（任意・100文字以内）");
  expect(container.querySelector("form")).not.toBeNull();
  await act(async () => container.querySelector("form")!.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true })));
  const put = apiFetch.mock.calls.find(([, options]) => options?.method === "PUT");
  expect(put?.[0]).toBe("/v1/coach/me/profile");
  expect(JSON.parse(put![1].body)).toMatchObject({ name: "コーチの名前", hitokoto: "よろしくお願いします", sns_urls: ["@tracktalk.office"] });
  expect(container.textContent).toContain("プロフィールを更新しました");
});

it("承認済み以外では編集フォームが出ない", async () => {
  apiFetch.mockResolvedValue({ application: { ...application, status: "pending", hitokoto: "" } });
  await act(async () => root.render(<CoachProfilePage />));
  expect(container.querySelector("form")).toBeNull();
  expect(container.textContent).not.toContain("プロフィールを保存する");
});
