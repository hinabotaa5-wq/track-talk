import { expect, it } from "vitest";
import { createGridLayout, gridTilePosition, visibleGridIndices } from "./home-feed-grid";

it.each([[390, 2], [768, 3], [1280, 4]])("幅 %ipx では %i 列を目安に 4:3 のカードで高さを使い切る", (width, columns) => {
  const layout = createGridLayout(720, width, 520, columns);
  expect(layout.tileHeight * layout.rows + 12 * (layout.rows - 1)).toBeCloseTo(layout.height);
  expect(layout.tileWidth / layout.tileHeight).toBeCloseTo(4 / 3);
  expect(gridTilePosition(layout, layout.rows)).toEqual({ left: layout.tileWidth + 12, top: 0 });
});

it("一覧が低くても 4:3 のカードが下部メニューの手前に収まる", () => {
  const layout = createGridLayout(20, 390, 90, 2);
  expect(layout.rows).toBe(1);
  expect(layout.tileWidth).toBe(120);
  expect(layout.tileHeight).toBe(90);
});

it("末尾の不完全な列でも、見える範囲のカードだけを順に返す", () => {
  const layout = createGridLayout(719, 390, 300, 2);
  const indices = visibleGridIndices(layout, layout.width - 390, 390);
  expect(indices[0]).toBeGreaterThan(700);
  expect(indices.at(-1)).toBe(718);
  expect(visibleGridIndices(layout, 0, 390, 0)).toEqual([0, 1, 2, 3]);
});
