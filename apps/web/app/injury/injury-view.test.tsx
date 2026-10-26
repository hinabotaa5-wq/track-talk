/** @vitest-environment jsdom */
import { act } from "react";
import { createRoot } from "react-dom/client";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it, vi } from "vitest";
import type { InjuryGroupSummary } from "@/lib/injury";
import { InjuryGroupPicker } from "./injury-view";

const groups: InjuryGroupSummary[] = [
  { body_part: "shin-splints", body_part_label: "シンスプリント", members: 4 },
  { body_part: "jumpers-knee", body_part_label: "ジャンパー膝", members: 2 },
  {
    body_part: "hamstring-strain",
    body_part_label: "ハムストリングの肉離れ",
    members: 6,
    me: { stage: "rehab", stage_label: "リハビリ中", injured_on: "2026-10-01", injured_on_exact: true, pre_changes: [] },
  },
];

describe("InjuryGroupPicker", () => {
  it("opens on the region of the group I joined and lists only its injuries", () => {
    const html = renderToStaticMarkup(<InjuryGroupPicker groups={groups} />);
    expect(html).toMatch(/aria-pressed="true"[^>]*>太もも</);
    expect(html).toContain("ハムストリングの肉離れ");
    expect(html).toContain("参加中 · リハビリ中");
    expect(html).toContain('href="/injury/hamstring-strain"');
    expect(html).not.toContain("シンスプリント");
  });

  it("shows how many people are in each region on the body", () => {
    const html = renderToStaticMarkup(<InjuryGroupPicker groups={groups.filter((group) => !group.me)} />);
    expect(html).toMatch(/aria-pressed="true"[^>]*>ひざ・すね</);
    expect(html).toMatch(/>6<\/text>/);
    expect(html).toContain("グループに入る");
  });
});

it("人型や場所ボタンを選ぶとグループ一覧へ移り、初期表示ではスクロールしない", async () => {
  (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  const scroll = vi.fn();
  const matchMedia = vi.fn().mockReturnValue({ matches: false });
  let frame: FrameRequestCallback | undefined;
  vi.stubGlobal("requestAnimationFrame", (callback: FrameRequestCallback) => { frame = callback; return 1; });
  vi.stubGlobal("matchMedia", matchMedia);
  Object.defineProperty(HTMLElement.prototype, "scrollIntoView", { value: scroll, configurable: true });
  const container = document.createElement("div");
  document.body.append(container);
  const root = createRoot(container);
  try {
    await act(async () => root.render(<InjuryGroupPicker groups={groups} />));
    expect(scroll).not.toHaveBeenCalled();
    const region = [...container.querySelectorAll("button")].find((button) => button.textContent === "ひざ・すね")!;
    await act(async () => region.click());
    frame?.(0);
    expect(container.textContent).toContain("シンスプリント");
    expect(scroll).toHaveBeenLastCalledWith({ behavior: "smooth", block: "start" });
    matchMedia.mockReturnValue({ matches: true });
    await act(async () => region.click());
    frame?.(0);
    expect(scroll).toHaveBeenLastCalledWith({ behavior: "instant", block: "start" });
    expect(scroll).toHaveBeenCalledTimes(2);
  } finally {
    await act(async () => root.unmount());
    container.remove();
    vi.unstubAllGlobals();
    Reflect.deleteProperty(HTMLElement.prototype, "scrollIntoView");
  }
});
