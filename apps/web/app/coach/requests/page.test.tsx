/** @vitest-environment jsdom */
import { act } from "react";
import { createRoot } from "react-dom/client";
import { expect, it, vi } from "vitest";
import CoachRequestsRedirectPage from "./page";

const replace = vi.fn();
vi.mock("next/navigation", () => ({ useRouter: () => ({ replace }) }));

// 旧URLのブックマークや古いリンクは、ハッシュに応じて新しい画面へ送る。
for (const [hash, target] of [["", "/coach/reviewing"], ["#offers", "/coach/reviewing"], ["#reviewing", "/coach/reviewing"]]) {
  it(`/coach/requests${hash} は ${target} へ送る`, async () => {
    (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
    replace.mockClear();
    window.history.replaceState({}, "", `/coach/requests${hash}`);
    const container = document.createElement("div");
    document.body.appendChild(container);
    const root = createRoot(container);
    await act(async () => root.render(<CoachRequestsRedirectPage />));
    expect(replace).toHaveBeenCalledWith(target);
    await act(async () => root.unmount());
    container.remove();
  });
}
