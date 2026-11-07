/** @vitest-environment jsdom */
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { PushPermissionGuide } from "./push-permission-guide";

const push = vi.hoisted(() => ({
  readPushSubscription: vi.fn(),
  registerPushSubscription: vi.fn(),
}));
vi.mock("@/lib/push", () => ({ ...push, pushSupported: () => true }));
vi.mock("@/lib/analytics", () => ({ trackEvent: vi.fn() }));
vi.mock("@/lib/after-startup", () => ({
  afterStartup: (callback: () => void) => {
    queueMicrotask(callback);
    return () => {};
  },
}));

const requestPermission = vi.fn();
let container: HTMLDivElement;
let root: Root;

beforeEach(() => {
  // jsdom は dialog のトップレイヤーを実装していない。
  Object.defineProperties(HTMLDialogElement.prototype, {
    showModal: { configurable: true, value(this: HTMLDialogElement) { this.open = true; } },
    close: { configurable: true, value(this: HTMLDialogElement) { this.open = false; } },
  });
  vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
  vi.stubGlobal("Notification", { permission: "default", requestPermission });
  requestPermission.mockReset().mockResolvedValue("granted");
  push.readPushSubscription.mockReset().mockResolvedValue(null);
  push.registerPushSubscription.mockReset().mockResolvedValue(undefined);
  localStorage.clear();
  container = document.createElement("div");
  document.body.appendChild(container);
  root = createRoot(container);
});

afterEach(async () => {
  await act(async () => root.unmount());
  container.remove();
  document.querySelector('[aria-modal="true"]')?.remove();
  Reflect.deleteProperty(HTMLDialogElement.prototype, "showModal");
  Reflect.deleteProperty(HTMLDialogElement.prototype, "close");
  vi.unstubAllGlobals();
});

function render(home = false) {
  return act(async () => root.render(
    <PushPermissionGuide
      home={home}
      userId="fixture"
      token="fixture-token"
      renderControl={home ? undefined : (_, open) => <button onClick={open}>通知設定</button>}
    />,
  ));
}

async function click(label: string) {
  const button = [...container.querySelectorAll("button")].find((item) => item.textContent === label)!;
  await act(async () => button.click());
}

it("waits for the active modal and only requests permission from the sheet", async () => {
  const otherModal = document.createElement("div");
  otherModal.setAttribute("aria-modal", "true");
  document.body.appendChild(otherModal);

  await render(true);
  expect(container.querySelector("dialog")).toBeNull();
  await act(async () => otherModal.remove());
  expect(container.querySelector("dialog[open]")).not.toBeNull();
  expect(requestPermission).not.toHaveBeenCalled();

  await click("通知を受け取る");
  expect(requestPermission).toHaveBeenCalledTimes(1);
  expect(push.registerPushSubscription).toHaveBeenCalledExactlyOnceWith("fixture-token");
  expect(container.querySelector("dialog")).toBeNull();
});

it("does not show an automatic sheet when this browser already has a subscription", async () => {
  push.readPushSubscription.mockResolvedValue({ endpoint: "fixture" });
  await render(true);
  expect(container.querySelector("dialog")).toBeNull();
  expect(requestPermission).not.toHaveBeenCalled();
});

function renderBreakthrough(key = "review-paid:req1") {
  return act(async () => root.render(
    <PushPermissionGuide userId="fixture" token="fixture-token" breakthroughKey={key} copy="reviewPaid" />,
  ));
}

it("shows the申込完了 sheet once even during the seven-day deferral", async () => {
  const clock = vi.spyOn(Date, "now").mockReturnValue(1_000_000);
  try {
    localStorage.setItem("tracktalk:push-guide:default:fixture:snoozed-until", String(1_000_000 + 1000));
    await renderBreakthrough();
    await act(async () => {});
    const dialog = container.querySelector("dialog[open]");
    expect(dialog).not.toBeNull();
    expect(dialog!.textContent).toContain("返信が届いたら、すぐお知らせします");
    expect(dialog!.textContent).toContain("コーチが添削を返したら通知します");
    expect(requestPermission).not.toHaveBeenCalled();

    await click("あとで");
    expect(container.querySelector("dialog")).toBeNull();
    // 閉じたらそこから7日数え直す。
    expect(Number(localStorage.getItem("tracktalk:push-guide:default:fixture:snoozed-until"))).toBe(
      1_000_000 + 7 * 24 * 60 * 60 * 1000,
    );

    // 同じ申込ではもう出ない。
    await renderBreakthrough();
    await act(async () => {});
    expect(container.querySelector("dialog")).toBeNull();
  } finally {
    clock.mockRestore();
  }
});

it("does not show the申込完了 sheet when this browser already has a subscription", async () => {
  push.readPushSubscription.mockResolvedValue({ endpoint: "fixture" });
  await renderBreakthrough();
  await act(async () => {});
  expect(container.querySelector("dialog")).toBeNull();
});

it("honors the seven-day deferral when the user opens notification settings", async () => {
  const clock = vi.spyOn(Date, "now").mockReturnValue(1_000_000);
  try {
    await render();
    await click("通知設定");
    await click("あとで");
    expect(requestPermission).not.toHaveBeenCalled();

    clock.mockReturnValue(1_000_000 + 7 * 24 * 60 * 60 * 1000 - 1);
    await click("通知設定");
    expect(container.querySelector("dialog")).toBeNull();

    clock.mockReturnValue(1_000_000 + 7 * 24 * 60 * 60 * 1000);
    await click("通知設定");
    expect(container.querySelector("dialog[open]")).not.toBeNull();
  } finally {
    clock.mockRestore();
  }
});
