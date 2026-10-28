/** @vitest-environment jsdom */
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";

const fetch = vi.hoisted(() => vi.fn());
const push = vi.hoisted(() => vi.fn());
const auth = vi.hoisted(() => ({ session: {} as object | null, token: "t" as string | null }));
vi.mock("@/lib/api", () => ({ apiFetch: fetch, apiErrorText: () => "失敗" }));
vi.mock("@/components/auth-provider", () => ({ useAuth: () => auth }));
vi.mock("next/navigation", () => ({ useRouter: () => ({ push }), usePathname: () => "/competitions/regional" }));

let root: Root, container: HTMLDivElement;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  vi.resetModules();
  fetch.mockReset();
  push.mockReset();
  auth.session = {};
  auth.token = "t";
  container = document.createElement("div");
  document.body.appendChild(container);
  root = createRoot(container);
});
afterEach(async () => {
  await act(async () => root.unmount());
  container.remove();
});

async function render() {
  const { TargetMeetButton } = await import("./target-meet-button");
  await act(async () =>
    root.render(
      <>
        <TargetMeetButton source="competition" meetRef="a" name="A大会" />
        <TargetMeetButton source="major" meetRef="b:2026" name="B大会" />
      </>,
    ),
  );
  return () => [...container.querySelectorAll("button")];
}

const meet = (source: string, ref: string, name: string) => ({ target_meet: { source, ref, name, starts_on: "2026-11-03", ends_on: null } });

it("セットすると置き換わり、セット中をもう一度押すと解除する", async () => {
  fetch.mockImplementation(async (path: string, init: { method?: string; body?: string } = {}) => {
    if (init.method === "PUT") {
      const { source, ref } = JSON.parse(init.body!);
      return meet(source, ref, ref);
    }
    if (init.method === "DELETE") return { target_meet: null };
    return meet("competition", "a", "A大会");
  });
  const buttons = await render();
  expect(buttons().map((b) => b.getAttribute("aria-pressed"))).toEqual(["true", "false"]);

  await act(async () => buttons()[1].click());
  expect(fetch).toHaveBeenLastCalledWith("/v1/me/target-meet", { method: "PUT", token: "t", body: JSON.stringify({ source: "major", ref: "b:2026" }) });
  expect(buttons().map((b) => b.getAttribute("aria-pressed"))).toEqual(["false", "true"]);
  expect(document.body.querySelector('[role="status"]')?.textContent).toBe("目標の大会にセットしました");

  await act(async () => buttons()[1].click());
  expect(fetch).toHaveBeenLastCalledWith("/v1/me/target-meet", { method: "DELETE", token: "t" });
  expect(buttons().map((b) => b.getAttribute("aria-pressed"))).toEqual(["false", "false"]);
  expect(document.body.querySelector('[role="status"]')).toBeNull();
});

it("未ログインならログインへ案内する", async () => {
  auth.session = null;
  auth.token = null;
  const buttons = await render();
  await act(async () => buttons()[0].click());
  expect(fetch).not.toHaveBeenCalled();
  expect(push).toHaveBeenCalledWith("/login?next=%2Fcompetitions%2Fregional");
});
