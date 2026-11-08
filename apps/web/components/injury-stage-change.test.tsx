/** @vitest-environment jsdom */
import { act } from "react";
import { createRoot } from "react-dom/client";
import { expect, it, vi } from "vitest";
import { InjuryStageChange } from "./injury-stage-change";
import { LeaveGuardProvider } from "./leave-guard";

it("段階を選んだ後にやめるときは離脱確認を出す", () => {
  vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
  const container = document.createElement("div");
  document.body.appendChild(container);
  const root = createRoot(container);
  const onCancel = vi.fn();
  const button = (label: string) => {
    const match = [...container.querySelectorAll("button")].find((item) => item.textContent === label);
    if (!match) throw new Error(`ボタンが見つかりません: ${label}`);
    return match;
  };

  try {
    act(() => root.render(
      <LeaveGuardProvider>
        <InjuryStageChange
          part="hamstring-strain"
          token={null}
          member={{
            stage: "resting",
            stage_label: "安静中",
            injured_on: "2025-01-01",
            injured_on_exact: true,
            pre_changes: [],
          }}
          onChanged={vi.fn()}
          onCancel={onCancel}
        />
      </LeaveGuardProvider>,
    ));
    act(() => button("リハビリ中").click());
    act(() => button("やめる").click());
    expect(container.querySelector('[role="alertdialog"] h2')?.textContent).toBe("ページを離れますか？");
    expect(onCancel).not.toHaveBeenCalled();

    act(() => button("入力を続ける").click());
    expect(button("リハビリ中").getAttribute("aria-pressed")).toBe("true");
    act(() => button("やめる").click());
    act(() => button("ページを離れる").click());
    expect(onCancel).toHaveBeenCalledTimes(1);
  } finally {
    act(() => root.unmount());
    container.remove();
    vi.unstubAllGlobals();
  }
});
