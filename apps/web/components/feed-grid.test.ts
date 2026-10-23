import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { FeedGrid, splitIntoColumns } from "./feed-grid";

describe("splitIntoColumns", () => {
  it("新しい順の投稿を、左から右へ順番に配る", () => {
    expect(splitIntoColumns([1, 2, 3, 4, 5], 2)).toEqual([
      [1, 3, 5],
      [2, 4],
    ]);
    expect(splitIntoColumns([1, 2, 3, 4], 3)).toEqual([[1, 4], [2], [3]]);
  });
});

it("画面幅に依存せず2列に配置し、カードを重複して描かない", () => {
  const html = renderToStaticMarkup(createElement(FeedGrid, { children: ["投稿1", "投稿2", "投稿3"].map((text) => createElement("article", { key: text }, text)) }));
  expect(html.match(/class="min-w-0 flex-1"/g)).toHaveLength(2);
  expect(html.match(/<article>/g)).toHaveLength(3);
  expect(html).toContain("<article>投稿1</article><article>投稿3</article>");
});
