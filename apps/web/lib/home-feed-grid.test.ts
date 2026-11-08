import { expect, it } from "vitest";
import { createGridLayout, gridTilePosition, visibleGridIndices } from "./home-feed-grid";

it.each([[390], [768], [1280]])("幅 %ipx でも段数は3のまま 4:3 のカードで高さを使い切る", (width) => {
  const layout = createGridLayout(720, width, 520);
  expect(layout.rows).toBe(3);
  expect(layout.tileHeight * layout.rows + 12 * (layout.rows - 1)).toBeCloseTo(layout.height);
  expect(layout.tileWidth / layout.tileHeight).toBeCloseTo(4 / 3);
  expect(gridTilePosition(layout, layout.rows)).toEqual({ left: layout.tileWidth + 12, top: 0 });
});

it.each([[300], [520], [800]])("高さ %ipx が変わっても段数は3のまま（帯の有無で動かない）", (height) => {
  expect(createGridLayout(720, 390, height).rows).toBe(3);
});

it("一覧が低くても段数は3のまま札を縮めて下部メニューの手前に収める", () => {
  const layout = createGridLayout(20, 390, 90);
  expect(layout.rows).toBe(3);
  expect(layout.tileWidth).toBeCloseTo(22 * (4 / 3));
  expect(layout.tileHeight).toBe(22);
});

it("末尾の不完全な列でも、見える範囲のカードだけを順に返す", () => {
  const layout = createGridLayout(719, 390, 300);
  const indices = visibleGridIndices(layout, layout.width - 390, 390);
  expect(indices[0]).toBeGreaterThan(690);
  expect(indices.at(-1)).toBe(718);
  expect(visibleGridIndices(layout, 0, 390, 0)).toEqual([0, 1, 2, 3, 4, 5, 6, 7, 8]);
});
