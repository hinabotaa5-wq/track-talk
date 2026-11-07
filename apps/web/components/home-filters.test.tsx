/** @vitest-environment jsdom */
import { act } from "react";
import { createRoot } from "react-dom/client";
import { expect, it, vi } from "vitest";
import { HomeFilters } from "./home-filters";

vi.mock("./home-post-button", () => ({ HomePostButton: () => <a href="/upload" aria-label="投稿する">＋</a> }));
const search = vi.hoisted(() => ({ query: "" }));
vi.mock("next/navigation", () => ({ useSearchParams: () => new URLSearchParams(search.query) }));

it("以前の検索URLを開いても、選択中のタブから通常の一覧に戻れる", async () => {
  search.query = "q=old-search&page=2";
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  const container = document.createElement("div");
  document.body.appendChild(container);
  const root = createRoot(container);
  const pushState = vi.spyOn(window.history, "pushState").mockImplementation(() => {});
  try {
    await act(async () => root.render(<HomeFilters />));
    expect(container.querySelector('[role="search"]')).toBeNull();
    await act(async () => { container.querySelector<HTMLAnchorElement>('a[aria-current="page"]')!.click(); });
    expect(pushState).toHaveBeenCalledExactlyOnceWith(null, "", "/");
  } finally {
    search.query = "";
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
    search.query = "";
    await scrollTo(0);
    await act(async () => root.unmount());
    container.remove();
  }
});
