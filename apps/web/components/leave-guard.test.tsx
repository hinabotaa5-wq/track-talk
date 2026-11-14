/** @vitest-environment jsdom */
import { act, useEffect } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { LeaveGuardProvider, useLeaveGuard } from "./leave-guard";

type Guard = ReturnType<typeof useLeaveGuard>;

let guard: Guard | null = null;
function Probe() {
  const value = useLeaveGuard();
  useEffect(() => {
    guard = value;
  }, [value]);
  return null;
}

let root: Root;
let container: HTMLDivElement;

beforeEach(() => {
  vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
  guard = null;
  container = document.createElement("div");
  document.body.appendChild(container);
  root = createRoot(container);
});

afterEach(async () => {
  await act(async () => root.unmount());
  container.remove();
  vi.unstubAllGlobals();
});

async function renderProvider() {
  await act(async () => {
    root.render(
      <LeaveGuardProvider>
        <Probe />
      </LeaveGuardProvider>,
    );
  });
  if (!guard) throw new Error("useLeaveGuard を取得できませんでした");
}

describe("既存の dirty 確認（回帰）", () => {
  it("入力中はダイアログを出し、離れるを選んだときだけ go を実行する", async () => {
    await renderProvider();
    const go = vi.fn();
    act(() => {
      guard!.setDirty(true);
    });
    let stopped = false;
    act(() => {
      stopped = guard!.confirmLeave(go);
    });
    expect(stopped).toBe(true);
    expect(go).not.toHaveBeenCalled();
    expect(container.textContent).toContain("ページを離れますか？");
    const leave = Array.from(container.querySelectorAll("button")).find(
      (button) => button.textContent === "ページを離れる",
    );
    if (!leave) throw new Error("「ページを離れる」が見つかりません");
    await act(async () => {
      leave.dispatchEvent(new MouseEvent("click", { bubbles: true, cancelable: true }));
    });
    expect(go).toHaveBeenCalledTimes(1);
  });

  it("入力がなければ confirmLeave は何もしない", async () => {
    await renderProvider();
    const go = vi.fn();
    let stopped = true;
    act(() => {
      stopped = guard!.confirmLeave(go);
    });
    expect(stopped).toBe(false);
    expect(go).not.toHaveBeenCalled();
    expect(container.textContent).not.toContain("ページを離れますか？");
  });
});
