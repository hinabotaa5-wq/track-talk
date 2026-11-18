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

it("Googleから戻った直後は履歴があっても外部画面へ戻らない", async () => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  replace.mockClear(); back.mockClear();
  const container = document.createElement("div");
  document.body.appendChild(container);
  const root = createRoot(container);
  pathname = "/upload";
  window.history.pushState({}, "", "/upload");
  try {
    await act(async () => root.render(<Header />));
    await act(async () => container.querySelector<HTMLButtonElement>('button[aria-label="戻る"]')!.click());
    expect(back).not.toHaveBeenCalled();
    expect(replace).toHaveBeenCalledWith("/");
    window.history.pushState({}, "", "/posts/new");
    pathname = "/posts/new";
    await act(async () => root.render(<Header />));
    await act(async () => container.querySelector<HTMLButtonElement>('button[aria-label="戻る"]')!.click());
    expect(back).toHaveBeenCalledOnce();
  } finally {
    await act(async () => root.unmount());
    container.remove();
  }
});

for (const [landing, parent, fallback] of [
  ["/coach/apply", "/coach", "/coaching"],
  ["/coach/requests/example", "/coach", "/coaching"],
]) {
  it(`ログイン後 ${landing} から二回戻ってもログイン画面へ戻らない`, async () => {
    (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
    replace.mockClear(); back.mockClear();
    const container = document.createElement("div");
    document.body.appendChild(container);
    const root = createRoot(container);
    window.history.pushState({}, "", "/login?next=" + encodeURIComponent(landing));
    window.history.pushState({}, "", landing);
    pathname = landing;
    try {
      await act(async () => root.render(<Header />));
      await act(async () => container.querySelector<HTMLButtonElement>('button[aria-label="戻る"]')!.click());
      expect(replace).toHaveBeenLastCalledWith(parent);
      window.history.replaceState({}, "", parent);
      pathname = parent;
      await act(async () => root.render(<Header />));
      await act(async () => container.querySelector<HTMLButtonElement>('button[aria-label="戻る"]')!.click());
      expect(back).not.toHaveBeenCalled();
      expect(replace).toHaveBeenLastCalledWith(fallback);
    } finally {
      await act(async () => root.unmount());
      container.remove();
    }
  });
}

// 継続プラン詳細は、ログイン後でも「＜」1回でコーチタブ（/coaching）に着く。/coaches は経由しない。
it("ログイン後 継続プラン詳細から一回戻るとコーチタブに着き、ログイン画面へ戻らない", async () => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  replace.mockClear(); back.mockClear();
  const container = document.createElement("div");
  document.body.appendChild(container);
  const root = createRoot(container);
  const landing = "/coaching/personal/plans/example";
  window.history.pushState({}, "", "/login?next=" + encodeURIComponent(landing));
  window.history.pushState({}, "", landing);
  pathname = landing;
  try {
    await act(async () => root.render(<Header />));
    await act(async () => container.querySelector<HTMLButtonElement>('button[aria-label="戻る"]')!.click());
    expect(replace).toHaveBeenLastCalledWith("/coaching");
    window.history.replaceState({}, "", "/coaching");
    pathname = "/coaching";
    await act(async () => root.render(<Header />));
    expect(container.querySelector('button[aria-label="戻る"]')).toBeNull();
    expect(back).not.toHaveBeenCalled();
  } finally {
    await act(async () => root.unmount());
    container.remove();
  }
});

// コーチの子画面は、Stripe往復・ガイド経由など履歴が深くても、ヘッダーの「＜」1回で /coach に戻る。
for (const landing of [
  "/coach/offers",
  "/coach/reviewing",
  "/coach/consultations",
  "/coach/contracts",
  "/coach/plans",
  "/coach/commerce",
  "/coach/payouts",
  "/coach/profile",
  "/coach/requests/example",
]) {
  it(`${landing} は履歴が深くてもヘッダーの戻る1回で /coach に戻る`, async () => {
    (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
    replace.mockClear(); back.mockClear();
    const container = document.createElement("div");
    document.body.appendChild(container);
    const root = createRoot(container);
    for (const step of ["/coach", "/coach/payout-guide", "/coach/payouts"]) window.history.pushState({}, "", step);
    window.history.pushState({}, "", landing);
    pathname = landing;
    try {
      await act(async () => root.render(<Header />));
      await act(async () => container.querySelector<HTMLButtonElement>('button[aria-label="戻る"]')!.click());
      expect(back).not.toHaveBeenCalled();
      expect(replace).toHaveBeenCalledTimes(1);
      expect(replace).toHaveBeenLastCalledWith("/coach");
    } finally {
      await act(async () => root.unmount());
      container.remove();
    }
  });
}

// 依頼を返却すると詳細は 404（依頼が見つかりません）になる。返却後の「＜」連鎖で詳細を再表示させない。
for (const [name, entryPoints] of [
  ["対応中から詳細を開いた場合", ["/coaching", "/coach", "/coach/reviewing", "/coach/requests/example"]],
  ["詳細へ直接リンクで入った場合", ["/coaching", "/coach/requests/example"]],
] as const) {
  it(`依頼返却後（${name}）は「＜」2回で 対応中→コーチ画面→コーチタブ へ進み、詳細を経由しない`, async () => {
    (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
    replace.mockReset(); back.mockReset();
    replace.mockImplementation((href: string) => window.history.replaceState({}, "", href));
    const container = document.createElement("div");
    document.body.appendChild(container);
    const root = createRoot(container);
    const render = async (path: string) => {
      pathname = path;
      await act(async () => root.render(<Header />));
    };
    const clickBack = () => act(async () => container.querySelector<HTMLButtonElement>('button[aria-label="戻る"]')!.click());
    window.history.replaceState({}, "", entryPoints[0]);
    try {
      await render(entryPoints[0]);
      for (const step of entryPoints.slice(1)) {
        window.history.pushState({}, "", step);
        await render(step);
      }
      // 返却完了：router.push("/coach/reviewing")
      window.history.pushState({}, "", "/coach/reviewing");
      await render("/coach/reviewing");

      await clickBack();
      expect(replace).toHaveBeenLastCalledWith("/coach");
      await render("/coach");
      await clickBack();
      expect(back).not.toHaveBeenCalled();
      expect(replace).toHaveBeenLastCalledWith("/coaching");
      for (const [href] of replace.mock.calls) expect(href).not.toMatch(/^\/coach\/requests\//);
    } finally {
      await act(async () => root.unmount());
      container.remove();
    }
  });
}

it("D1/D2: 感想ページは履歴や送信状態によらずコーチ一覧へ replace する", async () => {
  replace.mockReset(); back.mockReset();
  const container = document.createElement("div"); document.body.append(container); const root = createRoot(container);
  window.history.pushState({}, "", "/coaching/requests/example");
  pathname = "/coaching/requests/example/feedback";
  window.history.pushState({}, "", pathname);
  try {
    await act(async () => root.render(<Header />));
    for (let i = 0; i < 2; i++) {
      await act(async () => container.querySelector<HTMLButtonElement>('button[aria-label="戻る"]')!.click());
      expect(replace).toHaveBeenLastCalledWith("/coaching");
    }
    expect(back).not.toHaveBeenCalled();
  } finally { await act(async () => root.unmount()); container.remove(); }
});
