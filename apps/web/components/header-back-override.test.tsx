/** @vitest-environment jsdom */
import { act, StrictMode, useEffect } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { Header } from "./header";
import { LeaveGuardProvider, useLeaveGuard } from "./leave-guard";
import { useHeaderBackOverride, useHeaderBackDestination } from "./header-back-override";
const { router } = vi.hoisted(() => ({ router: { push: vi.fn(), replace: vi.fn(), back: vi.fn() } }));
let pathname = "/coaching/requests/one";
vi.mock("next/navigation", () => ({ usePathname: () => pathname, useRouter: () => router }));
vi.mock("./auth-provider", () => ({ useAuth: () => ({ session: {}, profile: null }) }));
function Register({ destination, dirty = false }: { destination: string | null; dirty?: boolean }) {
  useHeaderBackOverride("/coaching/requests/one", destination);
  const { setDirty } = useLeaveGuard();
  useEffect(() => { setDirty(dirty); return () => setDirty(false); }, [dirty, setDirty]);
  return null;
}
function Probe() { return <output>{useHeaderBackDestination(pathname) ?? "none"}</output>; }
let root: Root;
let container: HTMLDivElement;
beforeEach(() => {
  vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true); vi.clearAllMocks(); pathname = "/coaching/requests/one";
  container = document.createElement("div"); document.body.append(container); root = createRoot(container);
});
afterEach(async () => { await act(async () => root.unmount()); container.remove(); vi.unstubAllGlobals(); });
async function render(destination: string | null, mounted = true, dirty = false) {
  await act(async () => root.render(<StrictMode><LeaveGuardProvider><Header /><Probe />{mounted && <Register destination={destination} dirty={dirty} />}</LeaveGuardProvider></StrictMode>));
}
async function clickBack() { await act(async () => container.querySelector<HTMLButtonElement>('button[aria-label="戻る"]')!.click()); }
it("C1/C3: 登録、条件変更、再登録とアンマウントで差し替えを更新・解除する", async () => {
  await render("/coaching/requests/one/feedback");
  expect(container.querySelector("output")?.textContent).toContain("/feedback");
  await clickBack(); expect(router.push).toHaveBeenLastCalledWith("/coaching/requests/one/feedback");
  await render(null); expect(container.querySelector("output")?.textContent).toBe("none");
  await render("/coaching/requests/one/feedback");
  await render(null, false); expect(container.querySelector("output")?.textContent).toBe("none");
  router.push.mockClear(); await clickBack(); expect(router.push).not.toHaveBeenCalled();
});
it("C3: 別画面では、解除前でも登録元パスと一致しなければ差し替えない", async () => {
  await render("/coaching/requests/one/feedback"); pathname = "/coaching/requests/two";
  await render("/coaching/requests/one/feedback");
  expect(container.querySelector("output")?.textContent).toBe("none");
  await clickBack(); expect(router.push).not.toHaveBeenCalled();
});
it("C1: 入力中は既存の離脱確認を経てから感想ページへ進む", async () => {
  await render("/coaching/requests/one/feedback", true, true);
  await clickBack(); expect(router.push).not.toHaveBeenCalled();
  expect(container.querySelector('[role="alertdialog"]')).not.toBeNull();
  await act(async () => [...container.querySelectorAll("button")].find(b => b.textContent === "入力を続ける")!.click());
  expect(router.push).not.toHaveBeenCalled();
  await clickBack();
  await act(async () => [...container.querySelectorAll("button")].find(b => b.textContent === "ページを離れる")!.click());
  expect(router.push).toHaveBeenCalledExactlyOnceWith("/coaching/requests/one/feedback");
  expect(router.back).not.toHaveBeenCalled(); expect(router.replace).not.toHaveBeenCalled();
});
