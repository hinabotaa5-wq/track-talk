/** @vitest-environment jsdom */
import { act } from "react";
import { createRoot } from "react-dom/client";
import { expect, it, vi } from "vitest";
import { InjuryStageChange } from "./injury-stage-change";
import { LeaveGuardProvider } from "./leave-guard";

function renderStageChange() {
  vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
  const container = document.createElement("div");
  document.body.appendChild(container);
  const root = createRoot(container);
  const onCancel = vi.fn();
  const onChanged = vi.fn();
  const member = {
    stage: "resting" as const,
    stage_label: "安静中",
    injured_on: "2025-01-01",
    injured_on_exact: true,
    pre_changes: [],
  };
  const button = (label: string) => {
    const match = [...container.querySelectorAll("button")].find((item) => item.textContent === label);
    if (!match) throw new Error(`ボタンが見つかりません: ${label}`);
    return match;
  };

  act(() => root.render(
    <LeaveGuardProvider>
      <InjuryStageChange
        part="hamstring-strain"
        token={null}
        member={member}
        onChanged={onChanged}
        onCancel={onCancel}
      />
    </LeaveGuardProvider>,
  ));

  function cleanup() {
    act(() => root.unmount());
    container.remove();
    vi.unstubAllGlobals();
  }

  return { container, button, member, onChanged, onCancel, cleanup };
}

it("段階を選んだ後にやめるときは離脱確認を出す", () => {
  const { container, button, onCancel, cleanup } = renderStageChange();
  try {
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
    cleanup();
  }
});

it("段階を変えている間は変更先を表示して入力を止め、失敗後は再送できる", async () => {
  const { container, button, member, onChanged, cleanup } = renderStageChange();
  const next = { ...member, stage: "rehab", stage_label: "リハビリ中" };
  const first = Promise.withResolvers<Response>();
  const fetch = vi.fn().mockReturnValueOnce(first.promise).mockResolvedValueOnce(Response.json(next));
  vi.stubGlobal("fetch", fetch);

  try {
    act(() => button("リハビリ中").click());
    act(() => button("次へ").click());
    act(() => button("何もしなかった").click());
    act(() => button("次へ").click());
    act(() => button("段階を変える").click());

    expect(container.querySelector('[role="status"]')?.textContent).toBe("段階を変えています");
    expect(button("リハビリ中に変更しています…").disabled).toBe(true);
    expect(button("戻る").disabled).toBe(true);
    expect(container.querySelector("textarea")?.disabled).toBe(true);

    await act(async () => first.reject(new Error("offline")));
    expect(button("段階を変える").disabled).toBe(false);
    expect(container.querySelector('[role="status"]')?.textContent).toBe("");

    await act(async () => button("段階を変える").click());
    expect(fetch).toHaveBeenCalledTimes(2);
    expect(onChanged).toHaveBeenCalledExactlyOnceWith(next, false);
  } finally {
    cleanup();
  }
});
