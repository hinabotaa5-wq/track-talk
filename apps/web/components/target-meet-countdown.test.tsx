/** @vitest-environment jsdom */
import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";

const fetch = vi.hoisted(() => vi.fn());
vi.mock("@/lib/api", () => ({ apiFetch: fetch }));
vi.mock("next/link", () => ({ default: ({ children, href }: { children: ReactNode; href: string }) => <a href={href}>{children}</a> }));

let root: Root, container: HTMLDivElement;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  vi.resetModules();
  vi.useFakeTimers({ toFake: ["Date"] });
  vi.setSystemTime(new Date("2026-10-27T03:00:00Z"));
  fetch.mockReset();
  container = document.createElement("div");
  document.body.appendChild(container);
  root = createRoot(container);
});
afterEach(async () => {
  await act(async () => root.unmount());
  container.remove();
  vi.useRealTimers();
});

async function render(token: string | null) {
  const { TargetMeetCountdown } = await import("./target-meet-countdown");
  await act(async () => root.render(<TargetMeetCountdown token={token} />));
}

it("セットした大会まであと何日かを出し、タップで大会一覧へ", async () => {
  fetch.mockResolvedValue({ target_meet: { source: "competition", ref: "x", name: "県選手権", starts_on: "2026-11-03", ends_on: null } });
  await render("t");
  expect(fetch).toHaveBeenCalledWith("/v1/me/target-meet", { token: "t" });
  expect(container.textContent).toContain("県選手権まであと 7 日");
  expect(container.querySelector("a")!.getAttribute("href")).toBe("/competitions/regional");
});

it("セットしていなければ + で都道府県の大会一覧へ", async () => {
  fetch.mockResolvedValue({ target_meet: null });
  await render("t");
  expect(container.textContent).toContain("目標の大会をセット");
  expect(container.querySelector("a")!.getAttribute("href")).toBe("/competitions/regional");
});

it("未ログインでも + を出し、API は呼ばない", async () => {
  await render(null);
  expect(fetch).not.toHaveBeenCalled();
  expect(container.textContent).toContain("目標の大会をセット");
});
