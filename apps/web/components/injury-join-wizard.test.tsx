/** @vitest-environment jsdom */
import { act } from "react";
import { createRoot } from "react-dom/client";
import { expect, it, vi } from "vitest";
import { InjuryJoinWizard } from "./injury-join-wizard";

vi.mock("./leave-guard", () => ({
  LeaveGuardArea: ({ children }: { children: React.ReactNode }) => children,
  useLeaveGuard: () => ({ setDirty: vi.fn() }),
}));

it.each([
  ["違和感の段階で休む", "ウォームアップを念入りにする"],
  ["ウォームアップを念入りにする", "違和感の段階で休む"],
])("連続で選んだ回答を保持する (%s → %s)", (first, second) => {
  vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
  const container = document.createElement("div");
  document.body.appendChild(container);
  const root = createRoot(container);
  const button = (label: string) => {
    const match = [...container.querySelectorAll("button")].find((item) => item.textContent === label);
    if (!match) throw new Error(`選択肢が見つかりません: ${label}`);
    return match;
  };

  try {
    act(() => root.render(<InjuryJoinWizard part="hamstring-strain" token={null} onJoined={vi.fn()} />));
    for (const label of ["ケガの重さ", "病院でみてもらったか"]) {
      const choice = container.querySelector<HTMLButtonElement>(`[aria-label="${label}"] button`)!;
      act(() => choice.click());
      act(() => button("次へ").click());
    }
    act(() => {
      const date = container.querySelector<HTMLInputElement>('input[type="date"]')!;
      Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value")!.set!.call(date, "2025-01-01");
      date.dispatchEvent(new Event("input", { bubbles: true }));
    });
    act(() => button("次へ").click());
    for (const label of ["タータン", "ジョグ", "とくになし"]) {
      act(() => button(label).click());
      act(() => button("次へ").click());
    }

    act(() => {
      button("わからない・とくにない").click();
      button(first).click();
      button(second).click();
    });
    expect(button(first).getAttribute("aria-pressed")).toBe("true");
    expect(button(second).getAttribute("aria-pressed")).toBe("true");
    expect(button("わからない・とくにない").getAttribute("aria-pressed")).toBe("false");

    act(() => button(first).click());
    expect(button(first).getAttribute("aria-pressed")).toBe("false");
    expect(button(second).getAttribute("aria-pressed")).toBe("true");
  } finally {
    act(() => root.unmount());
    container.remove();
    vi.unstubAllGlobals();
  }
});
