/** @vitest-environment jsdom */
import { act, StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { expect, it, vi } from "vitest";
import { ProductAnalytics } from "./product-analytics";

const state = vi.hoisted(() => ({ pathname: "/register", capture: vi.fn() }));
vi.mock("next/navigation", () => ({ usePathname: () => state.pathname }));
vi.mock("@/lib/posthog", () => ({ capturePosthog: state.capture }));

it("counts page transitions once under StrictMode, including a return visit", async () => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  const container = document.createElement("div");
  const root = createRoot(container);
  try {
    await act(async () => root.render(<StrictMode><ProductAnalytics /></StrictMode>));
    expect(state.capture).toHaveBeenCalledExactlyOnceWith("$pageview", "/register");
    state.pathname = "/me/profile";
    await act(async () => root.render(<StrictMode><ProductAnalytics /></StrictMode>));
    state.pathname = "/register";
    await act(async () => root.render(<StrictMode><ProductAnalytics /></StrictMode>));
    expect(state.capture).toHaveBeenCalledTimes(3);
  } finally {
    await act(async () => root.unmount());
  }
});
