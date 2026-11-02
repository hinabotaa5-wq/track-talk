/** @vitest-environment jsdom */
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it } from "vitest";
import { SenseNoteJar } from "./sense-note-jar";

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
let container: HTMLDivElement;
let root: Root;
beforeEach(() => { container = document.createElement("div"); document.body.append(container); root = createRoot(container); });
afterEach(() => { act(() => root.unmount()); container.remove(); });

function label() { return container.querySelector("svg")?.getAttribute("aria-label"); }

it("6粒たまってまだまとめていない間は満杯のビンを見せる", () => {
  act(() => root.render(<SenseNoteJar total={6} />));
  expect(label()).toBe("感覚のビン 1杯目、6/6粒");
  expect(container.querySelectorAll("circle")).toHaveLength(6);
});

it("6粒をまとめ終えたら空の次のビンに移る", () => {
  act(() => root.render(<SenseNoteJar total={6} summarized />));
  expect(label()).toBe("感覚のビン 2杯目、0/6粒");
  expect(container.querySelectorAll("circle")).toHaveLength(0);
  expect(container.textContent).toContain("2杯目");
});

it("まとめた後に書き足した粒は次のビンに入る", () => {
  act(() => root.render(<SenseNoteJar total={7} summarized />));
  expect(label()).toBe("感覚のビン 2杯目、1/6粒");
});
