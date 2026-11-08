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
  const onLeft = vi.fn();
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
        onLeft={onLeft}
      />
    </LeaveGuardProvider>,
  ));

  function cleanup() {
    act(() => root.unmount());
    container.remove();
    vi.unstubAllGlobals();
  }

  return { container, button, member, onChanged, onCancel, onLeft, cleanup };
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
    expect(button("グループを抜ける").disabled).toBe(true);
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

it("確認してグループを抜けても段階変更は送信しない", async () => {
  const { container, button, onChanged, onLeft, cleanup } = renderStageChange();
  const request = Promise.withResolvers<Response>();
  const fetch = vi.fn().mockReturnValueOnce(request.promise);
  vi.stubGlobal("fetch", fetch);

  try {
    act(() => button("リハビリ中").click());
    act(() => button("グループを抜ける").click());
    expect(container.textContent).toContain("段階の記録は消えます。投稿は残ります。");
    expect(fetch).not.toHaveBeenCalled();

    act(() => button("抜ける").click());
    expect(button("次へ").disabled).toBe(true);
    expect(onLeft).not.toHaveBeenCalled();
    await act(async () => request.resolve(new Response(null, { status: 204 })));

    expect(fetch).toHaveBeenCalledExactlyOnceWith(
      expect.stringContaining("/v1/injury/groups/hamstring-strain/me"),
      expect.objectContaining({ method: "DELETE" }),
    );
    expect(onLeft).toHaveBeenCalledOnce();
    expect(onChanged).not.toHaveBeenCalled();
  } finally {
    cleanup();
  }
});
