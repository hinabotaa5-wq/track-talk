import { describe, expect, it } from "vitest";
import { splitIntoColumns } from "./feed-grid";

describe("splitIntoColumns", () => {
  it("新しい順の投稿を、左から右へ順番に配る", () => {
    expect(splitIntoColumns([1, 2, 3, 4, 5], 2)).toEqual([
      [1, 3, 5],
      [2, 4],
    ]);
    expect(splitIntoColumns([1, 2, 3, 4], 3)).toEqual([[1, 4], [2], [3]]);
  });
});
