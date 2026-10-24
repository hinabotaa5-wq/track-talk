import { describe, expect, it } from "vitest";
import { packMosaic } from "./masonry-feed";

const item = (ratio: number) => ({ ratio, footer: 40 });
describe("packMosaic", () => {
  it("縦長を小さくし、その下の空きにも横長を組み合わせる", () => {
    const { tiles } = packMosaic([item(9 / 16), item(16 / 9), item(16 / 9), item(16 / 9)], 358, 620);
    expect(tiles[0].height).toBeLessThanOrEqual(322);
    expect(tiles[0].width).toBeLessThan(160);
    expect(tiles[1].x).toBe(0);
    expect(tiles[1].y).toBeGreaterThan(tiles[0].height);
    expect(tiles[2].x).toBeGreaterThan(0);
  });
  it("画面が広ければ2列に制限せず、収まる枚数を配置する", () => {
    const { tiles } = packMosaic(Array.from({ length: 12 }, () => item(16 / 9)), 1000, 540);
    const visible = tiles.filter((tile) => tile.x + tile.width <= 1000);
    expect(new Set(visible.map((tile) => tile.x)).size).toBeGreaterThan(2);
    expect(visible.length).toBeGreaterThanOrEqual(6);
  });
  it("下にカードがある空きにも、縦長を縮めて収める", () => {
    const { tiles } = packMosaic([9 / 16, 16 / 9, 16 / 9, 16 / 9, 16 / 9, 9 / 16, 16 / 9, 9 / 16].map(item), 992, 479);
    expect(tiles[7].height).toBeLessThanOrEqual(tiles[0].height);
    expect(tiles[7].y + tiles[7].height).toBeLessThanOrEqual(479);
  });
  it("元の比率を保って、重ならずに画面内の空きへ配置する", () => {
    const items = Array.from({ length: 25 }, (_, index) => item([9 / 16, 16 / 9, 1, 4 / 3, 0.3][index % 5]));
    for (const width of [288, 358, 720, 1000]) {
      const { tiles, width: contentWidth } = packMosaic(items, width, 540);
      expect(tiles).toHaveLength(items.length);
      expect(contentWidth).toBeGreaterThan(width);
      tiles.forEach((tile, index) => {
        expect((tile.width - 2) / (tile.height - 42)).toBeCloseTo(items[index].ratio);
        expect(tile.y).toBeGreaterThanOrEqual(0);
        expect(tile.y + tile.height).toBeLessThanOrEqual(540.1);
        for (const earlier of tiles.slice(0, index)) {
          const overlaps = tile.x < earlier.x + earlier.width && tile.x + tile.width > earlier.x
            && tile.y < earlier.y + earlier.height && tile.y + tile.height > earlier.y;
          expect(overlaps).toBe(false);
        }
      });
    }
  });
  it("後の投稿が手前に残った空きへ戻って詰まる", () => {
    const { tiles } = packMosaic([4 / 3, 16 / 9, 9 / 16, 16 / 9].map(item), 358, 540);
    expect(tiles[2].x).toBeGreaterThan(0);
    expect(tiles[3].x).toBe(0);
    expect(tiles[3].y).toBeGreaterThan(tiles[1].y + tiles[1].height);
  });
  it("同じ投稿順と画面サイズでは配置を毎回変えない", () => {
    const items = [item(9 / 16), item(16 / 9), item(1), item(4 / 3)];
    expect(packMosaic(items, 358, 560)).toEqual(packMosaic(items, 358, 560));
  });
  it("続きの投稿を見せるために既存のカードを縮めず、自然に見切れさせる", () => {
    const items = Array.from({ length: 12 }, () => item(16 / 9));
    const short = packMosaic(items.slice(0, 3), 358, 590);
    const full = packMosaic(items, 358, 590);
    expect(full.tiles.slice(0, 3)).toEqual(short.tiles);
    expect(full.tiles[0].width).toBeGreaterThan(200);
    const next = full.tiles.find((tile) => tile.x + tile.width > 358)!;
    expect(358 - next.x).toBeGreaterThan(0);
    expect(358 - next.x).toBeLessThan(next.width);
  });
  it("文章だけの投稿は写真や横動画より小さく置く", () => {
    const { tiles } = packMosaic([
      { ratio: 4 / 3, footer: 0, compact: true },
      { ratio: 4 / 3, footer: 0 },
    ], 390, 560);
    expect(tiles[0].width).toBeLessThan(tiles[1].width);
  });
  it("縦動画は従来の小さい上限より大きく置く", () => {
    const { tiles } = packMosaic([{ ratio: 9 / 16, footer: 0 }], 390, 560);
    expect(tiles[0].height).toBeGreaterThan(250);
  });
});

it("600件でも表示付近のカードだけ選び、遠い過去のカードを含めない", async () => {
  const { visibleMosaicIndices, createMosaicPacker } = await import("./masonry-feed");
  const items = Array.from({ length: 600 }, (_, i) => item([9 / 16, 16 / 9, 1, 4 / 3][i % 4]));
  const pack = createMosaicPacker();
  const layout = pack(items, 390, 560);
  for (const left of [0, layout.width / 2, layout.width - 390]) {
    const indices = visibleMosaicIndices(layout, left, 390);
    expect(indices.length).toBeGreaterThan(0);
    expect(indices.length).toBeLessThan(40);
    expect(indices.every((i) => layout.tiles[i].x <= left + 780 && layout.tiles[i].x + layout.tiles[i].width >= left - 390)).toBe(true);
  }
  const extended = pack([...items, ...items.slice(0, 12)], 390, 560);
  expect(extended.tiles.slice(0, 600)).toEqual(layout.tiles);
});
