/** @vitest-environment jsdom */
import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { externalTripGapMs, recoveryGraceMs, recoveryRecheckMs, resumePagehideKey } from "@/lib/resume-recovery";
import { ResumeRecovery } from "./resume-recovery";

let pathname = "/";
let search = "";
const refresh = vi.fn();
vi.mock("next/navigation", () => ({
  usePathname: () => pathname,
  useRouter: () => ({ refresh }),
  useSearchParams: () => ({ toString: () => search }),
}));
vi.mock("next/link", () => ({ default: ({ href, children, ...props }: { href: string; children: ReactNode }) => <a href={href} {...props}>{children}</a> }));

let root: Root;
let container: HTMLDivElement;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  vi.useFakeTimers();
  pathname = "/";
  search = "";
  refresh.mockReset();
  window.sessionStorage.clear();
  document.body.innerHTML = "";
  container = document.createElement("div");
  document.body.appendChild(container);
  root = createRoot(container);
});
afterEach(async () => {
  await act(async () => root.unmount());
  container.remove();
  document.body.innerHTML = "";
  vi.useRealTimers();
  vi.restoreAllMocks();
});

async function render() {
  await act(async () => root.render(<ResumeRecovery />));
}

function pastPageHide() {
  window.sessionStorage.setItem(resumePagehideKey, String(Date.now() - externalTripGapMs - 60_000));
}

it("ふだんの表示では何もしない", async () => {
  document.body.insertAdjacentHTML("afterbegin", "<header></header><main><p>本文</p></main>");
  await render();
  await act(async () => vi.advanceTimersByTime(30_000));
  expect(refresh).not.toHaveBeenCalled();
  expect(container.querySelector('[role="alert"]')).toBeNull();
});

it("外部往復のあとにヘッダーと本文がなければ、取り直してから再読み込みの導線を出す", async () => {
  pastPageHide();
  document.body.insertAdjacentHTML("afterbegin", "<main></main>");
  await render();
  await act(async () => vi.advanceTimersByTime(recoveryGraceMs));
  expect(refresh).toHaveBeenCalledOnce();
  await act(async () => vi.advanceTimersByTime(recoveryRecheckMs));
  const alert = container.querySelector('[role="alert"]');
  expect(alert?.textContent).toContain("再読み込み");
});

it("外部往復のあとに正常に描画されていれば静かに終わる", async () => {
  pastPageHide();
  document.body.insertAdjacentHTML("afterbegin", "<header></header><main><p>本文</p></main>");
  await render();
  await act(async () => vi.advanceTimersByTime(recoveryGraceMs + recoveryRecheckMs));
  expect(refresh).not.toHaveBeenCalled();
  expect(container.querySelector('[role="alert"]')).toBeNull();
});

it("取り直しで直れば導線は出ない", async () => {
  pastPageHide();
  document.body.insertAdjacentHTML("afterbegin", "<main></main>");
  await render();
  await act(async () => vi.advanceTimersByTime(recoveryGraceMs));
  expect(refresh).toHaveBeenCalledOnce();
  document.querySelector("main")!.innerHTML = "<p>本文</p>";
  document.body.insertAdjacentHTML("afterbegin", "<header></header>");
  await act(async () => vi.advanceTimersByTime(recoveryRecheckMs));
  expect(container.querySelector('[role="alert"]')).toBeNull();
});

it("ヘッダーを出さない画面ではヘッダーがなくても導線は出ない", async () => {
  pathname = "/me/profile";
  search = "onboarding=1";
  pastPageHide();
  document.body.insertAdjacentHTML("afterbegin", "<main><p>入力</p></main>");
  await render();
  await act(async () => vi.advanceTimersByTime(recoveryGraceMs + recoveryRecheckMs));
  expect(container.querySelector('[role="alert"]')).toBeNull();
});

it("導線は閉じることができる", async () => {
  pastPageHide();
  document.body.insertAdjacentHTML("afterbegin", "<main></main>");
  await render();
  await act(async () => vi.advanceTimersByTime(recoveryGraceMs + recoveryRecheckMs));
  const close = container.querySelector('button[aria-label="閉じる"]') as HTMLButtonElement | null;
  expect(close).not.toBeNull();
  await act(async () => close!.click());
  expect(container.querySelector('[role="alert"]')).toBeNull();
});

it("bfcache の復帰でも確かめる", async () => {
  document.body.insertAdjacentHTML("afterbegin", "<main></main>");
  await render();
  await act(async () => vi.advanceTimersByTime(1_000));
  expect(refresh).not.toHaveBeenCalled();
  await act(async () => {
    const event = new Event("pageshow") as Event & { persisted?: boolean };
    event.persisted = true;
    window.dispatchEvent(event);
  });
  await act(async () => vi.advanceTimersByTime(recoveryGraceMs + recoveryRecheckMs));
  expect(refresh).toHaveBeenCalledOnce();
  expect(container.querySelector('[role="alert"]')).not.toBeNull();
});
