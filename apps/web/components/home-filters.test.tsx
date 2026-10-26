/** @vitest-environment jsdom */
import { act } from "react";
import { createRoot } from "react-dom/client";
import { expect, it, vi } from "vitest";
import { HomeFilters } from "./home-filters";

const track = vi.hoisted(() => vi.fn());
vi.mock("@/lib/analytics", () => ({ trackEvent: track }));
const search = vi.hoisted(() => ({ query: "q=private-search" }));
vi.mock("next/navigation", () => ({ useSearchParams: () => new URLSearchParams(search.query) }));

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

async function scrollTo(y: number) {
  Object.defineProperty(window, "scrollY", { configurable: true, value: y });
  await act(async () => {
    window.dispatchEvent(new Event("scroll"));
    await new Promise((resolve) => requestAnimationFrame(resolve));
  });
}

it.each([["recruitment", true], ["review", true], ["new", false], ["popular", false]])("%s では下へのスクロールでタブを引っ込める: %s", async (sort, hides) => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  search.query = `sort=${sort}`;
  const container = document.createElement("div");
  document.body.appendChild(container);
  const root = createRoot(container);
  try {
    await scrollTo(0);
    await act(async () => root.render(<HomeFilters />));
    const bar = () => container.firstElementChild!;
    await scrollTo(400);
    expect(bar().hasAttribute("data-hidden")).toBe(hides);
    await scrollTo(300);
    expect(bar().hasAttribute("data-hidden")).toBe(false);
  } finally {
    search.query = "q=private-search";
    await scrollTo(0);
    await act(async () => root.unmount());
    container.remove();
  }
});
