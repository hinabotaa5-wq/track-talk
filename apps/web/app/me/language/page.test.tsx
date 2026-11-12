// @vitest-environment jsdom
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { LocaleProvider } from "@/components/locale-provider";
import LanguagePage from "./page";

// 表示言語の手動選択は廃止（運営者決定 2026-11-13）。直接開いたら 404 にせずマイページへ送る。
const nav = vi.hoisted(() => ({ replace: vi.fn() }));
vi.mock("next/navigation", () => ({ usePathname: () => "/me/language", useRouter: () => ({ replace: nav.replace }) }));

let root: Root;
let container: HTMLDivElement;
async function render() {
  await act(async () => root.render(<LocaleProvider><LanguagePage /></LocaleProvider>));
}
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  nav.replace.mockReset();
  container = document.createElement("div");
  document.body.appendChild(container);
  root = createRoot(container);
});
afterEach(async () => {
  await act(async () => root.unmount());
  container.remove();
  localStorage.clear();
});

it("言語の選択肢は出さず、マイページへ送る", async () => {
  await render();
  expect(nav.replace).toHaveBeenCalledWith("/me");
  expect(container.querySelector('[role="radiogroup"]')).toBeNull();
  expect(container.textContent).not.toContain("表示言語");
});
