export type MosaicItem = { ratio: number; footer: number; compact?: boolean };
export type MosaicTile = { x: number; y: number; width: number; height: number };

// 列やページを作らず、既存のカードの端から次の空きを探す。
// 投稿順に配置し、後の投稿も手前に残った空きへ詰める。
function packBlock(items: MosaicItem[], viewportWidth: number, height: number, gap = 12) {
  const landscapeWidth = Math.max(176, Math.min(308, viewportWidth * 0.605));
  // 縦動画は走りの動きが見やすい大きさにする。文章だけの投稿は少し小さく、写真・動画を優先する。
  const portraitHeight = Math.min(280, height * 0.56);
  const tiles: MosaicTile[] = [];

  for (const item of items) {
    const ratio = Number.isFinite(item.ratio) && item.ratio > 0 ? item.ratio : 4 / 3;
    const footer = Math.max(0, item.footer);
    const preferredWidth = ratio < 1 ? portraitHeight * ratio + 2 : landscapeWidth * (item.compact ? 0.78 : 1);
    const width = Math.min(preferredWidth, (height - footer - 2) * ratio + 2);
    const xs = [...new Set([0, ...tiles.map((tile) => tile.x + tile.width + gap)])]
      .sort((a, b) => a - b);
    const ys = [...new Set([0, ...tiles.map((tile) => tile.y + tile.height + gap)])].sort((a, b) => a - b);
    let next: MosaicTile | undefined;
    for (const x of xs) {
      for (const y of ys) {
        // 画面の下だけでなく、下にあるカードまでの空きにも縮めて収める。
        const bottoms = [height, ...tiles.filter((tile) => tile.y > y).map((tile) => tile.y - gap)];
        const widths = [...new Set(bottoms.map((bottom) => Math.min(width, (bottom - y - footer - 2) * ratio + 2)))]
          .filter((candidate) => candidate >= width * (ratio < 1 ? 0.6 : 0.78)).sort((a, b) => b - a);
        for (const fittedWidth of widths) {
          const tileHeight = (fittedWidth - 2) / ratio + footer + 2;
          const overlaps = tiles.some((tile) => x < tile.x + tile.width + gap - 0.1 && x + fittedWidth + gap > tile.x + 0.1
            && y < tile.y + tile.height + gap - 0.1 && y + tileHeight + gap > tile.y + 0.1);
          if (!overlaps) {
            next = { x, y, width: fittedWidth, height: tileHeight };
            break;
          }
        }
        if (next) break;
      }
      if (next) break;
    }
    // 収まらない投稿は右へ続け、画面端で自然に見切れさせる。
    if (!next) next = { x: Math.max(0, ...tiles.map((tile) => tile.x + tile.width + gap)), y: 0, width, height: (width - 2) / ratio + footer + 2 };
    tiles.push(next);
  }
  return { tiles, width: Math.max(viewportWidth, ...tiles.map((tile) => tile.x + tile.width)), height };
}

// 既存の詰め方を12件ごとに閉じ、過去の全カードを探索しない。
export const mosaicBlockSize = 12;
type Block = { key: string; tiles: MosaicTile[]; width: number };
export function createMosaicPacker() {
  let blocks: Block[] = [];
  let dimensions = "";
  return (items: MosaicItem[], viewportWidth: number, height: number, gap = 12) => {
    const nextDimensions = `${viewportWidth}:${height}:${gap}`;
    if (dimensions !== nextDimensions) { blocks = []; dimensions = nextDimensions; }
    const tiles: MosaicTile[] = [];
    let offset = 0;
    const ranges: { start: number; end: number; left: number; right: number }[] = [];
    for (let start = 0; start < items.length; start += mosaicBlockSize) {
      const slice = items.slice(start, start + mosaicBlockSize);
      const key = slice.map((item) => `${item.ratio}:${item.footer}:${Boolean(item.compact)}`).join(",");
      const index = start / mosaicBlockSize;
      let block = blocks[index];
      if (block?.key !== key) {
        const packed = packBlock(slice, viewportWidth, height, gap);
        block = { key, tiles: packed.tiles, width: Math.max(0, ...packed.tiles.map((tile) => tile.x + tile.width)) };
        blocks[index] = block;
      }
      tiles.push(...block.tiles.map((tile) => ({ ...tile, x: tile.x + offset })));
      ranges.push({ start, end: tiles.length, left: offset, right: offset + block.width });
      offset += block.width + gap;
    }
    blocks.length = Math.ceil(items.length / mosaicBlockSize);
    return { tiles, width: Math.max(viewportWidth, offset - gap), height, ranges };
  };
}
export type MosaicLayout = ReturnType<ReturnType<typeof createMosaicPacker>>;
export function packMosaic(items: MosaicItem[], viewportWidth: number, height: number, gap = 12) {
  return createMosaicPacker()(items, viewportWidth, height, gap);
}

// スクロール時には全投稿を走査せず、座標順のブロックから表示対象を絞る。
export function visibleMosaicIndices(layout: MosaicLayout, left: number, width: number, overscan = width) {
  const min = Math.max(0, left - overscan), max = left + width + overscan;
  let lo = 0, hi = layout.ranges.length;
  while (lo < hi) {
    const mid = (lo + hi) >>> 1;
    if (layout.ranges[mid].right < min) lo = mid + 1;
    else hi = mid;
  }
  const indices: number[] = [];
  for (let b = lo; b < layout.ranges.length && layout.ranges[b].left <= max; b++) {
    const block = layout.ranges[b];
    for (let i = block.start; i < block.end; i++) {
      const tile = layout.tiles[i];
      if (tile.x <= max && tile.x + tile.width >= min) indices.push(i);
    }
  }
  return indices;
}
