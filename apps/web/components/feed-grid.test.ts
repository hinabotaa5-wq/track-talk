import { describe, expect, it } from "vitest";
import { layoutsFor, splitIntoColumns } from "./feed-grid";

describe("splitIntoColumns", () => {
  it("新しい順の投稿を、左から右へ順番に配る", () => {
    expect(splitIntoColumns([1, 2, 3, 4, 5], 2)).toEqual([
      [1, 3, 5],
      [2, 4],
    ]);
    expect(splitIntoColumns([1, 2, 3, 4], 3)).toEqual([[1, 4], [2], [3]]);
  });
});

describe("layoutsFor", () => {
  it("幅がわからないあいだは、2・3・4列をすべて描いて CSS で選ぶ", () => {
    expect(layoutsFor(0).map((l) => l.columns)).toEqual([2, 3, 4]);
    expect(layoutsFor(0).map((l) => l.className)).toEqual(["flex md:hidden", "hidden md:flex xl:hidden", "hidden xl:flex"]);
  });

  it("幅がわかったら、その列の数だけを描く", () => {
    expect(layoutsFor(3).map((l) => l.columns)).toEqual([3]);
  });
});
