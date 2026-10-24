/** @vitest-environment jsdom */
import { act } from "react";
import { createRoot } from "react-dom/client";
import { expect, it, vi } from "vitest";
import { Header } from "./header";

const { replace, back } = vi.hoisted(() => ({ replace: vi.fn(), back: vi.fn() }));
let pathname = "/coach/about";
vi.mock("next/navigation", () => ({ usePathname: () => pathname, useRouter: () => ({ replace, back }) }));
vi.mock("./auth-provider", () => ({ useAuth: () => ({ session: {}, profile: null }) }));
vi.mock("./leave-guard", () => ({ useLeaveGuard: () => ({ confirmLeave: () => false }), useGuardedLinkClick: () => vi.fn() }));
vi.mock("./today-meet-balloon", () => ({ TodayMeetBalloon: () => null }));

it("説明→応募→コーチ画面へ、履歴に左右されず一回ずつ戻れる", async () => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  const container = document.createElement("div");
  document.body.appendChild(container);
  const root = createRoot(container);
  window.history.pushState({}, "", "/coach/about");
  try {
    for (const [path, target] of [["/coach/about", "/coach/apply"], ["/coach/apply", "/coach"]]) {
      pathname = path;
      await act(async () => root.render(<Header />));
      await act(async () => container.querySelector<HTMLButtonElement>('button[aria-label="戻る"]')!.click());
      expect(replace).toHaveBeenLastCalledWith(target);
    }
    expect(back).not.toHaveBeenCalled();
  } finally {
    await act(async () => root.unmount());
    container.remove();
  }
});
