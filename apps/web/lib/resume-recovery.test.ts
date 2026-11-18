/** @vitest-environment jsdom */
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import {
  consumeExternalReturn,
  externalTripGapMs,
  isHeaderMissing,
  isMainEmpty,
  notePageHide,
  shouldRefreshAgain,
} from "./resume-recovery";

beforeEach(() => {
  vi.spyOn(HTMLElement.prototype, "getBoundingClientRect").mockReturnValue({ height: 60 } as DOMRect);
});

afterEach(() => {
  vi.restoreAllMocks();
  window.sessionStorage.clear();
  vi.useRealTimers();
});

it("pagehide の記録がない初回表示では復帰と判定しない", () => {
  expect(consumeExternalReturn(window.sessionStorage, 1_000_000)).toBe(false);
});

it("短い離脱（タブの往復程度）では復帰と判定せず、記録だけ消費する", () => {
  notePageHide(window.sessionStorage, 1_000_000);
  expect(consumeExternalReturn(window.sessionStorage, 1_000_000 + externalTripGapMs - 1)).toBe(false);
  // 消費済みなので二度目は記録なしと同じ。
  expect(consumeExternalReturn(window.sessionStorage, 1_000_000 + externalTripGapMs + 60_000)).toBe(false);
});

it("外部サイト往復の長さの離脱では復帰と判定する", () => {
  notePageHide(window.sessionStorage, 1_000_000);
  expect(consumeExternalReturn(window.sessionStorage, 1_000_000 + externalTripGapMs)).toBe(true);
});

it("壊れた記録では復帰と判定しない", () => {
  window.sessionStorage.setItem("tracktalk:resume-pagehide-at", "not-a-number");
  expect(consumeExternalReturn(window.sessionStorage, 1_000_000)).toBe(false);
});

it("保存できない環境でも落ちない", () => {
  const broken = { getItem: () => { throw new Error("denied"); }, setItem: () => { throw new Error("denied"); }, removeItem: () => { throw new Error("denied"); } };
  expect(() => notePageHide(broken, 1_000_000)).not.toThrow();
  expect(consumeExternalReturn(broken, 2_000_000)).toBe(false);
  expect(shouldRefreshAgain(broken, 2_000_000)).toBe(true);
});

it("自動の取り直しは一定時間に1回まで", () => {
  expect(shouldRefreshAgain(window.sessionStorage, 1_000_000)).toBe(true);
  expect(shouldRefreshAgain(window.sessionStorage, 1_000_001)).toBe(false);
  expect(shouldRefreshAgain(window.sessionStorage, 1_000_000 + 5 * 60_000)).toBe(true);
});

it("header 要素の有無を見分ける", () => {
  document.body.innerHTML = "<header><button>戻る</button></header><main><p>本文</p></main>";
  expect(isHeaderMissing(document)).toBe(false);
  document.querySelector("header")!.remove();
  expect(isHeaderMissing(document)).toBe(true);
});

it("main の中身が要素ごと空のときだけ空と判定する", () => {
  document.body.innerHTML = "<main></main>";
  expect(isMainEmpty(document)).toBe(true);
  document.body.innerHTML = "<main>文章だけ</main>";
  expect(isMainEmpty(document)).toBe(true);
  document.body.innerHTML = "<main><div>読み込み中</div></main>";
  expect(isMainEmpty(document)).toBe(false);
  document.body.innerHTML = "";
  expect(isMainEmpty(document)).toBe(true);
});

it.each([
  '<header></header>',
  '<header><div></div></header>',
  '<header style="display:none"><button>戻る</button></header>',
  '<header style="visibility:hidden"><button>戻る</button></header>',
  '<header style="opacity:0"><button>戻る</button></header>',
])("空または非表示の header を見分ける: %s", (html) => {
  document.body.innerHTML = html;
  expect(isHeaderMissing(document)).toBe(true);
});

it("高さが潰れた header を見分ける", () => {
  document.body.innerHTML = '<header><button>戻る</button></header>';
  vi.spyOn(document.querySelector("header")!, "getBoundingClientRect").mockReturnValue({ height: 0 } as DOMRect);
  expect(isHeaderMissing(document)).toBe(true);
});
