/** @vitest-environment jsdom */
import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import type { ReviewRequest } from "@/lib/types";
import Page from "./page";

const mocks = vi.hoisted(() => ({ api: vi.fn(), router: { replace: vi.fn() } }));
vi.mock("next/navigation", () => ({ useRouter: () => mocks.router }));
vi.mock("next/link", () => ({ default: ({ href, children, className }: { href: string; children: ReactNode; className?: string }) => <a href={href} className={className}>{children}</a> }));
vi.mock("@/components/auth-provider", () => ({ useAuth: () => ({ token: "test", loading: false, session: { user: { id: "athlete" } } }) }));
vi.mock("@/lib/api", async (original) => ({ ...await original<typeof import("@/lib/api")>(), apiFetch: (...args: unknown[]) => mocks.api(...args) }));

const oneRequest = {
  id: "req-1",
  event: "100m",
  status: "in_review",
  created_at: "2026-11-17T00:00:00Z",
  due_at: "2026-11-18T00:00:00Z",
  video_url: null,
  thumbnail_url: null,
} as unknown as ReviewRequest;

let container: HTMLDivElement;
let root: Root;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  mocks.api.mockReset();
  mocks.router.replace.mockReset();
  container = document.createElement("div");
  document.body.appendChild(container);
  root = createRoot(container);
});
afterEach(async () => { await act(async () => root.unmount()); container.remove(); });
async function render() { await act(async () => root.render(<Page />)); }

it("依頼0件では案内文とコーチ一覧リンクを出し、新しく頼むと /coaching/new リンクは出さない", async () => {
  mocks.api.mockResolvedValue({ requests: [], meta: {} });
  await render();
  expect(container.querySelector("h1")?.textContent).toBe("スタンダードの依頼");
  expect(container.textContent).toContain("まだ依頼はありません。");
  expect(container.textContent).toContain("から依頼できます。");
  const coachList = container.querySelector('a[href="/coaching"]');
  expect(coachList?.textContent).toBe("コーチ一覧");
  expect(coachList?.className).toContain("text-blue-300");
  expect(coachList?.className).toContain("underline");
  expect(container.textContent).not.toContain("新しく頼む");
  expect(container.querySelector('a[href="/coaching/new"]')).toBeNull();
});

it("依頼が1件あるときは案内文も新しく頼むも出さず、依頼行は詳細へリンクする", async () => {
  mocks.api.mockResolvedValue({ requests: [oneRequest], meta: {} });
  await render();
  expect(container.querySelector("h1")?.textContent).toBe("スタンダードの依頼");
  expect(container.textContent).not.toContain("まだ依頼はありません。");
  expect(container.textContent).not.toContain("から依頼できます。");
  expect(container.textContent).not.toContain("新しく頼む");
  expect(container.querySelector('a[href="/coaching/new"]')).toBeNull();
  expect(container.querySelector('a[href="/coaching"]')).toBeNull();
  expect(container.querySelector('a[href="/coaching/requests/req-1"]')).not.toBeNull();
});
