/** @vitest-environment jsdom */
import { act } from "react";
import { createRoot } from "react-dom/client";
import { expect, it, vi } from "vitest";
import { HomeFilters } from "./home-filters";

const track = vi.hoisted(() => vi.fn());
vi.mock("@/lib/analytics", () => ({ trackEvent: track }));
vi.mock("next/navigation", () => ({ useSearchParams: () => new URLSearchParams("q=private-search") }));

it("tracks a submitted search without its text and ignores clearing the search", async () => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  const container = document.createElement("div");
  document.body.appendChild(container);
  const root = createRoot(container);
  const pushState = vi.spyOn(window.history, "pushState").mockImplementation(() => {});
  try {
    await act(async () => root.render(<HomeFilters />));
    const form = container.querySelector("form")!;
    await act(async () => { form.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true })); });
    expect(track).toHaveBeenCalledExactlyOnceWith("search_submitted");
    await act(async () => { container.querySelector<HTMLButtonElement>('[aria-label="キーワードを消す"]')!.click(); });
    expect(track).toHaveBeenCalledTimes(1);
  } finally {
    pushState.mockRestore();
    await act(async () => root.unmount());
    container.remove();
  }
});
